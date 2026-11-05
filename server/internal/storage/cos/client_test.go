package cos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPresignPutBuildsBoundRequestWithoutNetwork(t *testing.T) {
	var seen *http.Request
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
	})})
	result, err := client.PresignPut(context.Background(), Credentials{AppID: "1250000000", SecretID: "sid", SecretKey: "skey", Bucket: "assets", Region: "ap-guangzhou", Endpoint: "https://cos.example.com"}, PutRequest{ObjectKey: "avatars/2026/photo.png", ContentType: "image/png", ContentLength: 123, PublicRead: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.URL == "" || !strings.Contains(result.URL, "q-signature") || result.Headers["Content-Type"] != "image/png" || result.Headers["Content-Length"] != "123" || result.Headers["x-cos-acl"] != "public-read" {
		t.Fatalf("result=%+v", result)
	}
	parsed, err := url.Parse(result.URL)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(parsed.Query().Get("q-sign-time"), ";")
	if len(parts) != 2 {
		t.Fatalf("q-sign-time=%q", parsed.Query().Get("q-sign-time"))
	}
	start, _ := strconv.ParseInt(parts[0], 10, 64)
	end, _ := strconv.ParseInt(parts[1], 10, 64)
	if end-start != int64(PresignValidity/time.Second) {
		t.Fatalf("signature validity=%d", end-start)
	}
	if seen != nil {
		t.Fatal("presign unexpectedly performed network request")
	}
}

func TestPresignGetBuildsSignedReadURLWithoutNetwork(t *testing.T) {
	var seen *http.Request
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})})
	before := time.Now().UTC()
	result, err := client.PresignGet(context.Background(), Credentials{
		AppID: "1250000000", SecretID: "sid", SecretKey: "skey", Bucket: "assets", Region: "ap-guangzhou", Endpoint: "https://cos.example.com",
	}, GetRequest{ObjectKey: "avatar/.admin-storage/v2/p1/r2/c3/v4/2026/09/17/6e7e53334a6da066b130a89cc3f69535.png"})
	if err != nil {
		t.Fatal(err)
	}
	if seen != nil {
		t.Fatal("presign unexpectedly performed network request")
	}
	if result.URL == "" || !strings.Contains(result.URL, "q-signature") {
		t.Fatalf("result = %+v", result)
	}
	if result.ExpiresAt.Before(before.Add(GetPresignValidity-time.Second)) || result.ExpiresAt.After(time.Now().UTC().Add(GetPresignValidity+time.Second)) {
		t.Fatalf("expiresAt = %v", result.ExpiresAt)
	}
	parsed, err := url.Parse(result.URL)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(parsed.Query().Get("q-sign-time"), ";")
	if len(parts) != 2 {
		t.Fatalf("q-sign-time = %q", parsed.Query().Get("q-sign-time"))
	}
	start, _ := strconv.ParseInt(parts[0], 10, 64)
	end, _ := strconv.ParseInt(parts[1], 10, 64)
	if end-start != int64(GetPresignValidity/time.Second) {
		t.Fatalf("signature validity = %d", end-start)
	}
	if strings.Contains(strings.ToLower(result.URL), "x-cos-acl") {
		t.Fatalf("GET URL unexpectedly contains ACL header: %s", result.URL)
	}
}

func TestPresignGetRejectsUnsafeOrIncompleteRequest(t *testing.T) {
	client := NewClient(nil)
	credentials := Credentials{AppID: "1250000000", SecretID: "sid", SecretKey: "skey", Bucket: "assets", Region: "ap-guangzhou"}
	for _, key := range []string{"", "/root", "../escape", "a\\b", "a\tb"} {
		if _, err := client.PresignGet(context.Background(), credentials, GetRequest{ObjectKey: key}); err == nil {
			t.Fatalf("unsafe key %q accepted", key)
		}
	}
	if _, err := client.PresignGet(context.Background(), Credentials{}, GetRequest{ObjectKey: "safe/key.png"}); err == nil {
		t.Fatal("incomplete credentials were accepted")
	}
}
func TestTestConnectionUsesBucketGet(t *testing.T) {
	called := false
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
	})})
	if err := client.TestConnection(context.Background(), Credentials{AppID: "1250000000", SecretID: "sid", SecretKey: "skey", Bucket: "assets", Region: "ap-guangzhou", Endpoint: "https://cos.example.com"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("bucket request was not sent")
	}
}

func TestBucketNameDoesNotDuplicateAppID(t *testing.T) {
	var host string
	client := NewClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		host = r.URL.Host
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
	})})
	if err := client.TestConnection(context.Background(), Credentials{AppID: "1314542588", SecretID: "sid", SecretKey: "skey", Bucket: "zgm-1314542588", Region: "ap-nanjing"}); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(host, "zgm-1314542588-1314542588.") {
		t.Fatalf("app id was duplicated in bucket host %q", host)
	}
}

func TestPresignPutRejectsUnsafeRequest(t *testing.T) {
	client := NewClient(nil)
	for _, key := range []string{"", "/root", "../escape", "a\\b"} {
		if _, err := client.PresignPut(context.Background(), Credentials{}, PutRequest{ObjectKey: key, ContentLength: 1}); err == nil {
			t.Fatalf("unsafe key %q accepted", key)
		}
	}
	_ = httptest.NewRecorder
}
