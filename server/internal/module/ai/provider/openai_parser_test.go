package provider

import "testing"

func TestPricingParserRejectsMalformedRecognizedPrice(t *testing.T) {
	for _, v := range []string{"$NaN", "$1e2", "$1/2", "contact us"} {
		raw := "### Standard pricing data\n| Model | Input | Output | Cached input |\n| --- | --- | --- | --- |\n| fixture | $1.00 | " + v + " | $0.1 |\n### Batch pricing data\n"
		if _, err := ParsePricingMarkdown([]byte(raw)); err == nil {
			t.Fatalf("malformed price %s accepted", v)
		}
	}
}

func TestPricingParserKeepsCacheWriteDimensions(t *testing.T) {
	raw := "### Standard pricing data\n| Model | Short context input | Short context output | Short context cached input | Short context 30m write | Short context 12h write |\n| --- | --- | --- | --- | --- | --- |\n| fixture | $1 | $2 | $0.1 | $1.25 | $2 |\n### Batch pricing data\n"
	rates, err := ParsePricingMarkdown([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 5 {
		t.Fatalf("cache write prices discarded: %v", rates)
	}
}
