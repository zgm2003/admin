package providerrequest

import (
	"encoding/json"
	"testing"
)

func TestReliableUsageRejectsDuplicatedCounters(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		chat bool
	}{
		{`{"input_tokens":5,"input_tokens":6,"input_tokens_details":{"cached_tokens":0},"output_tokens":1}`, false},
		{`{"input_tokens":5,"input_tokens_details":{"cached_tokens":0,"cached_tokens":1},"output_tokens":1}`, false},
		{`{"prompt_tokens":5,"prompt_tokens":6,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens":1}`, true},
	} {
		if usage := parseTokenUsage(json.RawMessage(tc.raw), tc.chat); usage.Known {
			t.Fatalf("ambiguous usage accepted: %s", tc.raw)
		}
	}
}
