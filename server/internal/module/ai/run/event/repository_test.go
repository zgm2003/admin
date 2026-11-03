package event

import (
	"admin/server/internal/module/realtime"
	"encoding/json"
	"github.com/google/uuid"
	"testing"
	"time"
)

func TestRunEventUsesExistingDurableRealtimeProtocol(t *testing.T) {
	user := int64(1)
	if err := realtime.ValidateEventInput(realtime.EventInput{EventID: uuid.NewString(), DedupKey: "ai.run:1:1", PlatformID: 1, TargetType: realtime.TargetUser, TargetUserID: &user, EventType: realtime.EventAIRunChanged, Payload: json.RawMessage(`{"runId":"1","sequence":1,"eventType":"queued"}`), OccurredAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
}
