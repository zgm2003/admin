package providerrequest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestChatCompletionsClientPreservesNativeToolContinuationAndUsage(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path=%s", r.URL.Path)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		var messages []map[string]json.RawMessage
		_ = json.Unmarshal(body["messages"], &messages)
		if string(messages[0]["role"]) != `"system"` || string(messages[0]["content"]) != `"frozen prompt"` {
			t.Errorf("prompt=%s", body["messages"])
		}
		if string(body["stream"]) != "false" || string(body["max_completion_tokens"]) != "4096" {
			t.Errorf("budget=%v", body)
		}
		if count.Add(1) == 1 {
			var tools []map[string]json.RawMessage
			_ = json.Unmarshal(body["tools"], &tools)
			if len(tools) != 1 || len(tools[0]["function"]) == 0 {
				t.Errorf("tools=%s", body["tools"])
			}
			io.WriteString(w, `{"id":"chat_1","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"reasoning_content":"OPAQUE","tool_calls":[{"id":"call_A","type":"function","function":{"name":"current_time","arguments":"{}"}}]}}],"usage":{"prompt_tokens":50,"prompt_tokens_details":{"cached_tokens":10},"completion_tokens":4,"total_tokens":54}}`)
			return
		}
		if len(messages) != 4 || string(messages[2]["reasoning_content"]) != `"OPAQUE"` || string(messages[3]["role"]) != `"tool"` || string(messages[3]["tool_call_id"]) != `"call_A"` {
			t.Errorf("tool continuation=%s", body["messages"])
		}
		io.WriteString(w, `{"id":"chat_2","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"done"}}],"usage":{"prompt_tokens":75,"prompt_tokens_details":{"cached_tokens":20},"completion_tokens":8,"total_tokens":83}}`)
	}))
	defer server.Close()
	request := textRequest(server.URL + "/v1")
	request.Tools = []FunctionTool{{Name: "current_time", Parameters: json.RawMessage(`{"type":"object","properties":{}}`)}}
	client := NewChatCompletionsClient(server.Client())
	first, err := client.Generate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Calls) != 1 || first.Calls[0].CallID != "call_A" || !first.Usage.Known || first.Usage.CachedInputTokens != 10 {
		t.Fatalf("first=%+v", first)
	}
	request.Input = append(request.Input, first.OutputItems...)
	request.Input = append(request.Input, json.RawMessage(`{"role":"tool","tool_call_id":"call_A","content":"tool result"}`))
	second, err := client.Generate(context.Background(), request)
	if err != nil || second.Text != "done" || !second.Usage.Known || count.Load() != 2 {
		t.Fatalf("response=%+v err=%v calls=%d", second, err, count.Load())
	}
}

func TestChatCompletionsClientMissingUsageIsUnknownAndLengthKeepsUsage(t *testing.T) {
	for _, tc := range []struct {
		reason, usage string
		known         bool
	}{
		{"stop", "", false},
		{"length", `,"usage":{"prompt_tokens":5,"prompt_tokens_details":{"cached_tokens":1},"completion_tokens":2,"total_tokens":7}`, true},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"id":"chat_1","choices":[{"finish_reason":"`+tc.reason+`","message":{"role":"assistant","content":"partial"}}]`+tc.usage+`}`)
		}))
		response, err := NewChatCompletionsClient(server.Client()).Generate(context.Background(), textRequest(server.URL))
		server.Close()
		if response.Usage.Known != tc.known || response.Text != "partial" {
			t.Fatalf("response=%+v err=%v", response, err)
		}
		if tc.reason == "length" && !errors.Is(err, ErrResponseIncomplete) {
			t.Fatalf("length err=%v", err)
		}
		if tc.reason == "stop" && err != nil {
			t.Fatal(err)
		}
	}
}
