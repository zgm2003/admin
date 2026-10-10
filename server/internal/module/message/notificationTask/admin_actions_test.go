package notificationtask

import (
	"encoding/json"
	"testing"
)

func TestAdminTaskActionsAreReturnedByBackend(t *testing.T) {
	for _, status := range []Status{StatusDraft, StatusScheduled, StatusQueued, StatusProcessing, StatusCompleted, StatusFailed, StatusCanceled} {
		data, err := json.Marshal(taskListDTO(Task{Status: status}))
		if err != nil {
			t.Fatal(err)
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		var actions map[string]bool
		if err := json.Unmarshal(row["actions"], &actions); err != nil {
			t.Fatalf("status %d must return actions: %v", status, err)
		}
		draft := status == StatusDraft
		cancelable := status == StatusScheduled || status == StatusQueued || status == StatusProcessing
		if actions["edit"] != draft || actions["delete"] != draft || actions["submit"] != draft || actions["cancel"] != cancelable || actions["copy"] == draft {
			t.Fatalf("status %d actions = %v", status, actions)
		}
	}
}
