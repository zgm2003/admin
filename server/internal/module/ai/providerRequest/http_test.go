package providerrequest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientsRejectInvalidFrozenRequestsBeforePOST(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	for _, mutate := range []func(*TextRequest){
		func(v *TextRequest) { v.Config.APIKey = "" }, func(v *TextRequest) { v.Config.Timeout = 0 }, func(v *TextRequest) { v.Config.Model = "" }, func(v *TextRequest) { v.Config.BaseURL += "?token=TOKEN" }, func(v *TextRequest) { v.MaxOutputTokens = 0 }, func(v *TextRequest) { v.Input = nil }, func(v *TextRequest) { v.Params = []byte(`{"model":"different"}`) }, func(v *TextRequest) { v.Params = []byte(`{"stream":true}`) }, func(v *TextRequest) { v.Config.Headers = map[string]string{"Authorization": "Bearer different"} },
		func(v *TextRequest) { v.Config.APIKey = "   " }, func(v *TextRequest) { v.Config.Headers = map[string]string{"X-Provider-Header": "invalid\x00value"} },
	} {
		request := textRequest(server.URL)
		mutate(&request)
		if _, err := NewResponsesClient(server.Client()).Generate(context.Background(), request); err == nil {
			t.Fatal("accepted invalid request")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewResponsesClient(server.Client()).Generate(ctx, textRequest(server.URL))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid/canceled requests sent %d POSTs", calls.Load())
	}
}

func TestResponsesClientDoesNotFollowPOSTRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	_, err := NewResponsesClient(server.Client()).Generate(context.Background(), textRequest(server.URL))
	if err == nil || redirected.Load() != 0 {
		t.Fatalf("err=%v redirected POST=%d", err, redirected.Load())
	}
}

func TestResponsesClientUnknownSubmissionIsNeverRetried(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(200)
			return
		}
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = connection.Close()
	}))
	defer server.Close()
	httpClient := server.Client()
	warm, err := httpClient.Get(server.URL + "/warm")
	if err != nil {
		t.Fatal(err)
	}
	_ = warm.Body.Close()
	request := textRequest(server.URL)
	request.Config.Headers = map[string]string{"Idempotency-Key": "stable-KEY"}
	_, err = NewResponsesClient(httpClient).Generate(context.Background(), request)
	var requestError *RequestError
	if !errors.As(err, &requestError) || !requestError.SubmissionUnknown || calls.Load() != 1 {
		t.Fatalf("err=%v requestError=%+v requests=%d", err, requestError, calls.Load())
	}
}

func TestResponsesClientTimeoutAfterPOSTIsUnknown(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		<-release
	}))
	defer server.Close()
	defer close(release)
	request := textRequest(server.URL)
	request.Config.Timeout = 30 * time.Millisecond
	_, err := NewResponsesClient(server.Client()).Generate(context.Background(), request)
	var requestError *RequestError
	if !errors.As(err, &requestError) || !requestError.SubmissionUnknown || !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("err=%v requestError=%+v requests=%d", err, requestError, calls.Load())
	}
}

func TestResponsesClientDoesNotExposeUpstreamErrorBodies(t *testing.T) {
	for _, status := range []int{401, 429, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			io.WriteString(w, `{"error":{"message":"TOKEN PRIVATE_PAYLOAD"}}`)
		}))
		response, err := NewResponsesClient(server.Client()).Generate(context.Background(), textRequest(server.URL))
		server.Close()
		var requestError *RequestError
		if !errors.As(err, &requestError) || requestError.HTTPStatus != status || strings.Contains(err.Error(), "TOKEN") || strings.Contains(err.Error(), "PRIVATE_PAYLOAD") || len(response.Raw) != 0 {
			t.Fatalf("response=%+v err=%v", response, err)
		}
		if status == 500 && !requestError.SubmissionUnknown {
			t.Fatal("5xx invented confirmed zero consumption")
		}
	}
}
