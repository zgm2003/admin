package providermodel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	aiprovider "admin/server/internal/module/ai/provider"
	"admin/server/internal/secretkey"
)

func TestPreviewModelsThenTestSelectedModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer sk-draft" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/v1/models":
			_, _ = writer.Write([]byte(`{"data":[
				{"id":"text-embedding-3-large"},
				{"id":"dall-e-3"},
				{"id":"gpt-4o-mini"},
				{"id":"gpt-4o"}
			]}`))
		case "/v1/chat/completions":
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	service := &Service{
		providerSource: syncFakeProvider{},
		keys:           testKeyRing(t),
		httpClient:     server.Client(),
		now:            time.Now,
	}
	input := TestDraftInput{
		Protocol: aiprovider.ProtocolOpenAI, BaseURL: server.URL + "/v1",
		APIKey: "sk-draft", TimeoutMS: 3000,
	}
	preview, err := service.PreviewModels(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.OK || len(preview.Models) != 4 {
		t.Fatalf("preview = %+v", preview)
	}
	input.Model = "gpt-4o-mini"
	result, err := service.TestDraft(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Model != "gpt-4o-mini" || result.Reply != "OK" {
		t.Fatalf("result = %+v", result)
	}
}

func TestPreviewAndTestUseStoredKeyWhenEditing(t *testing.T) {
	keys := testKeyRing(t)
	ciphertext, _, err := secretkey.EncryptProviderValue(keys.ProviderEncryptionKey(), "sk-saved")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer sk-saved" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/v1/models":
			_, _ = writer.Write([]byte(`{"data":[{"id":"saved-chat"}]}`))
		case "/v1/chat/completions":
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	providerID := int64(9)
	service := &Service{
		providerSource: syncFakeProvider{model: aiprovider.Model{APIKeyCiphertext: ciphertext}},
		keys:           keys,
		httpClient:     server.Client(),
		now:            time.Now,
	}
	input := TestDraftInput{
		ProviderID: &providerID, Protocol: aiprovider.ProtocolOpenAI,
		BaseURL: server.URL + "/v1", TimeoutMS: 3000,
	}
	preview, err := service.PreviewModels(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.OK || len(preview.Models) != 1 || preview.Models[0].ID != "saved-chat" {
		t.Fatalf("preview = %+v", preview)
	}
	input.Model = "saved-chat"
	result, err := service.TestDraft(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Model != "saved-chat" || result.Reply != "OK" {
		t.Fatalf("result = %+v", result)
	}
}

func testKeyRing(t *testing.T) *secretkey.KeyRing {
	t.Helper()
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	return keys
}
