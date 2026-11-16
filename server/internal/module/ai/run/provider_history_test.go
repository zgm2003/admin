package run

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func toolHistoryMessages() []ChatMessage {
	return []ChatMessage{
		{Role: "user", Content: "查用户数"},
		{Role: "assistant", Content: "我查一下", ToolCalls: []ToolCall{{ID: "call_1", Name: "system_user_count", Arguments: "{}"}}},
		{Role: "tool", Content: `{"total":3}`, ToolCallID: "call_1", Name: "system_user_count"},
	}
}

func testProviderServer(t *testing.T, protocol int, responseBody string, capture *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, capture)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(responseBody))
	}))
}

func TestOpenAIToolHistorySerialization(t *testing.T) {
	var payload map[string]any
	server := testProviderServer(t, 1, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n", &payload)
	defer server.Close()
	chunks, err := NewProviderClient(server.Client()).Stream(context.Background(), ProviderRequest{
		Protocol: 1, BaseURL: server.URL, APIKey: "k", Model: "m", Messages: toolHistoryMessages(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for range chunks {
	}
	messages, _ := payload["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("messages = %#v", payload["messages"])
	}
	assistant, _ := messages[1].(map[string]any)
	if _, ok := assistant["tool_calls"].([]any); !ok {
		t.Fatalf("assistant = %#v", assistant)
	}
	tool, _ := messages[2].(map[string]any)
	if tool["role"] != "tool" || tool["tool_call_id"] != "call_1" {
		t.Fatalf("tool = %#v", tool)
	}
}

func TestAnthropicToolHistorySerialization(t *testing.T) {
	var payload map[string]any
	server := testProviderServer(t, 2, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", &payload)
	defer server.Close()
	chunks, err := NewProviderClient(server.Client()).Stream(context.Background(), ProviderRequest{
		Protocol: 2, BaseURL: server.URL, APIKey: "k", Model: "m", Messages: toolHistoryMessages(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for range chunks {
	}
	messages, _ := payload["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("messages = %#v", payload["messages"])
	}
	assistant, _ := messages[1].(map[string]any)
	blocks, _ := assistant["content"].([]any)
	if len(blocks) != 2 {
		t.Fatalf("assistant blocks = %#v", blocks)
	}
	toolUse, _ := blocks[1].(map[string]any)
	if toolUse["type"] != "tool_use" || toolUse["id"] != "call_1" {
		t.Fatalf("tool use = %#v", toolUse)
	}
	toolResultMessage, _ := messages[2].(map[string]any)
	resultBlocks, _ := toolResultMessage["content"].([]any)
	result, _ := resultBlocks[0].(map[string]any)
	if result["type"] != "tool_result" || result["tool_use_id"] != "call_1" {
		t.Fatalf("tool result = %#v", result)
	}
}

func TestGeminiToolHistorySerialization(t *testing.T) {
	var payload map[string]any
	server := testProviderServer(t, 3, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]}}]}\n\n", &payload)
	defer server.Close()
	chunks, err := NewProviderClient(server.Client()).Stream(context.Background(), ProviderRequest{
		Protocol: 3, BaseURL: server.URL, APIKey: "k", Model: "m", Messages: toolHistoryMessages(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for range chunks {
	}
	contents, _ := payload["contents"].([]any)
	if len(contents) != 3 {
		t.Fatalf("contents = %#v", payload["contents"])
	}
	assistant, _ := contents[1].(map[string]any)
	parts, _ := assistant["parts"].([]any)
	callPart, _ := parts[1].(map[string]any)
	if _, ok := callPart["functionCall"]; !ok {
		t.Fatalf("assistant = %#v", assistant)
	}
	toolMessage, _ := contents[2].(map[string]any)
	toolParts, _ := toolMessage["parts"].([]any)
	responsePart, _ := toolParts[0].(map[string]any)
	if _, ok := responsePart["functionResponse"]; !ok {
		t.Fatalf("tool message = %#v", toolMessage)
	}
}

func TestResponsesToolHistorySerialization(t *testing.T) {
	var payload map[string]any
	server := testProviderServer(t, 4, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n", &payload)
	defer server.Close()
	chunks, err := NewProviderClient(server.Client()).Stream(context.Background(), ProviderRequest{
		Protocol: 4, BaseURL: server.URL, APIKey: "k", Model: "m", Messages: toolHistoryMessages(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for range chunks {
	}
	input, _ := payload["input"].([]any)
	if len(input) != 4 {
		t.Fatalf("input = %#v", payload["input"])
	}
	call, _ := input[2].(map[string]any)
	if call["type"] != "function_call" || call["call_id"] != "call_1" {
		t.Fatalf("call = %#v", call)
	}
	output, _ := input[3].(map[string]any)
	if output["type"] != "function_call_output" || output["call_id"] != "call_1" {
		t.Fatalf("output = %#v", output)
	}
}
