package run

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderStreamOpenAISendsImageContentParts(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewProviderClient(server.Client())
	chunks, err := client.Stream(context.Background(), ProviderRequest{
		Protocol: 1, BaseURL: server.URL, APIKey: "secret", Model: "demo",
		Messages: []ChatMessage{{
			Role: "user", Content: "看图",
			Attachments: []ChatAttachment{{URL: "https://example.test/a.png", IsImage: true}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range chunks {
	}
	messages, ok := received["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %#v", received["messages"])
	}
	message, _ := messages[0].(map[string]any)
	parts, ok := message["content"].([]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("content parts = %#v", message["content"])
	}
	imagePart, _ := parts[1].(map[string]any)
	if imagePart["type"] != "image_url" {
		t.Fatalf("image part = %#v", imagePart)
	}
}

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
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"先分析\"}}]}\n\n"))
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
	var content, reasoning strings.Builder
	var usage *ChatUsage
	done := false
	for chunk := range chunks {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content.WriteString(chunk.Delta)
		reasoning.WriteString(chunk.Reasoning)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		done = done || chunk.Done
	}
	if content.String() != "你好" || reasoning.String() != "先分析" || !done {
		t.Fatalf("content=%q reasoning=%q done=%v", content.String(), reasoning.String(), done)
	}
	if usage == nil || usage.UncachedInput != 70 || usage.CacheRead != 30 || usage.Output != 8 {
		t.Fatalf("usage = %#v", usage)
	}
}
