package mail

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type SendInput struct {
	Region, Endpoint, SecretID, SecretKey string
	FromEmail, FromName, ReplyTo, ToEmail string
	Subject                               string
	TemplateID                            int
	TemplateData                          map[string]string
}
type ProviderSendResult struct{ RequestID, MessageID string }
type ProviderError struct{ Code, Summary string }

func (e *ProviderError) Error() string { return e.Code + ": " + e.Summary }

type Sender interface {
	Send(context.Context, SendInput) (ProviderSendResult, error)
}

func NewProviderError(code, summary string) *ProviderError {
	code = strings.TrimSpace(code)
	// Error codes are persisted in VARCHAR(128); malformed upstream values must
	// not turn a delivery failure into an audit-write failure.
	if len(code) == 0 || len(code) > 128 || strings.IndexFunc(code, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-')
	}) >= 0 {
		code = "provider_error"
	}
	return &ProviderError{Code: code, Summary: truncateErrorSummary(strings.TrimSpace(summary))}
}
func providerError(err error) *ProviderError {
	if err == nil {
		return nil
	}
	var value *ProviderError
	if errors.As(err, &value) {
		return value
	}
	return NewProviderError("provider_error", fmt.Sprintf("%v", err))
}

func truncateErrorSummary(value string) string {
	value = strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", "�"), "�")
	if len(value) <= 512 {
		return value
	}
	end := 512
	for !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end]
}
