package providerrequest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsesStreamingPersistsDeltasBeforeTerminalUsage(t *testing.T) {
	var deltas []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		json.NewDecoder(r.Body).Decode(&body)
		if string(body["stream"]) != "true" {
			t.Errorf("stream=%s", body["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"a\"}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"b\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"r\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"ab\"}]}],\"usage\":{\"input_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":0},\"output_tokens\":2}}}\n\n")
	}))
	defer server.Close()
	in := textRequest(server.URL)
	in.OnTextDelta = func(value string) error { deltas = append(deltas, value); return nil }
	result, e := NewResponsesClient(server.Client()).Generate(context.Background(), in)
	if e != nil || result.Text != "ab" || !result.Usage.Known || strings.Join(deltas, "") != "ab" {
		t.Fatalf("%+v %v deltas=%v", result, e, deltas)
	}
}
func TestChatStreamingAssemblesToolFragmentsAndFinalUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call1\",\"type\":\"function\",\"function\":{\"name\":\"clock\",\"arguments\":\"{\"}}]}}]}\n\ndata: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: {\"id\":\"c\",\"choices\":[],\"usage\":{\"prompt_tokens\":2,\"prompt_tokens_details\":{\"cached_tokens\":0},\"completion_tokens\":1}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	in := textRequest(server.URL)
	in.OnTextDelta = func(string) error { return nil }
	result, e := NewChatCompletionsClient(server.Client()).Generate(context.Background(), in)
	if e != nil || len(result.Calls) != 1 || result.Calls[0].Arguments != "{}" || !result.Usage.Known {
		t.Fatalf("%+v %v", result, e)
	}
}
func TestStreamingLossNeverLooksLikeConfirmedZeroUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n")
	}))
	defer server.Close()
	in := textRequest(server.URL)
	in.OnTextDelta = func(string) error { return nil }
	result, e := NewResponsesClient(server.Client()).Generate(context.Background(), in)
	if e == nil || !errors.Is(e, ErrSubmissionUnknown) || result.Usage.Known {
		t.Fatalf("%+v %v", result, e)
	}
}
