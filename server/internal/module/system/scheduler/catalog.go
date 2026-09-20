package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

func NotificationBatchDefinition(execute TaskExecutor) TaskDefinition {
	if execute == nil {
		execute = func(context.Context, ExecutionContext) error {
			return errors.New("notification batch executor is unavailable")
		}
	}
	return TaskDefinition{Type: "message.notificationtask.batch", DisplayName: "站内通知批次", AdminCreatable: false, DefaultParams: json.RawMessage(`{}`), Queue: "default", Timeout: 60 * time.Second, MaxAttempts: 10, Backoff: func(attempt int) time.Duration { return publishBackoff(attempt) }, ValidateParams: func(raw json.RawMessage) error {
		var payload struct {
			SchemaVersion int   `json:"schemaVersion"`
			TaskID        int64 `json:"taskId"`
			BatchNo       int   `json:"batchNo"`
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			return err
		}
		if payload.SchemaVersion != 1 || payload.TaskID <= 0 || payload.BatchNo < 0 {
			return ErrInvalidPayload
		}
		return nil
	}, Execute: execute}
}

type TaskCatalog struct {
	definitions map[string]TaskDefinition
}

func NewTaskCatalog(definitions ...TaskDefinition) (TaskCatalog, error) {
	catalog := TaskCatalog{definitions: make(map[string]TaskDefinition, len(definitions))}
	builtinKeys := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		definition.Type = strings.TrimSpace(definition.Type)
		definition.DisplayName = strings.TrimSpace(definition.DisplayName)
		definition.BuiltinKey = strings.TrimSpace(definition.BuiltinKey)
		definition.Queue = strings.TrimSpace(definition.Queue)
		if definition.Type == "" || definition.DisplayName == "" || definition.Queue == "" || definition.Timeout <= 0 || definition.MaxAttempts <= 0 || definition.Execute == nil {
			return TaskCatalog{}, ErrInvalidTaskDefinition
		}
		if _, exists := catalog.definitions[definition.Type]; exists {
			return TaskCatalog{}, ErrDuplicateTaskType
		}
		if definition.BuiltinKey != "" {
			if _, exists := builtinKeys[definition.BuiltinKey]; exists {
				return TaskCatalog{}, ErrDuplicateBuiltinKey
			}
			builtinKeys[definition.BuiltinKey] = struct{}{}
		}
		if definition.DefaultParams == nil {
			definition.DefaultParams = json.RawMessage(`{}`)
		}
		if !json.Valid(definition.DefaultParams) {
			return TaskCatalog{}, ErrInvalidTaskDefinition
		}
		definition.DefaultParams = append(json.RawMessage(nil), definition.DefaultParams...)
		catalog.definitions[definition.Type] = definition
	}
	return catalog, nil
}

func (c TaskCatalog) Lookup(taskType string) (TaskDefinition, bool) {
	definition, ok := c.definitions[strings.TrimSpace(taskType)]
	return definition, ok
}

func (c TaskCatalog) ValidatePayload(taskType string, payload json.RawMessage) error {
	definition, ok := c.Lookup(taskType)
	if !ok {
		return ErrUnknownTaskType
	}
	if len(bytes.TrimSpace(payload)) == 0 || !json.Valid(payload) {
		return ErrInvalidPayload
	}
	if definition.ValidateParams != nil {
		if err := definition.ValidateParams(payload); err != nil {
			return errorsJoin(ErrInvalidPayload, err)
		}
	}
	return nil
}

func (c TaskCatalog) Options() []TaskOption {
	options := make([]TaskOption, 0, len(c.definitions))
	for _, definition := range c.definitions {
		options = append(options, TaskOption{Type: definition.Type, DisplayName: definition.DisplayName, AdminCreatable: definition.AdminCreatable, BuiltinKey: definition.BuiltinKey, DefaultParams: append(json.RawMessage(nil), definition.DefaultParams...)})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Type < options[j].Type })
	return options
}

func errorsJoin(first, second error) error {
	if second == nil {
		return first
	}
	return &validationError{first: first, second: second}
}

type validationError struct {
	first  error
	second error
}

func (e *validationError) Error() string { return e.first.Error() + ": " + e.second.Error() }
func (e *validationError) Unwrap() error { return e.first }
