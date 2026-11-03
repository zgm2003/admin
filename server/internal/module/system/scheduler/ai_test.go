package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestAITaskDefinitionsRequireEveryNamedExecutorAndBudgets(t *testing.T) {
	execute := func(context.Context, ExecutionContext) error { return nil }
	valid := AITaskExecutors{RunExecute: execute, MediaPoll: execute, MediaImport: execute, UsageSettle: execute, RunRecover: execute, RunEventCleanup: execute}
	for _, unset := range []func(*AITaskExecutors){
		func(v *AITaskExecutors) { v.RunExecute = nil }, func(v *AITaskExecutors) { v.MediaPoll = nil }, func(v *AITaskExecutors) { v.MediaImport = nil }, func(v *AITaskExecutors) { v.UsageSettle = nil }, func(v *AITaskExecutors) { v.RunRecover = nil }, func(v *AITaskExecutors) { v.RunEventCleanup = nil },
	} {
		value := valid
		unset(&value)
		if _, err := AITaskDefinitions(value, time.Minute, time.Minute); !errors.Is(err, ErrInvalidTaskDefinition) {
			t.Fatalf("missing executor err=%v", err)
		}
	}
	for _, budgets := range [][2]time.Duration{{0, time.Minute}, {time.Minute, 0}, {-time.Second, time.Minute}, {time.Duration(1<<63 - 1), time.Minute}} {
		if _, err := AITaskDefinitions(valid, budgets[0], budgets[1]); !errors.Is(err, ErrInvalidTaskDefinition) {
			t.Fatalf("invalid budget err=%v", err)
		}
	}
}

func TestAITaskDefinitionsKeepIndependentRetryBoundariesAndNamedExecutors(t *testing.T) {
	sentinel := errors.New("named executor failure")
	execute := func(ctx context.Context, input ExecutionContext) error {
		if input.JobID != 9 || ctx.Value("test") != "context" {
			t.Fatal("execution context lost")
		}
		return sentinel
	}
	definitions, err := AITaskDefinitions(AITaskExecutors{execute, execute, execute, execute, execute, execute}, 2*time.Minute, 3*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := NewTaskCatalog(definitions...)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		task     string
		attempts int
		timeout  time.Duration
		builtin  string
	}{
		{"ai.run.execute", 1, 330 * time.Second, ""}, {"ai.media.poll", 3, 30 * time.Second, ""}, {"ai.media.import", 3, 210 * time.Second, ""}, {"ai.usage.settle", 10, 30 * time.Second, ""}, {"ai.run.recover", 3, 20 * time.Second, "ai.run.recover"}, {"ai.run.event.cleanup", 3, 30 * time.Second, "ai.run.event.cleanup"},
	} {
		definition, ok := catalog.Lookup(tc.task)
		if !ok || definition.MaxAttempts != tc.attempts || definition.Timeout != tc.timeout || definition.AdminCreatable || definition.BuiltinKey != tc.builtin {
			t.Fatalf("definition=%+v", definition)
		}
		for _, raw := range []string{"null", "[]", "true", `"text"`} {
			if err := catalog.ValidatePayload(tc.task, json.RawMessage(raw)); !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("accepted %s", raw)
			}
		}
		if err := catalog.ValidatePayload(tc.task, json.RawMessage(`{"schemaVersion":1,"runId":9}`)); err != nil {
			t.Fatal(err)
		}
		ctx := context.WithValue(context.Background(), "test", "context")
		if err := definition.Execute(ctx, ExecutionContext{JobID: 9}); !errors.Is(err, sentinel) {
			t.Fatalf("executor err=%v", err)
		}
	}
	settle, _ := catalog.Lookup("ai.usage.settle")
	for attempt, want := range []time.Duration{5, 10, 20, 40, 80, 160, 300, 300, 300, 300} {
		if got := settle.Backoff(attempt + 1); got != want*time.Second {
			t.Fatalf("attempt %d got %s", attempt+1, got)
		}
	}
}
