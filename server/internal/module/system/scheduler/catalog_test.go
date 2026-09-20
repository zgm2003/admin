package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func testDefinition(taskType string) TaskDefinition {
	return TaskDefinition{
		Type:          taskType,
		DisplayName:   taskType,
		Queue:         "default",
		Timeout:       time.Minute,
		MaxAttempts:   3,
		DefaultParams: json.RawMessage(`{"enabled":true}`),
		Execute:       func(context.Context, ExecutionContext) error { return nil },
	}
}

func TestTaskCatalogRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		defs []TaskDefinition
		want error
	}{
		{name: "missing type", defs: []TaskDefinition{{DisplayName: "name", Queue: "default", Timeout: time.Second, MaxAttempts: 1, Execute: func(context.Context, ExecutionContext) error { return nil }}}, want: ErrInvalidTaskDefinition},
		{name: "duplicate type", defs: []TaskDefinition{testDefinition("a"), testDefinition("a")}, want: ErrDuplicateTaskType},
		{name: "duplicate builtin", defs: func() []TaskDefinition {
			a, b := testDefinition("a"), testDefinition("b")
			a.BuiltinKey, b.BuiltinKey = "cleanup", "cleanup"
			return []TaskDefinition{a, b}
		}(), want: ErrDuplicateBuiltinKey},
		{name: "invalid default json", defs: func() []TaskDefinition {
			d := testDefinition("a")
			d.DefaultParams = json.RawMessage(`{`)
			return []TaskDefinition{d}
		}(), want: ErrInvalidTaskDefinition},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewTaskCatalog(test.defs...)
			if !errors.Is(err, test.want) {
				t.Fatalf("NewTaskCatalog() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestTaskCatalogValidatesPayloadAndReturnsSortedOptions(t *testing.T) {
	a := testDefinition("b")
	b := testDefinition("a")
	b.ValidateParams = func(payload json.RawMessage) error {
		var value struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(payload, &value); err != nil {
			return err
		}
		if !value.Enabled {
			return errors.New("enabled must be true")
		}
		return nil
	}
	catalog, err := NewTaskCatalog(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.ValidatePayload("a", json.RawMessage(`{"enabled":false}`)); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("ValidatePayload() error = %v, want invalid payload", err)
	}
	if err := catalog.ValidatePayload("missing", json.RawMessage(`{}`)); !errors.Is(err, ErrUnknownTaskType) {
		t.Fatalf("ValidatePayload() error = %v, want unknown task type", err)
	}
	options := catalog.Options()
	if len(options) != 2 || options[0].Type != "a" || options[1].Type != "b" {
		t.Fatalf("Options() = %#v, want sorted task types", options)
	}
}
