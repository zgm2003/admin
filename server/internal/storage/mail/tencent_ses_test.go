package mail

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	messagemail "admin/server/internal/module/message/mail"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testSendInput() messagemail.SendInput {
	return messagemail.SendInput{
		Region: "ap-guangzhou", SecretID: "test-secret-id", SecretKey: "test-secret-key",
		FromEmail: "sender@example.com", ToEmail: "recipient@example.com", Subject: "Verification",
		TemplateID: 1, TemplateData: map[string]string{"code": "654321", "ttl_minutes": "5"},
	}
}

func clientWithResponse(body string) *Client {
	return NewTencentSESClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)), Request: request,
		}, nil
	})})
}

func TestSendPreservesTencentErrorCodeAndRedactsDiagnostics(t *testing.T) {
	client := clientWithResponse(`{"Response":{"Error":{"Code":"FailedOperation.EmailAddrInBlacklist","Message":"recipient blocked; test-secret-id test-secret-key 654321"},"RequestId":"request-fixture"}}`)
	result, err := client.Send(context.Background(), testSendInput())
	var failure *messagemail.ProviderError
	if !errors.As(err, &failure) || failure.Code != "FailedOperation.EmailAddrInBlacklist" {
		t.Fatalf("error=%v, want structured Tencent error code", err)
	}
	if result != (messagemail.ProviderSendResult{}) || !strings.Contains(failure.Summary, "recipient blocked") || !strings.Contains(failure.Summary, "request-fixture") {
		t.Fatalf("result=%+v failure=%+v", result, failure)
	}
	for _, secret := range []string{"test-secret-id", "test-secret-key", "654321"} {
		if strings.Contains(failure.Error(), secret) {
			t.Fatalf("provider error contains sensitive input %q", secret)
		}
	}
}

func TestSendRejectsMalformedOrIncompleteProviderResponses(t *testing.T) {
	for _, body := range []string{
		`not-json`, `{}`, `{"Response":null}`, `{"Response":{}}`,
		`{"Response":{"RequestId":"request-fixture"}}`,
		`{"Response":{"MessageId":"message-fixture"}}`,
		`{"Response":{"RequestId":" ","MessageId":"message-fixture"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			result, err := clientWithResponse(body).Send(context.Background(), testSendInput())
			var failure *messagemail.ProviderError
			if !errors.As(err, &failure) || result != (messagemail.ProviderSendResult{}) {
				t.Fatalf("body=%s result=%+v error=%v, want provider failure", body, result, err)
			}
		})
	}
}

func TestSendTransportFailureRedactsSensitiveInputs(t *testing.T) {
	client := NewTencentSESClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused; test-secret-id test-secret-key 654321")
	})})
	_, err := client.Send(context.Background(), testSendInput())
	var failure *messagemail.ProviderError
	if !errors.As(err, &failure) || !strings.Contains(failure.Summary, "connection refused") {
		t.Fatalf("error=%v, want bounded transport diagnostic", err)
	}
	for _, secret := range []string{"test-secret-id", "test-secret-key", "654321"} {
		if strings.Contains(failure.Error(), secret) {
			t.Fatalf("transport error contains sensitive input %q", secret)
		}
	}
}

func TestSendPropagatesRequestContextAndReportsTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "request-context"))
	defer cancel()
	client := NewTencentSESClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Context().Value(contextKey{}) != "request-context" {
			t.Fatal("request context was lost")
		}
		cancel()
		return nil, context.DeadlineExceeded
	})})
	_, err := client.Send(ctx, testSendInput())
	var failure *messagemail.ProviderError
	if !errors.As(err, &failure) || failure.Code != "timeout" {
		t.Fatalf("error=%v, want timeout", err)
	}
}

type contextKey struct{}

type transportTimeout struct{}

func (transportTimeout) Error() string   { return "TLS handshake timed out" }
func (transportTimeout) Timeout() bool   { return true }
func (transportTimeout) Temporary() bool { return true }

func TestSendRecognizesTransportTimeoutBeforeContextExpires(t *testing.T) {
	ctx := context.Background()
	client := NewTencentSESClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, transportTimeout{}
	})})
	_, err := client.Send(ctx, testSendInput())
	var failure *messagemail.ProviderError
	if ctx.Err() != nil || !errors.As(err, &failure) || failure.Code != "timeout" {
		t.Fatalf("context=%v error=%v, want transport timeout", ctx.Err(), err)
	}
}

func TestSendReturnsProviderReceipt(t *testing.T) {
	client := clientWithResponse(`{"Response":{"RequestId":"request-fixture","MessageId":"message-fixture"}}`)
	result, err := client.Send(context.Background(), testSendInput())
	if err != nil || result.RequestID != "request-fixture" || result.MessageID != "message-fixture" {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}
