package authstate

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

func (s *Store) ReadUser(ctx context.Context, userID int64) (UserState, bool, error) {
	raw, found, err := s.redis.GetString(ctx, UserStateKey(userID))
	if err != nil || !found {
		return UserState{}, found, err
	}
	var state UserState
	if err := decodeClosedJSON(raw, []string{"schemaVersion", "state", "userId", "generation", "isEnabled", "deleted", "mutationToken"}, &state); err != nil {
		return UserState{}, true, fmt.Errorf("decode authentication user state: %w", err)
	}
	if err := validateUserState(userID, state); err != nil {
		return UserState{}, true, err
	}
	return state, true, nil
}

func (s *Store) ReadSessions(ctx context.Context, platform string, userID int64) (SessionsState, bool, error) {
	raw, found, err := s.redis.GetString(ctx, SessionsStateKey(platform, userID))
	if err != nil || !found {
		return SessionsState{}, found, err
	}
	var state SessionsState
	if err := decodeClosedJSON(raw, []string{"schemaVersion", "state", "platform", "userId", "generation", "mutationToken"}, &state); err != nil {
		return SessionsState{}, true, fmt.Errorf("decode authentication sessions state: %w", err)
	}
	if err := validateSessionsState(platform, userID, state); err != nil {
		return SessionsState{}, true, err
	}
	return state, true, nil
}

func (s *Store) InstallUserReadyIfMissing(ctx context.Context, fact UserFact) (UserState, bool, error) {
	if err := validateUserFact(fact); err != nil {
		return UserState{}, false, err
	}
	state := userStateFromFact(fact, StateReady, nil)
	payload, err := encodeState(state)
	if err != nil {
		return UserState{}, false, err
	}
	installed, err := s.redis.SetStringIfMissing(ctx, UserStateKey(fact.UserID), payload, 0)
	if err != nil {
		return UserState{}, false, err
	}
	if installed {
		return state, true, nil
	}
	return s.readOrRepairUser(ctx, fact.UserID, state, payload)
}

func (s *Store) InstallSessionsReadyIfMissing(ctx context.Context, fact SessionsFact) (SessionsState, bool, error) {
	if err := validateSessionsFact(fact); err != nil {
		return SessionsState{}, false, err
	}
	state := sessionsStateFromFact(fact, StateReady, nil)
	payload, err := encodeState(state)
	if err != nil {
		return SessionsState{}, false, err
	}
	installed, err := s.redis.SetStringIfMissing(ctx, SessionsStateKey(fact.Platform, fact.UserID), payload, 0)
	if err != nil {
		return SessionsState{}, false, err
	}
	if installed {
		return state, true, nil
	}
	return s.readOrRepairSessions(ctx, fact.Platform, fact.UserID, state, payload)
}

func (s *Store) readOrRepairUser(ctx context.Context, userID int64, desired UserState, desiredPayload string) (UserState, bool, error) {
	raw, found, err := s.redis.GetString(ctx, UserStateKey(userID))
	if err != nil {
		return UserState{}, false, err
	}
	if !found {
		installed, setErr := s.redis.SetStringIfMissing(ctx, UserStateKey(userID), desiredPayload, 0)
		if setErr != nil {
			return UserState{}, false, setErr
		}
		if installed {
			return desired, true, nil
		}
		return s.readOrRepairUser(ctx, userID, desired, desiredPayload)
	}
	var current UserState
	decodeErr := decodeClosedJSON(raw, []string{"schemaVersion", "state", "userId", "generation", "isEnabled", "deleted", "mutationToken"}, &current)
	if decodeErr == nil {
		decodeErr = validateUserState(userID, current)
	}
	if decodeErr == nil {
		return current, false, nil
	}
	repaired, repairErr := s.repairExact(ctx, UserStateKey(userID), raw, desiredPayload)
	if repairErr != nil {
		return UserState{}, false, repairErr
	}
	if repaired {
		return desired, true, nil
	}
	current, found, err = s.ReadUser(ctx, userID)
	if err != nil {
		return UserState{}, false, err
	}
	if !found {
		return s.readOrRepairUser(ctx, userID, desired, desiredPayload)
	}
	return current, false, nil
}

func (s *Store) readOrRepairSessions(ctx context.Context, platform string, userID int64, desired SessionsState, desiredPayload string) (SessionsState, bool, error) {
	key := SessionsStateKey(platform, userID)
	raw, found, err := s.redis.GetString(ctx, key)
	if err != nil {
		return SessionsState{}, false, err
	}
	if !found {
		installed, setErr := s.redis.SetStringIfMissing(ctx, key, desiredPayload, 0)
		if setErr != nil {
			return SessionsState{}, false, setErr
		}
		if installed {
			return desired, true, nil
		}
		return s.readOrRepairSessions(ctx, platform, userID, desired, desiredPayload)
	}
	var current SessionsState
	decodeErr := decodeClosedJSON(raw, []string{"schemaVersion", "state", "platform", "userId", "generation", "mutationToken"}, &current)
	if decodeErr == nil {
		decodeErr = validateSessionsState(platform, userID, current)
	}
	if decodeErr == nil {
		return current, false, nil
	}
	repaired, repairErr := s.repairExact(ctx, key, raw, desiredPayload)
	if repairErr != nil {
		return SessionsState{}, false, repairErr
	}
	if repaired {
		return desired, true, nil
	}
	current, found, err = s.ReadSessions(ctx, platform, userID)
	if err != nil {
		return SessionsState{}, false, err
	}
	if !found {
		return s.readOrRepairSessions(ctx, platform, userID, desired, desiredPayload)
	}
	return current, false, nil
}

func (s *Store) repairExact(ctx context.Context, key, corruptPayload, readyPayload string) (bool, error) {
	result, err := s.redis.EvalString(ctx, repairStateScript, []string{key}, corruptPayload, readyPayload)
	if err != nil {
		return false, err
	}
	switch result {
	case "repaired":
		return true, nil
	case "changed", "missing":
		return false, nil
	default:
		return false, fmt.Errorf("repair authentication state returned %q", result)
	}
}

func userStateFromFact(fact UserFact, state string, token *string) UserState {
	return UserState{
		SchemaVersion: SchemaVersion, State: state, UserID: fact.UserID, Generation: fact.Generation,
		IsEnabled: fact.IsEnabled, Deleted: fact.Deleted, MutationToken: token,
	}
}

func sessionsStateFromFact(fact SessionsFact, state string, token *string) SessionsState {
	return SessionsState{
		SchemaVersion: SchemaVersion, State: state, Platform: fact.Platform, UserID: fact.UserID,
		Generation: fact.Generation, MutationToken: token,
	}
}

func encodeState(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode authentication state: %w", err)
	}
	return string(payload), nil
}

func decodeClosedJSON(raw string, expectedKeys []string, target any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return err
	}
	if len(fields) != len(expectedKeys) {
		return fmt.Errorf("JSON field set is invalid")
	}
	for _, key := range expectedKeys {
		if _, ok := fields[key]; !ok {
			return fmt.Errorf("JSON field %q is missing", key)
		}
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("JSON has trailing data")
	}
	return nil
}

const repairStateScript = `
local current = redis.call('GET', KEYS[1])
if not current then return 'missing' end
if current ~= ARGV[1] then return 'changed' end
redis.call('SET', KEYS[1], ARGV[2])
return 'repaired'
`
