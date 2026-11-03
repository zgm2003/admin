package provider

import (
	"encoding/json"
	"testing"
)

func TestAudioPricingTokenFactsReplaceMinuteEstimates(t *testing.T) {
	rows, e := parseAudioPricing("| Model | Modality | Input | Output | Estimated cost |\n| gpt-4o-transcribe | Transcription | $2.50 | $10.00 | $0.006 / minute |\n| Whisper | Transcription | - | - | $0.006 / minute |\n")
	if e != nil {
		t.Fatal(e)
	}
	token, minute := 0, 0
	for _, r := range rows {
		var selector map[string]string
		if e = json.Unmarshal(r.Selector, &selector); e != nil {
			t.Fatal(e)
		}
		if r.ModelKey == "gpt-4o-transcribe" {
			if r.Unit != "token" || selector["billing"] != "token" {
				t.Fatalf("estimated cost became a second charge: %+v", r)
			}
			token++
		}
		if r.ModelKey == "Whisper" && r.Unit == "second" && r.UnitQuantity == "60" {
			minute++
		}
	}
	if token != 2 || minute != 1 {
		t.Fatalf("token=%d minute=%d", token, minute)
	}
}
func TestAudioPricingRejectsPartialTokenFactsInsteadOfUsingEstimate(t *testing.T) {
	for _, prices := range []string{"$2.50 | -", "$broken | $10.00"} {
		if _, e := parseAudioPricing("| gpt-4o-transcribe | Transcription | " + prices + " | $0.006 / minute |\n"); e == nil {
			t.Fatalf("partial token pricing accepted: %s", prices)
		}
	}
}

func TestRealtimeAudioTokenPricesDoNotAdvertiseSpeechEndpoint(t *testing.T) {
	rates, e := parseAudioPricing("| gpt-audio | Audio | $32.00 | $64.00 | - |\n| gpt-realtime | Audio | $32.00 | $64.00 | $0.40 |\n| gpt-live-transcribe | Live transcription | - | - | $0.017 / minute |\n")
	if e != nil {
		t.Fatal(e)
	}
	if len(rates) != 0 {
		t.Fatalf("unsupported endpoints advertised: %+v", rates)
	}
}
