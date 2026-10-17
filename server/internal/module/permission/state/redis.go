package permissionstate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	projectredis "admin/server/internal/redis"
)

type Store struct {
	redis     *projectredis.Client
	keyPrefix string
}

func (s *Store) key(id int64) string {
	if s.keyPrefix != "" {
		return fmt.Sprintf("%s%d", s.keyPrefix, id)
	}
	return StateKey(id)
}

func NewStore(redis *projectredis.Client) *Store {
	return &Store{redis: redis}
}

func (s *Store) Read(ctx context.Context, userID int64) (State, bool, error) {
	raw, found, err := s.redis.GetString(ctx, s.key(userID))
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
	installed, err := s.redis.SetStringIfMissing(ctx, s.key(version.UserID), payload, 0)
	if err != nil {
		return State{}, false, err
	}
	if installed {
		return desired, true, nil
	}
	raw, found, err := s.redis.GetString(ctx, s.key(version.UserID))
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
	result, err := s.redis.EvalString(ctx, repairStateScript, []string{s.key(version.UserID)}, raw, payload)
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
// A mutation lease, corrupt state or higher version aborts the whole batch.
// Go validates int64 versions exactly; Lua only compares the observed raw values
// so cjson's double precision cannot turn adjacent large versions into equals.
func (s *Store) RebuildReadyState(ctx context.Context, versions []Version) error {
	normalized, err := normalizeVersions(versions)
	if err != nil {
		return err
	}
	keys := make([]string, len(normalized))
	for index, version := range normalized {
		keys[index] = s.key(version.UserID)
	}
	if len(keys) == 0 {
		return nil
	}
	observed, err := s.redis.UniversalClient().MGet(ctx, keys...).Result()
	if err != nil {
		return fmt.Errorf("read access states for rebuild: %w", err)
	}
	args := make([]any, 0, 3*len(normalized))
	for index, version := range normalized {
		found, raw := "0", ""
		if observed[index] != nil {
			var ok bool
			raw, ok = observed[index].(string)
			if !ok {
				return fmt.Errorf("access state payload type is invalid")
			}
			current, err := decodeState(raw)
			if err != nil {
				return fmt.Errorf("access state is corrupt: %w", err)
			}
			if current.State != StateReady {
				return ErrUpdating
			}
			if current.Version > version.Version {
				return ErrVersionChanged
			}
			found = "1"
		}
		payload, err := encodeState(State{SchemaVersion: SchemaVersion, State: StateReady, Version: version.Version})
		if err != nil {
			return err
		}
		args = append(args, found, raw, payload)
	}
	result, err := s.redis.EvalString(ctx, rebuildReadyStateScript, keys, args...)
	if err != nil {
		return err
	}
	if result == "changed" {
		return ErrVersionChanged
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
for index, key in ipairs(KEYS) do
  local current = redis.call('GET', key)
  local offset = (index - 1) * 3
  if ARGV[offset + 1] == '0' then
    if current then return 'changed' end
  elseif current ~= ARGV[offset + 2] then
    return 'changed'
  end
end
for index, key in ipairs(KEYS) do redis.call('SET', key, ARGV[index * 3]) end
return 'rebuilt'
`
