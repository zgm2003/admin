package scheduler

import (
	"encoding/json"
	"testing"
)

func TestJobResponseOwnsRetryEligibility(t *testing.T) {
	for _, test := range []struct {
		status JobStatus
		retry  bool
	}{{JobFailed, true}, {JobRunning, false}, {JobCompleted, false}, {JobStatus(99), false}} {
		data, err := json.Marshal(jobDTO(Job{Status: test.status}))
		if err != nil {
			t.Fatal(err)
		}
		var output struct {
			Actions *struct {
				Retry bool `json:"retry"`
			} `json:"actions"`
		}
		if err := json.Unmarshal(data, &output); err != nil {
			t.Fatal(err)
		}
		if output.Actions == nil || output.Actions.Retry != test.retry {
			t.Fatalf("status %d response %s, want retry %v", test.status, data, test.retry)
		}
	}
}

func TestScheduleResponseOwnsDeleteEligibility(t *testing.T) {
	builtin := "builtin"
	for _, test := range []struct {
		key    *string
		remove bool
	}{{nil, true}, {&builtin, false}} {
		data, err := json.Marshal(scheduleDTO(Schedule{BuiltinKey: test.key}))
		if err != nil {
			t.Fatal(err)
		}
		var output struct {
			Actions *struct {
				Delete bool `json:"delete"`
			} `json:"actions"`
		}
		if err := json.Unmarshal(data, &output); err != nil {
			t.Fatal(err)
		}
		if output.Actions == nil || output.Actions.Delete != test.remove {
			t.Fatalf("schedule response %s, want delete %v", data, test.remove)
		}
	}
}
