package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}
type SecretReader interface {
	Headers(context.Context, Version) (map[string]string, error)
}
type Executor struct {
	http   HTTPDoer
	secret SecretReader
}

func NewExecutor(h HTTPDoer, s SecretReader) *Executor {
	if h == nil {
		h = newHTTPClient()
	}
	return &Executor{http: h, secret: s}
}
func (e *Executor) Execute(ctx context.Context, v Version, args json.RawMessage) (json.RawMessage, error) {
	if err := validateSchemaValue(args, v.InputSchema); err != nil {
		return nil, fmt.Errorf("tool arguments: %w", err)
	}
	if v.TimeoutMS < 100 || v.TimeoutMS > 30000 || v.OutputLimitBytes < 1 || v.OutputLimitBytes > 65536 {
		return nil, errors.New("tool execution policy invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(v.TimeoutMS)*time.Millisecond)
	defer cancel()
	var out json.RawMessage
	var err error
	switch v.Type {
	case TypeBuiltin:
		if v.BuiltinKey == nil {
			return nil, errors.New("builtin key missing")
		}
		out, err = executeBuiltin(*v.BuiltinKey, args)
	case TypeHTTP:
		out, err = e.executeHTTP(ctx, v, args)
	default:
		err = errors.New("tool type invalid")
	}
	if err != nil {
		return nil, err
	}
	if len(out) > v.OutputLimitBytes || len(out) > 65536 {
		return nil, errors.New("tool output exceeds limit")
	}
	if err := validateSchemaValue(out, v.OutputSchema); err != nil {
		return nil, fmt.Errorf("tool output: %w", err)
	}
	return out, nil
}
func executeBuiltin(key string, args json.RawMessage) (json.RawMessage, error) {
	var input map[string]any
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	switch key {
	case "calculator":
		x, ok := input["expression"].(string)
		if !ok {
			return nil, errors.New("expression required")
		}
		result, e := calculate(x)
		if e != nil {
			return nil, e
		}
		return json.Marshal(map[string]string{"result": result})
	case "currentTime":
		z := time.UTC
		if x, ok := input["timezone"].(string); ok && x != "" {
			var e error
			z, e = time.LoadLocation(x)
			if e != nil {
				return nil, errors.New("timezone invalid")
			}
		}
		now := time.Now().In(z)
		zone, _ := now.Zone()
		return json.Marshal(map[string]string{"time": now.Format(time.RFC3339Nano), "timezone": z.String(), "offset": zone})
	default:
		return nil, errors.New("builtin not allowed")
	}
}
func (e *Executor) executeHTTP(ctx context.Context, v Version, args json.RawMessage) (json.RawMessage, error) {
	d, err := parseHTTPDefinition(v.HTTPDefinition)
	if err != nil {
		return nil, err
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	payload := map[string]json.RawMessage{}
	for target, source := range d.RequestMapping {
		val, ok := input[source]
		if !ok {
			return nil, fmt.Errorf("tool argument %s missing", source)
		}
		payload[target] = val
	}
	body, _ := json.Marshal(payload)
	method := d.Method
	if method == "" {
		method = "POST"
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), d.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, val := range d.Headers {
		req.Header.Set(k, val)
	}
	if v.SecretCiphertext != nil && e.secret == nil {
		return nil, errors.New("tool secret reader unavailable")
	}
	if e.secret != nil {
		h, er := e.secret.Headers(ctx, v)
		if er != nil {
			return nil, er
		}
		for k, val := range h {
			if !allowedHeader(k) || strings.ContainsAny(val, "\r\n") {
				return nil, errors.New("secret header is restricted")
			}
			req.Header.Set(k, val)
		}
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 65537)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(raw) > 65536 {
		return nil, errors.New("HTTP response exceeds limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP tool status %d", resp.StatusCode)
	}
	if len(d.ResponseMapping) == 0 {
		return raw, nil
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	mapped := map[string]json.RawMessage{}
	for target, source := range d.ResponseMapping {
		val, ok := result[source]
		if !ok {
			return nil, fmt.Errorf("HTTP response field %s missing", source)
		}
		mapped[target] = val
	}
	return json.Marshal(mapped)
}
