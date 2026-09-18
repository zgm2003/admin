// Package cachegeneration implements the stable, mechanical cache-generation
// protocol shared by configuration caches: scope coordinates, deterministic
// Redis keys and a strict state codec. It owns no business snapshot; callers
// keep ownership of their payloads.
package cachegeneration

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	SchemaVersion = 1

	StateReady        = "ready"
	StateInvalidating = "invalidating"
)

const (
	stateKeyPrefix    = "config-cache:state:v1:"
	snapshotKeyPrefix = "config-cache:snapshot:v1:"
	fillKeyPrefix     = "config-cache:fill:v1:"
)

const (
	namespaceMaxLength = 128
	variantMaxLength   = 192
)

var (
	namespacePattern  = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)
	coordinatePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$`)
	variantPattern    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,191}$`)
)

// ErrMutationRolledBack marks a business transaction that is known to have
// rolled back. Coordinators use it to distinguish a generation race from an
// uncertain post-commit transport error.
var ErrMutationRolledBack = errors.New("cache generation business mutation rolled back")

// ErrStateCorrupt 表示 Redis 中的 state payload 无法按协议解析。
var ErrStateCorrupt = errors.New("config cache state is corrupt")

// MutationResult 只表达一次业务 mutation 的机械提交结果：是否真实发生变化、提交后的 generation 与 outbox 事件 ID。
// 它不执行回调、不持有业务 Repository，也不是 Manager；各模块在同一个 PostgreSQL 事务里填充它。
type MutationResult struct {
	Changed    bool
	Generation int64
	OutboxID   int64
}

type Scope struct {
	Namespace string
	ScopeKey  string
}

func NewScope(namespace, scopeKey string) (Scope, error) {
	scope := Scope{Namespace: namespace, ScopeKey: scopeKey}
	if err := scope.Validate(); err != nil {
		return Scope{}, err
	}
	return scope, nil
}

func (s Scope) Validate() error {
	if len(s.Namespace) == 0 || len(s.Namespace) > namespaceMaxLength || !namespacePattern.MatchString(s.Namespace) {
		return fmt.Errorf("cache generation namespace is invalid")
	}
	if !coordinatePattern.MatchString(s.ScopeKey) {
		return fmt.Errorf("cache generation scope key is invalid")
	}
	return nil
}

func StateKey(scope Scope) string {
	return stateKeyPrefix + scope.Namespace + ":" + scope.ScopeKey
}

func SnapshotKey(scope Scope, generation int64, variant string) (string, error) {
	return payloadKey(snapshotKeyPrefix, scope, generation, variant)
}

func FillKey(scope Scope, generation int64, variant string) (string, error) {
	return payloadKey(fillKeyPrefix, scope, generation, variant)
}

func payloadKey(prefix string, scope Scope, generation int64, variant string) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", err
	}
	if generation < 1 {
		return "", fmt.Errorf("cache generation payload generation is invalid")
	}
	if !variantPattern.MatchString(variant) || len(variant) > variantMaxLength {
		return "", fmt.Errorf("cache generation payload variant is invalid")
	}
	return fmt.Sprintf("%s%s:%s:%d:%s", prefix, scope.Namespace, scope.ScopeKey, generation, variant), nil
}

// State 只表达代际状态本身，不包含任何业务快照内容。
type State struct {
	SchemaVersion  int    `json:"schemaVersion"`
	State          string `json:"state"`
	Generation     int64  `json:"generation,omitempty"`
	BaseGeneration int64  `json:"baseGeneration,omitempty"`
	MutationToken  string `json:"mutationToken,omitempty"`
}

func encodeState(state State) (string, error) {
	if err := validateState(state); err != nil {
		return "", err
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode config cache state: %w", err)
	}
	return string(payload), nil
}

func decodeState(raw string) (State, error) {
	fields, err := decodeStrictFields(raw)
	if err != nil {
		return State{}, err
	}
	stateName, err := decodeStateName(fields)
	if err != nil {
		return State{}, err
	}
	required := []string{"schemaVersion", "state", "generation"}
	if stateName == StateInvalidating {
		required = []string{"schemaVersion", "state", "baseGeneration", "mutationToken"}
	}
	if len(fields) != len(required) {
		return State{}, corruptError("config cache state field set is invalid")
	}
	for _, key := range required {
		if _, ok := fields[key]; !ok {
			return State{}, corruptError("config cache state field %q is missing", key)
		}
	}

	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var state State
	if err := decoder.Decode(&state); err != nil {
		return State{}, corruptError("decode config cache state: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return State{}, corruptError("config cache state has trailing data")
	}
	if err := validateState(state); err != nil {
		return State{}, corruptError("validate config cache state: %v", err)
	}
	return state, nil
}

func decodeStateName(fields map[string]json.RawMessage) (string, error) {
	raw, ok := fields["state"]
	if !ok {
		return "", corruptError("config cache state field \"state\" is missing")
	}
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return "", corruptError("decode config cache state name: %v", err)
	}
	return name, nil
}

// decodeStrictFields 逐 token 解析对象字段，拒绝重复字段与尾随内容。
func decodeStrictFields(raw string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, corruptError("decode config cache state: %v", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, corruptError("config cache state must be a JSON object")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, corruptError("decode config cache state field: %v", err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, corruptError("config cache state field name is invalid")
		}
		if _, exists := fields[key]; exists {
			return nil, corruptError("config cache state field %q is duplicated", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, corruptError("decode config cache state field %q: %v", key, err)
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, corruptError("decode config cache state: %v", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, corruptError("config cache state has trailing data")
	}
	return fields, nil
}

func validateState(state State) error {
	if state.SchemaVersion != SchemaVersion {
		return fmt.Errorf("cache generation state schema version is invalid")
	}
	switch state.State {
	case StateReady:
		if state.Generation < 1 || state.BaseGeneration != 0 || state.MutationToken != "" {
			return fmt.Errorf("ready cache generation state is invalid")
		}
	case StateInvalidating:
		if state.Generation != 0 || state.BaseGeneration < 1 || state.MutationToken == "" {
			return fmt.Errorf("invalidating cache generation state is invalid")
		}
	default:
		return fmt.Errorf("cache generation state value is invalid")
	}
	return nil
}

func corruptError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrStateCorrupt, fmt.Sprintf(format, args...))
}
