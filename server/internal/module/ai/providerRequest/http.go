package providerrequest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxProtocolBytes = 16 << 20

func validateRequest(in TextRequest) (map[string]json.RawMessage, error) {
	if strings.TrimSpace(in.Config.APIKey) == "" || !validHeaderValue(in.Config.APIKey) || strings.TrimSpace(in.Config.Model) == "" || in.Config.Timeout <= 0 || in.MaxOutputTokens < 1 || in.MaxOutputTokens > 1<<53-1 || len(in.Input) == 0 {
		return nil, ErrRequestInvalid
	}
	for _, raw := range in.Input {
		if !jsonObject(raw) {
			return nil, ErrRequestInvalid
		}
	}
	seen := map[string]bool{}
	for _, tool := range in.Tools {
		if !validFunctionName(tool.Name) || seen[tool.Name] || !jsonObject(tool.Parameters) {
			return nil, ErrRequestInvalid
		}
		seen[tool.Name] = true
	}
	params := map[string]json.RawMessage{}
	if len(in.Params) > 0 {
		if !jsonObject(in.Params) || json.Unmarshal(in.Params, &params) != nil {
			return nil, ErrRequestInvalid
		}
	}
	for _, key := range []string{"model", "input", "messages", "instructions", "tools", "stream", "stream_options", "store", "background", "previous_response_id", "conversation", "prompt", "max_output_tokens", "max_completion_tokens", "max_tokens", "n"} {
		if _, ok := params[key]; ok {
			return nil, ErrRequestInvalid
		}
	}
	return params, nil
}

func jsonObject(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 1 && raw[0] == '{' && json.Valid(raw)
}
func validFunctionName(name string) bool {
	if len(name) < 1 || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
func putJSON(body map[string]json.RawMessage, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	body[key] = raw
	return nil
}

func endpoint(base, path string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", ErrRequestInvalid
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + path
	u.RawPath = ""
	return u.String(), nil
}

func postJSON(ctx context.Context, client *http.Client, in TextRequest, path string, body map[string]json.RawMessage) ([]byte, string, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", 0, requestFailure("canceled", 0, "", false, err)
	}
	target, err := endpoint(in.Config.BaseURL, path)
	if err != nil {
		return nil, "", 0, requestFailure("invalid", 0, "", false, err)
	}
	raw, err := json.Marshal(body)
	if err != nil || len(raw) > maxProtocolBytes {
		return nil, "", 0, requestFailure("invalid", 0, "", false, ErrRequestInvalid)
	}
	workCtx, cancel := context.WithTimeout(ctx, in.Config.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(workCtx, http.MethodPost, target, bytes.NewReader(raw))
	if err != nil {
		return nil, "", 0, requestFailure("invalid", 0, "", false, ErrRequestInvalid)
	}
	// Transport retries can otherwise treat Idempotency-Key POSTs with GetBody
	// as replayable. A model POST is always one attempt, even on a stale socket.
	req.GetBody = nil
	for key, value := range in.Config.Headers {
		if !validHeaderName(key) || !validHeaderValue(value) || reservedHeader(key) {
			return nil, "", 0, requestFailure("invalid", 0, "", false, ErrRequestInvalid)
		}
		req.Header.Set(key, value)
	}
	req.Header.Set("Authorization", "Bearer "+in.Config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	owned := http.Client{}
	if client != nil {
		owned = *client
	}
	owned.Jar = nil
	owned.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := owned.Do(req)
	if err != nil {
		kind := "transport"
		if errors.Is(err, context.Canceled) {
			kind = "canceled"
		}
		if errors.Is(err, context.DeadlineExceeded) {
			kind = "timeout"
		}
		return nil, "", 0, requestFailure(kind, 0, "", true, err)
	}
	defer response.Body.Close()
	id := safeRequestID(response.Header.Get("x-request-id"), in.Config.APIKey)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		unknown := true
		switch response.StatusCode {
		case 400, 401, 403, 404, 405, 413, 415, 422, 429:
			unknown = false
		}
		return nil, id, response.StatusCode, requestFailure("http", response.StatusCode, id, unknown, ErrUpstreamRejected)
	}
	if strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		return nil, id, response.StatusCode, requestFailure("protocol", response.StatusCode, id, true, ErrResponseInvalid)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxProtocolBytes+1))
	if err != nil {
		return nil, id, response.StatusCode, requestFailure("read", response.StatusCode, id, true, err)
	}
	if len(payload) > maxProtocolBytes || !json.Valid(payload) {
		return nil, id, response.StatusCode, requestFailure("protocol", response.StatusCode, id, true, ErrResponseInvalid)
	}
	return payload, id, response.StatusCode, nil
}

func reservedHeader(name string) bool {
	switch strings.ToLower(name) {
	case "authorization", "proxy-authorization", "host", "cookie", "set-cookie", "content-type", "content-length", "connection", "transfer-encoding":
		return true
	}
	return false
}
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c) {
			continue
		}
		return false
	}
	return true
}

func validHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 32 && value[i] != '\t' || value[i] == 127 {
			return false
		}
	}
	return true
}
func safeRequestID(value, key string) string {
	if len(value) > 128 || value == key {
		return ""
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-') {
			return ""
		}
	}
	return value
}
