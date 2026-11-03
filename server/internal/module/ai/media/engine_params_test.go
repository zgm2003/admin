package media

import (
	"encoding/json"
	"testing"

	"admin/server/internal/module/ai"
	airun "admin/server/internal/module/ai/run"
)

func TestFrozenMediaParamsMergesAgentAndBindingWithBindingPriority(t *testing.T) {
	config := airun.RuntimeConfig{
		GenerationParams: json.RawMessage(`{"speed":1.1,"background":"transparent","temperature":0.2}`),
		Parameters:       json.RawMessage(`{"speed":1.4,"temperature":0.6}`),
	}
	raw, err := FrozenMediaParams(config)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got["speed"] != 1.4 || got["temperature"] != 0.6 || got["background"] != "transparent" {
		t.Fatalf("frozen media parameters=%s", raw)
	}
}

func TestFrozenMediaParamsRejectsMalformedAndReservedParameters(t *testing.T) {
	for _, raw := range []string{`{`, `null`, `[]`, `"value"`, `{"model":"override"}`, `{"prompt":"override"}`, `{"n":4}`, `{"quality":"low"}`, `{"size":"auto"}`, `{"output_format":"webp"}`, `{"image":"override"}`, `{"file":"override"}`, `{"input":"override"}`, `{"voice":"override"}`, `{"response_format":"pcm"}`, `{"language":"en"}`, `{"instructions":"override"}`} {
		for _, binding := range []bool{false, true} {
			t.Run(raw+map[bool]string{false: "/agent", true: "/binding"}[binding], func(t *testing.T) {
				config := airun.RuntimeConfig{GenerationParams: json.RawMessage(`{}`), Parameters: json.RawMessage(`{}`)}
				if binding {
					config.Parameters = json.RawMessage(raw)
				} else {
					config.GenerationParams = json.RawMessage(raw)
				}
				if result, err := FrozenMediaParams(config); err == nil || len(result) != 0 {
					t.Fatalf("invalid frozen parameters accepted: %s err=%v", result, err)
				}
			})
		}
	}
}

func TestFrozenMediaParamsSpeechKeepsMergedSpeedAndForcesUsageSSE(t *testing.T) {
	config := airun.RuntimeConfig{
		GenerationParams: json.RawMessage(`{"speed":1.1,"stream_format":"audio"}`),
		Parameters:       json.RawMessage(`{"speed":1.4}`),
		Rates:            []airun.RuntimeRate{{Operation: string(ai.OperationAudio), Unit: "token"}},
	}
	raw, err := FrozenMediaParams(config)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(speechParams(config, raw), &got); err != nil {
		t.Fatal(err)
	}
	if got["speed"] != 1.4 || got["stream_format"] != "sse" {
		t.Fatalf("speech parameters=%v", got)
	}
}
