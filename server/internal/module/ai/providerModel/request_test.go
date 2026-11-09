package providermodel

import (
	"net/url"
	"testing"

	"admin/server/internal/module/ai/pricing"
	aiprovider "admin/server/internal/module/ai/provider"
	"admin/server/internal/shared/yesno"
)

func TestParseCNYMicros(t *testing.T) {
	cases := map[string]int64{
		"0":        0,
		"0.01":     10000,
		"0.000001": 1,
		"12.5":     12500000,
		"+3.20":    3200000,
	}
	for raw, want := range cases {
		got, err := parseCNYMicros(raw)
		if err != nil {
			t.Fatalf("parseCNYMicros(%q) error = %v", raw, err)
		}
		if got != want {
			t.Fatalf("parseCNYMicros(%q) = %d, want %d", raw, got, want)
		}
	}
	for _, raw := range []string{"", "-1", "1.0000001", "abc", "1.2.3"} {
		if _, err := parseCNYMicros(raw); err == nil {
			t.Fatalf("parseCNYMicros(%q) accepted", raw)
		}
	}
	if formatCNYMicros(12500000) != "12.50" || formatCNYMicros(1) != "0.000001" {
		t.Fatalf("formatCNYMicros values are wrong: %q %q", formatCNYMicros(12500000), formatCNYMicros(1))
	}
}

func TestUpdateRequestInput(t *testing.T) {
	displayName := " gpt-4o "
	kind := pricing.KindChat
	mode := pricing.ModeOfficialToken
	unit := pricing.UnitMillionTokens
	officialID := int64(7)
	status := yesno.Yes
	input, err := (updateRequest{
		DisplayName: &displayName, ModelKind: &kind, PricingMode: &mode,
		PriceUnit: &unit, OfficialModelID: &officialID, Status: &status,
	}).input()
	if err != nil {
		t.Fatal(err)
	}
	if input.DisplayName != "gpt-4o" || input.ModelKind != pricing.KindChat ||
		input.PricingMode != pricing.ModeOfficialToken || input.PriceUnit != pricing.UnitMillionTokens ||
		input.OfficialModelID == nil || *input.OfficialModelID != 7 || input.Status != yesno.Yes {
		t.Fatalf("input = %+v", input)
	}
}

func TestParseListQuery(t *testing.T) {
	query, err := parseListQuery(url.Values{
		"page": {"2"}, "pageSize": {"50"}, "keyword": {" image "}, "modelKind": {"2"}, "status": {"0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if query.Page != 2 || query.PageSize != 50 || query.Keyword != "image" ||
		query.ModelKind != pricing.KindImage || query.Status == nil || *query.Status != yesno.No {
		t.Fatalf("query = %+v", query)
	}
	for _, values := range []url.Values{
		{"page": {"1"}, "modelKind": {"9"}},
		{"page": {"1"}, "status": {"2"}},
		{"page": {"1"}, "unknown": {"x"}},
	} {
		if _, err := parseListQuery(values); err == nil {
			t.Fatalf("query %v accepted", values)
		}
	}
}

func TestParseUpstreamModels(t *testing.T) {
	openAI, err := parseUpstreamModels(aiprovider.ProtocolOpenAI, []byte(`{
		"data": [
			{"id":"gpt-4o"},
			{"id":"gpt-4o"},
			{"id":"gpt-image-1","display_name":"Image"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(openAI) != 2 || openAI[0].ID != "gpt-4o" || openAI[1].DisplayName != "Image" {
		t.Fatalf("OpenAI models = %+v", openAI)
	}

	gemini, err := parseUpstreamModels(aiprovider.ProtocolGemini, []byte(`{
		"models": [{"name":"models/gemini-2.5-pro","displayName":"Gemini 2.5 Pro"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(gemini) != 1 || gemini[0].ID != "gemini-2.5-pro" {
		t.Fatalf("Gemini models = %+v", gemini)
	}
}
