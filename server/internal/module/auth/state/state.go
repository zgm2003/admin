package authstate

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"

	"admin/server/internal/module/auth/client"
)

const (
	SchemaVersion     = 1
	StateReady        = "ready"
	StateInvalidating = "invalidating"
)

var (
	ErrUpdating              = errors.New("authentication state is updating")
	ErrGenerationChanged     = errors.New("authentication state generation changed")
	ErrMutationTokenMismatch = errors.New("authentication state mutation token mismatch")
)

type UserState struct {
	SchemaVersion int     `json:"schemaVersion"`
	State         string  `json:"state"`
	UserID        int64   `json:"userId"`
	Generation    string  `json:"generation"`
	IsEnabled     bool    `json:"isEnabled"`
	Deleted       bool    `json:"deleted"`
	MutationToken *string `json:"mutationToken"`
}

type SessionsState struct {
	SchemaVersion int     `json:"schemaVersion"`
	State         string  `json:"state"`
	Platform      string  `json:"platform"`
	UserID        int64   `json:"userId"`
	Generation    string  `json:"generation"`
	MutationToken *string `json:"mutationToken"`
}

type UserFact struct {
	UserID     int64
	Generation string
	IsEnabled  bool
	Deleted    bool
}

type SessionsFact struct {
	Platform   string
	UserID     int64
	Generation string
}

type MutationFacts struct {
	Users    []UserFact
	Sessions []SessionsFact
}

func UserStateKey(userID int64) string {
	return fmt.Sprintf("auth:user-state:%d", userID)
}

func SessionsStateKey(platform string, userID int64) string {
	return fmt.Sprintf("auth:sessions-state:%s:%d", platform, userID)
}

func NewGeneration() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate authentication state generation: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func (s UserState) Fact() UserFact {
	return UserFact{UserID: s.UserID, Generation: s.Generation, IsEnabled: s.IsEnabled, Deleted: s.Deleted}
}

func (s SessionsState) Fact() SessionsFact {
	return SessionsFact{Platform: s.Platform, UserID: s.UserID, Generation: s.Generation}
}

func validateUserFact(fact UserFact) error {
	if fact.UserID < 1 || fact.Generation == "" || (fact.Deleted && fact.IsEnabled) {
		return fmt.Errorf("authentication user fact is invalid")
	}
	return nil
}

func validateSessionsFact(fact SessionsFact) error {
	if fact.UserID < 1 || fact.Generation == "" {
		return fmt.Errorf("authentication sessions fact is invalid")
	}
	if err := authclient.ValidatePlatform(fact.Platform); err != nil {
		return fmt.Errorf("authentication sessions fact: %w", err)
	}
	return nil
}

func validateUserState(expectedUserID int64, state UserState) error {
	if state.SchemaVersion != SchemaVersion || state.UserID != expectedUserID {
		return fmt.Errorf("authentication user state identity is invalid")
	}
	if err := validateUserFact(state.Fact()); err != nil {
		return err
	}
	return validateStateToken(state.State, state.MutationToken)
}

func validateSessionsState(expectedPlatform string, expectedUserID int64, state SessionsState) error {
	if state.SchemaVersion != SchemaVersion || state.Platform != expectedPlatform || state.UserID != expectedUserID {
		return fmt.Errorf("authentication sessions state identity is invalid")
	}
	if err := validateSessionsFact(state.Fact()); err != nil {
		return err
	}
	return validateStateToken(state.State, state.MutationToken)
}

func validateStateToken(state string, token *string) error {
	switch state {
	case StateReady:
		if token != nil {
			return fmt.Errorf("ready authentication state has a mutation token")
		}
	case StateInvalidating:
		if token == nil || *token == "" {
			return fmt.Errorf("invalidating authentication state has no mutation token")
		}
	default:
		return fmt.Errorf("authentication state value is invalid")
	}
	return nil
}

func normalizeMutationFacts(facts MutationFacts) (MutationFacts, error) {
	normalized := MutationFacts{
		Users:    append([]UserFact(nil), facts.Users...),
		Sessions: append([]SessionsFact(nil), facts.Sessions...),
	}
	for _, fact := range normalized.Users {
		if err := validateUserFact(fact); err != nil {
			return MutationFacts{}, err
		}
	}
	for _, fact := range normalized.Sessions {
		if err := validateSessionsFact(fact); err != nil {
			return MutationFacts{}, err
		}
	}
	sort.Slice(normalized.Users, func(left, right int) bool {
		return normalized.Users[left].UserID < normalized.Users[right].UserID
	})
	sort.Slice(normalized.Sessions, func(left, right int) bool {
		if normalized.Sessions[left].UserID != normalized.Sessions[right].UserID {
			return normalized.Sessions[left].UserID < normalized.Sessions[right].UserID
		}
		return normalized.Sessions[left].Platform < normalized.Sessions[right].Platform
	})

	users := normalized.Users[:0]
	for _, fact := range normalized.Users {
		if len(users) > 0 && users[len(users)-1].UserID == fact.UserID {
			if users[len(users)-1] != fact {
				return MutationFacts{}, fmt.Errorf("duplicate authentication user fact conflicts")
			}
			continue
		}
		users = append(users, fact)
	}
	sessions := normalized.Sessions[:0]
	for _, fact := range normalized.Sessions {
		if len(sessions) > 0 && sessions[len(sessions)-1].UserID == fact.UserID && sessions[len(sessions)-1].Platform == fact.Platform {
			if sessions[len(sessions)-1] != fact {
				return MutationFacts{}, fmt.Errorf("duplicate authentication sessions fact conflicts")
			}
			continue
		}
		sessions = append(sessions, fact)
	}
	normalized.Users = users
	normalized.Sessions = sessions
	return normalized, nil
}
