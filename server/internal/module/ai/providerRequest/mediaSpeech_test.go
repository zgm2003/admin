package providerrequest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSpeechSSEDecodesAudioAndFinalUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"type\":\"speech.audio.delta\",\"audio\":\"SUQz\"}\n\ndata: {\"type\":\"speech.audio.done\",\"usage\":{\"input_tokens\":14,\"output_tokens\":101,\"total_tokens\":115}}\n\n")
	}))
	defer srv.Close()
	out, err := NewAudioClient(srv.Client()).Synthesize(context.Background(), SpeechRequest{Config: FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "gpt-4o-mini-tts", Timeout: time.Second}, Input: "hello", Voice: "v", Params: []byte(`{"stream_format":"sse"}`)})
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	raw, err := io.ReadAll(out.Body)
	if err != nil || string(raw) != "ID3" {
		t.Fatalf("audio=%q err=%v", raw, err)
	}
	source, ok := out.Body.(interface{ Usage() MediaUsage })
	if !ok {
		t.Fatal("final usage missing")
	}
	usage := source.Usage()
	if !usage.Known || usage.InputTokens == nil || *usage.InputTokens != 14 || usage.OutputTokens == nil || *usage.OutputTokens != 101 {
		t.Fatalf("usage=%+v", usage)
	}
}
func TestTranscribeParsesRealTokenAndDurationUsage(t *testing.T) {
	for _, tc := range []struct {
		wire   string
		tokens bool
	}{{`{"text":"x","usage":{"type":"tokens","input_tokens":17,"output_tokens":2,"total_tokens":19,"input_token_details":{"audio_tokens":12,"text_tokens":5}}}`, true}, {`{"text":"x","usage":{"type":"duration","seconds":2.5}}`, false}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.Copy(io.Discard, r.Body); io.WriteString(w, tc.wire) }))
		out, err := NewAudioClient(srv.Client()).Transcribe(context.Background(), TranscriptionRequest{Config: FrozenConfig{BaseURL: srv.URL, APIKey: "k", Model: "m", Timeout: time.Second}, Audio: strings.NewReader("audio"), MIME: "audio/mpeg"})
		srv.Close()
		if err != nil || !out.Usage.Known {
			t.Fatalf("usage=%+v err=%v", out.Usage, err)
		}
		if tc.tokens {
			if out.Usage.InputTokens == nil || *out.Usage.InputTokens != 17 || out.Usage.AudioInputTokens == nil || *out.Usage.AudioInputTokens != 12 || out.Usage.TextInputTokens == nil || *out.Usage.TextInputTokens != 5 || out.Usage.OutputTokens == nil || *out.Usage.OutputTokens != 2 {
				t.Fatalf("usage=%+v", out.Usage)
			}
		} else if out.Usage.DurationSeconds != "2.5" {
			t.Fatal(out.Usage)
		}
	}
}

func TestSpeechSSERejectsMissingDoneMalformedAudioAndInvalidUsage(t *testing.T) {
	for _, wire := range []string{
		"data: {\"type\":\"speech.audio.delta\",\"audio\":\"SUQz\"}\n\n",
		"data: {\"type\":\"speech.audio.delta\",\"audio\":\"%%%\"}\n\n",
		"data: {\"type\":\"speech.audio.done\",\"usage\":{\"input_tokens\":-1,\"output_tokens\":2}}\n\n",
		"data: {\"type\":\"speech.audio.done\",\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"total_tokens\":9}}\n\n",
	} {
		body := newSpeechSSEBody(io.NopCloser(strings.NewReader(wire)))
		if _, err := io.ReadAll(body); err == nil {
			t.Fatalf("accepted %q", wire)
		}
		if body.Usage().Known {
			t.Fatalf("invalid usage=%+v", body.Usage())
		}
		body.Close()
	}
}
func TestAudioUsageDoesNotSubstituteDurationForMissingTokens(t *testing.T) {
	u := parseAudioUsage([]byte(`{"type":"tokens","input_tokens":2}`), "2.5")
	if u.Known || u.DurationSeconds != "" {
		t.Fatalf("usage=%+v", u)
	}
}
