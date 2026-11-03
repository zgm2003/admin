package providerrequest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func textRequest(baseURL string) TextRequest {
	return TextRequest{Config: FrozenConfig{BaseURL: baseURL, APIKey: "TOKEN", Model: "MODEL", Timeout: time.Second}, Prompt: "frozen prompt", Input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"question"}`)}, MaxOutputTokens: 4096}
}

func TestResponsesClientPreservesReasoningAndCallIDAcrossToolContinuation(t *testing.T) {
	const reasoning = `{"type":"reasoning","id":"rs_1","summary":[],"encrypted_content":"OPAQUE_REASONING","vendor_extension":{"preserve":true}}`
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer TOKEN" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if string(body["stream"]) != "false" || string(body["store"]) != "false" || string(body["model"]) != `"MODEL"` || string(body["instructions"]) != `"frozen prompt"` {
			t.Errorf("frozen request fields=%v", body)
		}
		var input []json.RawMessage
		if err := json.Unmarshal(body["input"], &input); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req_TEST")
		if calls.Add(1) == 1 {
			var tools []map[string]json.RawMessage
			if err := json.Unmarshal(body["tools"], &tools); err != nil || len(tools) != 1 || string(tools[0]["type"]) != `"function"` || string(tools[0]["name"]) != `"current_time"` {
				t.Errorf("responses tools=%s err=%v", body["tools"], err)
			}
			io.WriteString(w, `{"id":"resp_1","status":"completed","output":[`+reasoning+`,{"type":"function_call","id":"fc_1","call_id":"call_A","name":"current_time","arguments":"{}"}],"usage":{"input_tokens":50,"input_tokens_details":{"cached_tokens":10},"output_tokens":5,"total_tokens":55}}`)
			return
		}
		if len(input) != 4 || string(input[1]) != reasoning {
			t.Errorf("reasoning/complete continuation input=%s", body["input"])
		}
		var result struct{ Type, CallID, Output string }
		var raw map[string]string
		_ = json.Unmarshal(input[3], &raw)
		result.Type, result.CallID, result.Output = raw["type"], raw["call_id"], raw["output"]
		if result.Type != "function_call_output" || result.CallID != "call_A" || result.Output != "tool result" {
			t.Errorf("continuation=%s", input[3])
		}
		io.WriteString(w, `{"id":"resp_2","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}],"usage":{"input_tokens":75,"input_tokens_details":{"cached_tokens":20},"output_tokens":8,"total_tokens":83}}`)
	}))
	defer server.Close()
	request := textRequest(server.URL + "/v1")
	request.Tools = []FunctionTool{{Name: "current_time", Parameters: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`), Strict: true}}
	client := NewResponsesClient(server.Client())
	first, err := client.Generate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Usage.Known || first.Usage.InputTokens != 50 || first.Usage.CachedInputTokens != 10 || first.Usage.OutputTokens != 5 || first.RequestID != "req_TEST" {
		t.Fatalf("response=%+v", first)
	}
	if len(first.Calls) != 1 || first.Calls[0].CallID != "call_A" || first.Calls[0].Name != "current_time" || first.Calls[0].Arguments != "{}" {
		t.Fatalf("calls=%+v", first.Calls)
	}
	request.Input = append(request.Input, first.OutputItems...)
	request.Input = append(request.Input, json.RawMessage(`{"type":"function_call_output","call_id":"call_A","output":"tool result"}`))
	second, err := client.Generate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if second.Text != "done" || second.ID != "resp_2" || !second.Usage.Known || second.Usage.InputTokens != 75 || calls.Load() != 2 {
		t.Fatalf("response=%+v calls=%d", second, calls.Load())
	}
}

func TestResponsesClientKeepsMissingOrMalformedUsageUnknown(t *testing.T) {
	for _, usage := range []string{"", `,"usage":null`, `,"usage":{}`, `,"usage":{"input_tokens":5,"output_tokens":1}`, `,"usage":{"input_tokens":5,"input_tokens_details":{"cached_tokens":6},"output_tokens":1}`, `,"usage":{"input_tokens":-1,"input_tokens_details":{"cached_tokens":0},"output_tokens":1}`, `,"usage":{"input_tokens":1.5,"input_tokens_details":{"cached_tokens":0},"output_tokens":1}`, `,"usage":{"input_tokens":5,"input_tokens_details":{"cached_tokens":0},"output_tokens":1,"total_tokens":7}`} {
		t.Run(usage, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `{"id":"resp_1","status":"completed","output":[]`+usage+`}`)
			}))
			defer server.Close()
			response, err := NewResponsesClient(server.Client()).Generate(context.Background(), textRequest(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			if response.Usage.Known {
				t.Fatalf("invented reliable usage from %s", usage)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":"resp_1","status":"completed","output":[],"usage":{"input_tokens":0,"input_tokens_details":{"cached_tokens":0},"output_tokens":0,"total_tokens":0}}`)
	}))
	defer server.Close()
	response, err := NewResponsesClient(server.Client()).Generate(context.Background(), textRequest(server.URL))
	if err != nil || !response.Usage.Known || !reflect.DeepEqual(response.Calls, []FunctionCall{}) {
		t.Fatalf("known zero response=%+v err=%v", response, err)
	}
}

func TestResponsesClientTreatsReasoningPayloadAsOpaque(t *testing.T) {
	const opaque = `{"type":"reasoning","encrypted_content":"OPAQUE","content":{"private_encoding":[1,2,3]},"summary":[]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":"resp_1","status":"completed","output":[`+opaque+`]}`)
	}))
	defer server.Close()
	response, err := NewResponsesClient(server.Client()).Generate(context.Background(), textRequest(server.URL))
	if err != nil || len(response.OutputItems) != 1 || string(response.OutputItems[0]) != opaque {
		t.Fatalf("response=%+v err=%v", response, err)
	}
}

func TestResponsesClientKeepsIncompleteUsageAndUncertainStatusSeparate(t *testing.T) {
	for _, tc := range []struct {
		body           string
		known, unknown bool
	}{
		{`{"id":"resp_1","status":"incomplete","output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}],"usage":{"input_tokens":2,"input_tokens_details":{"cached_tokens":0},"output_tokens":1,"total_tokens":3}}`, true, false},
		{`{"id":"resp_1","status":"failed","output":[]}`, false, false},
		{`{"id":"resp_1","status":"in_progress"}`, false, true},
		{`{"id":"resp_1","status":"completed","output":[`, false, true},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, tc.body) }))
		response, err := NewResponsesClient(server.Client()).Generate(context.Background(), textRequest(server.URL))
		server.Close()
		var requestError *RequestError
		if !errors.As(err, &requestError) || requestError.SubmissionUnknown != tc.unknown || response.Usage.Known != tc.known {
			t.Fatalf("body=%s usageKnown=%v error=%+v", tc.body, response.Usage.Known, requestError)
		}
	}
}
