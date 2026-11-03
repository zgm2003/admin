package run

import (
	"testing"
	"time"
)

func TestPublicResponseHidesRunSnapshotsAndStringifiesIDs(t *testing.T) {
	finished := time.Now().UTC()
	code := "FAILED"
	out := PublicResponse(Model{ID: 7, AgentID: 9, Operation: "text.generate", ExecutionStatus: 4, FinishedAt: &finished, ErrorCode: &code})
	if out.ID != "7" || out.AgentID != "9" || out.Operation != "text.generate" || out.FinishedAt == nil || out.ErrorCode == nil {
		t.Fatalf("response=%+v", out)
	}
}
