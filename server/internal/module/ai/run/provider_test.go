package run

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderStreamOpenAIParsesDeltasAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "demo" || body["stream"] != true {
			t.Fatalf("body = %#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":8,\"prompt_tokens_details\":{\"cached_tokens\":30}}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewProviderClient(server.Client())
	chunks, err := client.Stream(context.Background(), ProviderRequest{
		Protocol: 1, BaseURL: server.URL, APIKey: "secret", Model: "demo",
		SystemPrompt: "system", Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	var usage *ChatUsage
	done := false
	for chunk := range chunks {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content.WriteString(chunk.Delta)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		done = done || chunk.Done
	}
	if content.String() != "你好" || !done {
		t.Fatalf("content=%q done=%v", content.String(), done)
	}
	if usage == nil || usage.UncachedInput != 70 || usage.CacheRead != 30 || usage.Output != 8 {
		t.Fatalf("usage = %#v", usage)
	}
}
