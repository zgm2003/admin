package providerrequest

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProtocolValuesUseLowerCamelAndExcludeFrozenCredentials(t *testing.T) {
	raw, err := json.Marshal(struct {
		Config FrozenConfig `json:"config"`
		Call   FunctionCall `json:"call"`
		Usage  TokenUsage   `json:"usage"`
	}{
		Config: FrozenConfig{APIKey: "TOKEN", Headers: map[string]string{"x-provider-secret": "HEADER_SECRET"}},
		Call:   FunctionCall{CallID: "call_A", Name: "current_time", Arguments: "{}"}, Usage: TokenUsage{Known: true, InputTokens: 4, CachedInputTokens: 1, OutputTokens: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "TOKEN") || strings.Contains(string(raw), "HEADER_SECRET") || !strings.Contains(string(raw), `"callId":"call_A"`) || !strings.Contains(string(raw), `"cachedInputTokens":1`) {
		t.Fatalf("wire=%s", raw)
	}
}
