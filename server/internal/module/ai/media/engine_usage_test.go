package media

import (
	"admin/server/internal/module/ai"
	pr "admin/server/internal/module/ai/providerRequest"
	airun "admin/server/internal/module/ai/run"
	"encoding/json"
	"testing"
)

func TestSpeechUsageCountsUnicodeCharacters(t *testing.T) {
	cfg := airun.RuntimeConfig{Rates: []airun.RuntimeRate{{Operation: string(ai.OperationAudio), Unit: "character"}}}
	u := speechUsage(cfg, "你好🙂", pr.MediaUsage{})
	if !u.Known || u.TextInputTokens == nil || *u.TextInputTokens != 3 {
		t.Fatalf("usage=%+v", u)
	}
}
func TestSpeechTokenPricingRequestsFinalSSEUsage(t *testing.T) {
	cfg := airun.RuntimeConfig{Rates: []airun.RuntimeRate{{Operation: string(ai.OperationAudio), Unit: "token"}}}
	var params map[string]any
	if err := json.Unmarshal(speechParams(cfg, []byte(`{"speed":1.2}`)), &params); err != nil {
		t.Fatal(err)
	}
	if params["stream_format"] != "sse" || params["speed"] != 1.2 {
		t.Fatal(params)
	}
	if speechUsage(cfg, "你好", pr.MediaUsage{}).Known {
		t.Fatal("invented token usage")
	}
}

func TestZeroMediaUsageExplicitlyCarriesEveryCounter(t *testing.T) {
	u := zeroMediaUsage()
	if !u.Known || u.ImageCount != 0 || u.DurationSeconds != "0" {
		t.Fatal(u)
	}
	for _, v := range []*int64{u.InputTokens, u.OutputTokens, u.AudioInputTokens, u.TextInputTokens} {
		if v == nil || *v != 0 {
			t.Fatal(u)
		}
	}
}
