package run

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderOpenAIToolCalls(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"system.user_count","arguments":"{}"}}]}}]}

`))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	chunks, err := NewProviderClient(server.Client()).Stream(context.Background(), ProviderRequest{
		Protocol: 1, BaseURL: server.URL, APIKey: "secret", Model: "demo",
		Messages: []ChatMessage{{Role: "user", Content: "多少用户"}},
		Tools:    []ToolDefinition{{Name: "system.user_count", Description: "count", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls []ToolCall
	for chunk := range chunks {
		calls = append(calls, chunk.ToolCalls...)
	}
	if len(calls) != 1 || calls[0].ID != "call_1" || calls[0].Name != "system.user_count" || calls[0].Arguments != "{}" {
		t.Fatalf("calls = %#v", calls)
	}
	if tools, ok := payload["tools"].([]any); !ok || len(tools) != 1 {
		t.Fatalf("tools payload = %#v", payload["tools"])
	}
}

func TestProviderAnthropicToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" || r.Header.Get("x-api-key") != "secret" {
			t.Fatalf("path=%s headers=%v", r.URL.Path, r.Header)
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if _, ok := payload["tools"].([]any); !ok {
			t.Fatalf("tools = %#v", payload["tools"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10}}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"system.server_overview\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{}\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"先查\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"好的\"}}\n\n"))
		_, _ = w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":3}}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	chunks, err := NewProviderClient(server.Client()).Stream(context.Background(), ProviderRequest{
		Protocol: 2, BaseURL: server.URL, APIKey: "secret", Model: "claude",
		SystemPrompt: "system", Messages: []ChatMessage{{Role: "user", Content: "服务器"}},
		Tools: []ToolDefinition{{Name: "system.server_overview", Description: "overview", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var content, reasoning strings.Builder
	var calls []ToolCall
	var usage *ChatUsage
	for chunk := range chunks {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content.WriteString(chunk.Delta)
		reasoning.WriteString(chunk.Reasoning)
		calls = append(calls, chunk.ToolCalls...)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	if content.String() != "好的" || reasoning.String() != "先查" {
		t.Fatalf("content=%q reasoning=%q", content.String(), reasoning.String())
	}
	if len(calls) != 1 || calls[0].ID != "toolu_1" || calls[0].Name != "system.server_overview" || calls[0].Arguments != "{}" {
		t.Fatalf("calls = %#v", calls)
	}
	if usage == nil || usage.UncachedInput != 10 || usage.Output != 3 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestProviderGeminiToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/models/demo:streamGenerateContent") || r.URL.Query().Get("alt") != "sse" {
			t.Fatalf("url = %s", r.URL.String())
		}
		if r.Header.Get("x-goog-api-key") != "secret" {
			t.Fatalf("headers = %v", r.Header)
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if _, ok := payload["tools"].([]any); !ok {
			t.Fatalf("tools = %#v", payload["tools"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"candidates":[{"content":{"parts":[{"thought":true,"text":"先想"},{"text":"查询"},{"functionCall":{"name":"system.user_count","args":{}}}]}}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":4,"cachedContentTokenCount":2}}

`))
	}))
	defer server.Close()

	chunks, err := NewProviderClient(server.Client()).Stream(context.Background(), ProviderRequest{
		Protocol: 3, BaseURL: server.URL, APIKey: "secret", Model: "demo",
		SystemPrompt: "system", Messages: []ChatMessage{{Role: "user", Content: "用户"}},
		Tools: []ToolDefinition{{Name: "system.user_count", Description: "count", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var content, reasoning strings.Builder
	var calls []ToolCall
	var usage *ChatUsage
	for chunk := range chunks {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content.WriteString(chunk.Delta)
		reasoning.WriteString(chunk.Reasoning)
		calls = append(calls, chunk.ToolCalls...)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	if content.String() != "查询" || reasoning.String() != "先想" {
		t.Fatalf("content=%q reasoning=%q", content.String(), reasoning.String())
	}
	if len(calls) != 1 || calls[0].Name != "system.user_count" || calls[0].Arguments != "{}" {
		t.Fatalf("calls = %#v", calls)
	}
	if usage == nil || usage.UncachedInput != 10 || usage.CacheRead != 2 || usage.Output != 4 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestProviderResponsesToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if _, ok := payload["tools"].([]any); !ok {
			t.Fatalf("tools = %#v", payload["tools"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"正在查\"}\n\n"))
		_, _ = w.Write([]byte("event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"system.user_count\"}}\n\n"))
		_, _ = w.Write([]byte("event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"item_1\",\"delta\":\"{}\"}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"system.user_count\",\"arguments\":\"{}\"}],\"usage\":{\"input_tokens\":20,\"output_tokens\":5,\"input_tokens_details\":{\"cached_tokens\":4}}}}\n\n"))
	}))
	defer server.Close()

	chunks, err := NewProviderClient(server.Client()).Stream(context.Background(), ProviderRequest{
		Protocol: 4, BaseURL: server.URL, APIKey: "secret", Model: "gpt-5",
		SystemPrompt: "system", Messages: []ChatMessage{{Role: "user", Content: "用户"}},
		Tools: []ToolDefinition{{Name: "system.user_count", Description: "count", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	var calls []ToolCall
	var usage *ChatUsage
	for chunk := range chunks {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content.WriteString(chunk.Delta)
		calls = append(calls, chunk.ToolCalls...)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}
	if content.String() != "正在查" || len(calls) != 1 || calls[0].ID != "call_1" || calls[0].Arguments != "{}" {
		t.Fatalf("content=%q calls=%#v", content.String(), calls)
	}
	if usage == nil || usage.UncachedInput != 16 || usage.CacheRead != 4 || usage.Output != 5 {
		t.Fatalf("usage = %#v", usage)
	}
}
