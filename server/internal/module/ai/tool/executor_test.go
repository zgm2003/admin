package tool

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func builtinVersion(key string) Version {
	return Version{Type: TypeBuiltin, BuiltinKey: &key, InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"expression":{"type":"string"}},"required":["expression"]}`), OutputSchema: json.RawMessage(`{"type":"object"}`), TimeoutMS: 1000, OutputLimitBytes: 65536}
}
func TestCalculatorUsesBoundedArithmeticParser(t *testing.T) {
	e := NewExecutor(nil, nil)
	v := builtinVersion("calculator")
	out, err := e.Execute(context.Background(), v, json.RawMessage(`{"expression":"(1 + 2)* 3 / 2 - -1"}`))
	if err != nil || string(out) != `{"result":"5.5"}` {
		t.Fatalf("result=%s err=%v", out, err)
	}
	for _, exp := range []string{"", "1/0", "Math.pow(2,3)", "__import__('os')", strings.Repeat("(", 65) + "1" + strings.Repeat(")", 65), strings.Repeat("1+", 2048) + "1"} {
		b, _ := json.Marshal(map[string]string{"expression": exp})
		if _, err := e.Execute(context.Background(), v, b); err == nil {
			t.Fatalf("unsafe expression accepted %q", exp)
		}
	}
}
func TestCurrentTimeValidatesTimezoneAndOutputSchema(t *testing.T) {
	v := builtinVersion("currentTime")
	v.InputSchema = json.RawMessage(`{"type":"object","properties":{"timezone":{"type":"string"}},"required":["timezone"],"additionalProperties":false}`)
	e := NewExecutor(nil, nil)
	before := time.Now()
	out, err := e.Execute(context.Background(), v, json.RawMessage(`{"timezone":"Asia/Shanghai"}`))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Time     string `json:"time"`
		Timezone string `json:"timezone"`
	}
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	tm, err := time.Parse(time.RFC3339Nano, got.Time)
	if err != nil || tm.Before(before.Add(-time.Second)) || got.Timezone != "Asia/Shanghai" {
		t.Fatalf("bad time %s %v", out, err)
	}
	if _, err = e.Execute(context.Background(), v, json.RawMessage(`{"timezone":"NOT/TIMEZONE"}`)); err == nil {
		t.Fatal("invalid timezone accepted")
	}
	v.OutputLimitBytes = 1
	if _, err = e.Execute(context.Background(), v, json.RawMessage(`{"timezone":"UTC"}`)); err == nil {
		t.Fatal("output limit ignored")
	}
}

type doFunc func(*http.Request) (*http.Response, error)

func (f doFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }
func TestHTTPUsesFixedTargetMappingAndLimitsResponse(t *testing.T) {
	v := Version{Type: TypeHTTP, InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"],"additionalProperties":false}`), OutputSchema: json.RawMessage(`{"type":"object"}`), HTTPDefinition: json.RawMessage(`{"url":"https://example.com/search","method":"POST","requestMapping":{"q":"query"},"responseMapping":{"answer":"result"}}`), TimeoutMS: 100, OutputLimitBytes: 65536}
	e := NewExecutor(doFunc(func(r *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(r.Body)
		if r.URL.String() != "https://example.com/search" || string(b) != `{"q":"hello"}` {
			t.Fatalf("unexpected request %s %s", r.URL, b)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"result":"ok"}`))}, nil
	}), nil)
	out, err := e.Execute(context.Background(), v, json.RawMessage(`{"query":"hello"}`))
	if err != nil || string(out) != `{"answer":"ok"}` {
		t.Fatalf("out=%s err=%v", out, err)
	}
	if _, err = e.Execute(context.Background(), v, json.RawMessage(`{"query":"hello","url":"https://evil.com"}`)); err == nil {
		t.Fatal("model target injection accepted")
	}
	e = NewExecutor(doFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() }), nil)
	_, err = e.Execute(context.Background(), v, json.RawMessage(`{"query":"hello"}`))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout not enforced: %v", err)
	}
}
