package accessstate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	projectredis "admin/server/internal/redis"
)

type Store struct {
	redis *projectredis.Client
}

func NewStore(redis *projectredis.Client) *Store {
	return &Store{redis: redis}
}

func (s *Store) Read(ctx context.Context, userID int64) (State, bool, error) {
	raw, found, err := s.redis.GetString(ctx, StateKey(userID))
	if err != nil || !found {
		return State{}, found, err
	}
	state, err := decodeState(raw)
	if err != nil {
		return State{}, true, err
	}
	return state, true, nil
}

func (s *Store) InstallReadyIfMissing(ctx context.Context, version Version) (State, bool, error) {
	if version.UserID < 1 || version.Version < 1 {
		return State{}, false, fmt.Errorf("access version is invalid")
	}
	desired := State{SchemaVersion: SchemaVersion, State: StateReady, Version: version.Version}
	payload, err := encodeState(desired)
	if err != nil {
		return State{}, false, err
	}
	installed, err := s.redis.SetStringIfMissing(ctx, StateKey(version.UserID), payload, 0)
	if err != nil {
		return State{}, false, err
	}
	if installed {
		return desired, true, nil
	}
	raw, found, err := s.redis.GetString(ctx, StateKey(version.UserID))
	if err != nil {
		return State{}, false, err
	}
	if !found {
		return s.InstallReadyIfMissing(ctx, version)
	}
	current, decodeErr := decodeState(raw)
	if decodeErr == nil {
		return current, false, nil
	}
	result, err := s.redis.EvalString(ctx, repairStateScript, []string{StateKey(version.UserID)}, raw, payload)
	if err != nil {
		return State{}, false, err
	}
	if result == "repaired" {
		return desired, true, nil
	}
	if result == "changed" || result == "missing" {
		return s.InstallReadyIfMissing(ctx, version)
	}
	return State{}, false, fmt.Errorf("repair access state returned %q", result)
}

// RebuildReadyState synchronizes ready access states with database versions.
// A state currently held by a mutation lease is left untouched and reported as updating.
func (s *Store) RebuildReadyState(ctx context.Context, versions []Version) error {
	normalized, err := normalizeVersions(versions)
	if err != nil {
		return err
	}
	keys := make([]string, len(normalized))
	args := make([]any, 0, len(normalized))
	for index, version := range normalized {
		payload, err := encodeState(State{SchemaVersion: SchemaVersion, State: StateReady, Version: version.Version})
		if err != nil {
			return err
		}
		keys[index] = StateKey(version.UserID)
		args = append(args, payload)
	}
	if len(keys) == 0 {
		return nil
	}
	result, err := s.redis.EvalString(ctx, rebuildReadyStateScript, keys, args...)
	if err != nil {
		return err
	}
	if result == "updating" {
		return ErrUpdating
	}
	if result != "rebuilt" {
		return fmt.Errorf("rebuild access state returned %q", result)
	}
	return nil
}

func encodeState(state State) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode access state: %w", err)
	}
	return string(payload), nil
}

func decodeState(raw string) (State, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return State{}, fmt.Errorf("decode access state fields: %w", err)
	}
	stateField, ok := fields["state"]
	if !ok {
		return State{}, fmt.Errorf("access state field is missing")
	}
	var stateName string
	if err := json.Unmarshal(stateField, &stateName); err != nil {
		return State{}, fmt.Errorf("decode access state name: %w", err)
	}
	expected := 3
	if stateName == StateInvalidating {
		expected = 5
	}
	if len(fields) != expected {
		return State{}, fmt.Errorf("access state JSON field set is invalid")
	}
	for _, key := range []string{"schemaVersion", "state", "version"} {
		if _, ok := fields[key]; !ok {
			return State{}, fmt.Errorf("access state field %q is missing", key)
		}
	}
	if stateName == StateInvalidating {
		for _, key := range []string{"mutationToken", "baseVersion"} {
			if _, ok := fields[key]; !ok {
				return State{}, fmt.Errorf("access state field %q is missing", key)
			}
		}
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	var state State
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode access state: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return State{}, fmt.Errorf("access state has trailing data")
	}
	if err := validateState(state); err != nil {
		return State{}, err
	}
	return state, nil
}

const repairStateScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
if current ~= ARGV[1] then return 'changed' end
redis.call('SET', KEYS[1], ARGV[2])
return 'repaired'
`

const rebuildReadyStateScript = `
for _, key in ipairs(KEYS) do
  local current = redis.call('GET', key)
  if current then
    local ok, decoded = pcall(cjson.decode, current)
    if ok and decoded.state == 'invalidating' then return 'updating' end
  end
end
for index, key in ipairs(KEYS) do redis.call('SET', key, ARGV[index]) end
return 'rebuilt'
`
