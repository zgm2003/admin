package providerrequest

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type FrozenConfig struct {
	BaseURL            string            `json:"baseUrl"`
	APIKey             string            `json:"-"`
	Model              string            `json:"model"`
	Headers            map[string]string `json:"-"`
	MediaHostAllowlist []string          `json:"mediaHostAllowlist,omitempty"`
	Timeout            time.Duration     `json:"timeout"`
}

// Input contains protocol-native history, including complete prior output
// items. No client reloads mutable provider configuration or runs tools.
type TextRequest struct {
	OnTextDelta     func(string) error `json:"-"`
	Config          FrozenConfig       `json:"config"`
	Prompt          string             `json:"prompt"`
	Input           []json.RawMessage  `json:"input"`
	Tools           []FunctionTool     `json:"tools"`
	MaxOutputTokens int64              `json:"maxOutputTokens"`
	Params          json.RawMessage    `json:"params"`
}
type FunctionTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}
type FunctionCall struct {
	CallID    string `json:"callId"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// InputTokens includes CachedInputTokens. Consumers must subtract cached input
// before applying an uncached-input rate. Known=false never means zero usage.
type TokenUsage struct {
	Known             bool            `json:"known"`
	InputTokens       int64           `json:"inputTokens"`
	CachedInputTokens int64           `json:"cachedInputTokens"`
	OutputTokens      int64           `json:"outputTokens"`
	Raw               json.RawMessage `json:"raw"`
}
type TextResponse struct {
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	FinishReason string            `json:"finishReason"`
	Text         string            `json:"text"`
	Refusal      string            `json:"refusal"`
	RequestID    string            `json:"requestId"`
	OutputItems  []json.RawMessage `json:"outputItems"`
	Calls        []FunctionCall    `json:"calls"`
	Usage        TokenUsage        `json:"usage"`
	Raw          json.RawMessage   `json:"raw"`
}

var (
	ErrRequestInvalid     = errors.New("provider request is invalid")
	ErrSubmissionUnknown  = errors.New("provider submission outcome is unknown")
	ErrUpstreamRejected   = errors.New("provider rejected the request")
	ErrResponseInvalid    = errors.New("provider response is invalid")
	ErrResponseIncomplete = errors.New("provider response is incomplete or failed")
)

// Error returns only a bounded class/status. Upstream bodies and credentials
// never form a public error string; Cause is available through errors.Is/As.
type RequestError struct {
	Kind              string `json:"kind"`
	HTTPStatus        int    `json:"httpStatus"`
	RequestID         string `json:"requestId"`
	SubmissionUnknown bool   `json:"submissionUnknown"`
	cause             error
}

func (e *RequestError) Error() string {
	if e.HTTPStatus > 0 {
		return fmt.Sprintf("provider request %s (HTTP %d)", e.Kind, e.HTTPStatus)
	}
	return "provider request " + e.Kind
}
func (e *RequestError) Unwrap() error { return e.cause }

func requestFailure(kind string, status int, id string, unknown bool, cause error) *RequestError {
	if unknown {
		cause = errors.Join(ErrSubmissionUnknown, cause)
	}
	return &RequestError{Kind: kind, HTTPStatus: status, RequestID: id, SubmissionUnknown: unknown, cause: cause}
}
