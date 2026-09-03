package mail

import (
	"context"
	"fmt"
	"strings"
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
	if len(summary) > 512 {
		summary = summary[:512]
	}
	return &ProviderError{Code: strings.TrimSpace(code), Summary: strings.TrimSpace(summary)}
}
func providerError(err error) *ProviderError {
	if err == nil {
		return nil
	}
	if value, ok := err.(*ProviderError); ok {
		return value
	}
	return NewProviderError("provider_error", fmt.Sprintf("%v", err))
}
