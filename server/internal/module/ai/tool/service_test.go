package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolDefinitionsUseProviderSafeNames(t *testing.T) {
	service := NewService(nil, nil, nil, nil)
	for code, definition := range service.defs {
		if definition.APIName == "" {
			t.Fatalf("tool %s has no API name", code)
		}
		if strings.Contains(definition.APIName, ".") {
			t.Fatalf("tool API name %q contains a dot", definition.APIName)
		}
		if apiNameFor(code) != definition.APIName {
			t.Fatalf("API name %q does not match %q", definition.APIName, apiNameFor(code))
		}
		parameters, ok := definition.Parameters["type"].(string)
		if !ok || parameters != "object" {
			t.Fatalf("tool %s parameters = %#v", code, definition.Parameters)
		}
	}
}

func TestHTTPExecutorSendsJSONArguments(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("request = %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	service := NewService(nil, nil, nil, nil)
	executor, err := service.httpExecutor(Tool{
		Code: "system.lookup", Kind: KindHTTP, Method: http.MethodPost,
		URL: server.URL, TimeoutMS: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := executor(context.Background(), json.RawMessage(`{"id":7}`), Invocation{})
	if err != nil {
		t.Fatal(err)
	}
	if received["id"] != json.Number("7") && received["id"] != float64(7) {
		t.Fatalf("received = %#v", received)
	}
	payload, _ := json.Marshal(result)
	if string(payload) != `{"ok":true}` {
		t.Fatalf("result = %s", payload)
	}
}

func TestNormalizeParametersRejectsNonObjectRoot(t *testing.T) {
	if _, err := normalizeParameters(json.RawMessage(`{"type":"array"}`)); err == nil {
		t.Fatal("array schema was accepted")
	}
	if _, err := normalizeParameters(json.RawMessage(`{"type":"object","properties":{}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeHTTPToolInput(t *testing.T) {
	if _, err := normalizeInput("Demo", "", "", json.RawMessage(`{"type":"object","properties":{}}`),
		"TRACE", "https://example.com", 5000, StatusEnabled, 0, true); err == nil {
		t.Fatal("unsupported method was accepted")
	}
	if _, err := normalizeInput("Demo", "", "", json.RawMessage(`{"type":"object","properties":{}}`),
		"GET", "ftp://example.com", 5000, StatusEnabled, 0, true); err == nil {
		t.Fatal("non-http url was accepted")
	}
	values, err := normalizeInput("  Demo  ", "  description ", "internal",
		json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
		"get", "https://example.com/api?x=1", 5000, StatusEnabled, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if values.Name != "Demo" || values.Method != "GET" || values.URL != "https://example.com/api?x=1" {
		t.Fatalf("values = %+v", values)
	}
}

func TestStatusRequest(t *testing.T) {
	enabled := int16(1)
	status, err := (statusRequest{Status: &enabled}).input()
	if err != nil || status != StatusEnabled {
		t.Fatalf("status=%v err=%v", status, err)
	}
	invalid := int16(2)
	if _, err := (statusRequest{Status: &invalid}).input(); err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestExecuteRejectsUnknownTool(t *testing.T) {
	service := NewService(nil, nil, nil, nil)
	if _, err := service.Execute(context.Background(), "unknown", "{}", Invocation{}); err == nil {
		t.Fatal("expected unknown tool error")
	}
}
