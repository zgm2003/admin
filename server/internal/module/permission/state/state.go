package permissionstate

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
)

const (
	SchemaVersion     = 2
	StateReady        = "ready"
	StateInvalidating = "invalidating"
)

var (
	ErrUpdating              = errors.New("access state is updating")
	ErrVersionChanged        = errors.New("access state version changed")
	ErrMutationTokenMismatch = errors.New("access state mutation token mismatch")
)

type Version struct {
	UserID  int64
	Version int64
}

type State struct {
	SchemaVersion int    `json:"schemaVersion"`
	State         string `json:"state"`
	Version       int64  `json:"version"`
	MutationToken string `json:"mutationToken,omitempty"`
	BaseVersion   int64  `json:"baseVersion,omitempty"`
}

func StateKey(userID int64) string {
	return "authz:permission-state:" + strconv.FormatInt(userID, 10)
}

func normalizeVersions(values []Version) ([]Version, error) {
	normalized := append([]Version(nil), values...)
	for _, value := range normalized {
		if value.UserID < 1 || value.Version < 1 {
			return nil, fmt.Errorf("access version is invalid")
		}
	}
	sort.Slice(normalized, func(left, right int) bool {
		return normalized[left].UserID < normalized[right].UserID
	})
	result := normalized[:0]
	for _, value := range normalized {
		if len(result) > 0 && result[len(result)-1].UserID == value.UserID {
			if result[len(result)-1].Version != value.Version {
				return nil, fmt.Errorf("duplicate access versions conflict")
			}
			continue
		}
		result = append(result, value)
	}
	return result, nil
}

func validateState(state State) error {
	if state.SchemaVersion != SchemaVersion {
		return fmt.Errorf("access state schema version is invalid")
	}
	switch state.State {
	case StateReady:
		if state.Version < 1 || state.MutationToken != "" || state.BaseVersion != 0 {
			return fmt.Errorf("ready access state is invalid")
		}
	case StateInvalidating:
		if state.Version != 0 || state.MutationToken == "" || state.BaseVersion < 1 {
			return fmt.Errorf("invalidating access state is invalid")
		}
	default:
		return fmt.Errorf("access state value is invalid")
	}
	return nil
}
