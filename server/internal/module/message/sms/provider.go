package sms

import (
	"context"
	"strings"
)

// SendInput carries one single-recipient SMS request to the provider. SMS never
// shares this contract, the provider or its credentials with Mail.
type SendInput struct {
	Region, Endpoint, SecretID, SecretKey string
	SDKAppID, SignName, TemplateID        string
	ToPhone                               string
	TemplateParams                        []string
}

type ProviderResult struct {
	RequestID string
	SerialNo  string
	Fee       uint64
}

type ProviderError struct{ Code, Summary string }

func (e *ProviderError) Error() string { return e.Code + ": " + e.Summary }

// NewProviderError keeps provider diagnostics bounded and free of credentials.
func NewProviderError(code, summary string) *ProviderError {
	summary = strings.TrimSpace(summary)
	if len(summary) > 512 {
		summary = summary[:512]
	}
	return &ProviderError{Code: strings.TrimSpace(code), Summary: summary}
}

type Sender interface {
	Send(context.Context, SendInput) (ProviderResult, error)
}
