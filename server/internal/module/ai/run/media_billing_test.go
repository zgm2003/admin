package run

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"encoding/json"
	"testing"
)

func TestMediaBillingSelectsFrozenImageTier(t *testing.T) {
	s := fixtureSnapshot()
	s.Config = json.RawMessage(`{"rates":[{"id":1,"operation":"image.generate","usageKey":"output","selector":{"quality":"low","size":"1024x1024"},"unit":"image","unitQuantity":"1","billingUnitPriceUnits":10},{"id":2,"operation":"image.generate","usageKey":"output","selector":{"quality":"high","size":"1024x1024"},"unit":"image","unitQuantity":"1","billingUnitPriceUnits":90}]}`)
	lines, err := mediaUsageLines(s, 9, pr.MediaUsage{Known: true, ImageCount: 2}, "image.generate", RunInput{Quality: "high", Size: "1024x1024"})
	if err != nil || len(lines) != 1 || lines[0].Rate.ID != 2 || lines[0].Calculation.Quantity != "2" {
		t.Fatalf("tier lines=%+v err=%v", lines, err)
	}
	if _, err = mediaUsageLines(s, 9, pr.MediaUsage{Known: true, ImageCount: 2}, "image.generate", RunInput{Quality: "medium", Size: "1024x1024"}); err == nil {
		t.Fatal("missing tier accepted")
	}
}

func TestMediaBillingRejectsIncompleteUsage(t *testing.T) {
	s := fixtureSnapshot()
	s.Config = json.RawMessage(`{"rates":[{"id":1,"operation":"audio.speech","usageKey":"input","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":10},{"id":2,"operation":"audio.speech","usageKey":"output","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":90}]}`)
	n := int64(2)
	if _, err := mediaUsageLines(s, 9, pr.MediaUsage{Known: true, InputTokens: &n}, "audio.speech", RunInput{}); err == nil {
		t.Fatal("missing output usage accepted")
	}
}

func TestMediaBillingAcceptsSynchronizedSelectors(t *testing.T) {
	for _, tc := range []struct {
		op, rates string
		input     RunInput
		usage     pr.MediaUsage
		want      int
	}{
		{"image.generate", `[{"id":1,"operation":"image.generate","usageKey":"output","selector":{"billing":"image","quality":"high","size":"1024x1024"},"unit":"image","unitQuantity":"1","billingUnitPriceUnits":90}]`, RunInput{Quality: "high", Size: "1024x1024"}, pr.MediaUsage{Known: true, ImageCount: 1}, 1},
		{"audio.speech", `[{"id":1,"operation":"audio.speech","usageKey":"input","selector":{"modality":"text"},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":10},{"id":2,"operation":"audio.speech","usageKey":"output","selector":{"modality":"audio"},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":90}]`, RunInput{}, pr.MediaUsage{Known: true, InputTokens: intPointer(2), OutputTokens: intPointer(3)}, 2},
		{"audio.transcribe", `[{"id":1,"operation":"audio.transcribe","usageKey":"input","selector":{"billing":"token"},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":10},{"id":2,"operation":"audio.transcribe","usageKey":"output","selector":{"billing":"token"},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":90}]`, RunInput{}, pr.MediaUsage{Known: true, InputTokens: intPointer(2), OutputTokens: intPointer(3)}, 2},
	} {
		t.Run(tc.op, func(t *testing.T) {
			s := fixtureSnapshot()
			s.Config = json.RawMessage(`{"rates":` + tc.rates + `}`)
			lines, err := mediaUsageLines(s, 9, tc.usage, tc.op, tc.input)
			if err != nil || len(lines) != tc.want {
				t.Fatalf("lines=%+v err=%v", lines, err)
			}
		})
	}
}

func intPointer(n int64) *int64 { return &n }

func TestUsageDigestSurvivesJSONBObjectOrdering(t *testing.T) {
	s := fixtureSnapshot()
	s.Config = json.RawMessage(`{"rates":[{"id":1,"operation":"text.generate","usageKey":"input","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":10},{"id":2,"operation":"text.generate","usageKey":"output","selector":{},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":90}]}`)
	a, err := textUsageLines(s, 9, pr.TokenUsage{Known: true, InputTokens: 2, OutputTokens: 3, Raw: json.RawMessage(`{"input_tokens":2,"output_tokens":3}`)})
	if err != nil {
		t.Fatal(err)
	}
	b, err := textUsageLines(s, 9, pr.TokenUsage{Known: true, InputTokens: 2, OutputTokens: 3, Raw: json.RawMessage(`{"output_tokens":3,"input_tokens":2}`)})
	if err != nil {
		t.Fatal(err)
	}
	if a[0].RawUsageDigest != b[0].RawUsageDigest {
		t.Fatal("JSONB key order changed usage digest")
	}
}

func TestAudioAdmissionSupportsActualSynchronizedPriceDimensions(t *testing.T) {
	for _, tc := range []struct {
		op, rates string
		input     RunInput
	}{
		{"audio.speech", `[{"id":1,"operation":"audio.speech","usageKey":"input","selector":{"modality":"text"},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":10},{"id":2,"operation":"audio.speech","usageKey":"output","selector":{"modality":"audio"},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":90}]`, RunInput{Text: "你好", Voice: "alloy"}},
		{"audio.transcribe", `[{"id":1,"operation":"audio.transcribe","usageKey":"input","selector":{"billing":"token"},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":10},{"id":2,"operation":"audio.transcribe","usageKey":"output","selector":{"billing":"token"},"unit":"token","unitQuantity":"1000000","billingUnitPriceUnits":90}]`, RunInput{FileIDs: []string{"1"}}},
	} {
		t.Run(tc.op, func(t *testing.T) {
			s := fixtureSnapshot()
			s.Config = json.RawMessage(`{"capabilities":["audio"],"operations":["` + tc.op + `"],"limits":{"maxInputTokens":1000,"maxOutputTokens":100},"rates":` + tc.rates + `}`)
			estimate, variable, _, err := estimateRun(s, tc.op, tc.input, Policy{AudioTimeoutSeconds: 60, MaxOutputTokens: 100})
			if err != nil || !variable || estimate < 1 {
				t.Fatalf("estimate=%d variable=%v err=%v", estimate, variable, err)
			}
		})
	}
}
