package usage

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"encoding/json"
	"testing"
)

func TestRequestUsageDigestSurvivesJSONBOrdering(t *testing.T) {
	makeFacts := func(raw string) SettlementFacts {
		t.Helper()
		facts := SettlementFacts{RunID: 9, Rates: []FrozenRate{{ID: 1, Operation: "text.generate", UsageKey: "input"}, {ID: 2, Operation: "text.generate", UsageKey: "output"}}}
		usage, _ := json.Marshal(pr.TokenUsage{Known: true, InputTokens: 2, OutputTokens: 3, Raw: json.RawMessage(raw)})
		if err := attachRequestFacts(&facts, "text.generate", json.RawMessage(`{}`), []pr.Fact{{ID: 11, RunID: 9, Status: 2, UsageStatus: 1, Usage: usage}}); err != nil {
			t.Fatal(err)
		}
		return facts
	}
	a := makeFacts(`{"input_tokens":2,"output_tokens":3}`)
	b := makeFacts(`{"output_tokens":3,"input_tokens":2}`)
	if a.UsageDigests[11] != b.UsageDigests[11] {
		t.Fatal("JSONB ordering changed durable usage digest")
	}
	if len(a.RequiredItemKeys) != 2 || a.Quantities["11:input"] != "2" || a.Quantities["11:output"] != "3" {
		t.Fatal("durable quantities incomplete")
	}
}
