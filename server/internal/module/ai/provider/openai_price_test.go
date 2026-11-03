package provider

import (
	"testing"
	"time"
)

func TestPriceSnapshotCannotMixModelsOrApplyOfficialPriceToCompatibleBaseURL(t *testing.T) {
	p := OpenAIPricingSnapshot{Digest: "fixture", Payload: []byte("fixture official source"), SyncedAt: time.Now(), Rates: []OpenAIRate{{ModelKey: "one", Operation: "text.generate", UsageKey: "input", Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: "1", Selector: []byte(`{}`)}, {ModelKey: "two", Operation: "text.generate", UsageKey: "input", Unit: "token", UnitQuantity: "1000000", SourceUnitPrice: "2", Selector: []byte(`{}`)}}}
	if _, err := p.ForModel("openai", "https://compatible.example.test/v1", "one"); err == nil {
		t.Fatal("compatible upstream received official price")
	}
	snapshot, err := p.ForModel("openai", "https://api.openai.com/v1", "one")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Rates) != 1 || snapshot.Rates[0].SourceUnitPrice != "1" {
		t.Fatal("model rates mixed")
	}
	if _, err := p.ForModel("openai", "https://api.openai.com/v1", "missing"); err == nil {
		t.Fatal("unknown model became free")
	}
}
