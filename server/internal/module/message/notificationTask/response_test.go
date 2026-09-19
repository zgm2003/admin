package notificationtask

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTaskListDTOExcludesDetailFieldsAndIncludesCompletion(t *testing.T) {
	completedAt := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(taskListDTO(Task{
		ID:          1,
		Title:       "notice",
		ContentHTML: "<p>secret detail</p>",
		TargetIDs:   []int64{99},
		CompletedAt: &completedAt,
	}))
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	for _, forbidden := range []string{"contentHtml", "targetIds", "secret detail"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("list response leaked %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(encoded, `"completedAt":"2026-09-18T12:00:00Z"`) {
		t.Fatalf("list response missing completedAt: %s", encoded)
	}
}
