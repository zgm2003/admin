package main

import (
	"context"
	"encoding/json"
	"testing"

	"admin/server/internal/module/system/scheduler"
)

func TestAPICatalogExposesAIRecoveryWithoutExecutingInAPI(t *testing.T) {
	catalog, err := buildAPISchedulerCatalog(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []string{"ai.run.execute", "ai.media.poll", "ai.media.import", "ai.usage.settle", "ai.run.recover", "ai.run.event.cleanup"} {
		definition, ok := catalog.Lookup(task)
		if !ok {
			t.Errorf("AI task %s unavailable to scheduler management", task)
			continue
		}
		if definition.AdminCreatable {
			t.Errorf("AI task %s must not allow arbitrary schedules", task)
		}
		if err := definition.Execute(context.Background(), scheduler.ExecutionContext{}); err == nil {
			t.Errorf("API must not execute %s", task)
		}
	}
	for _, task := range []string{"ai.run.recover", "ai.run.event.cleanup"} {
		if err := catalog.ValidatePayload(task, json.RawMessage(`{}`)); err != nil {
			t.Errorf("cannot edit seeded %s: %v", task, err)
		}
	}
}
