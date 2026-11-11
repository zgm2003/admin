package providermodel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/ai/pricing"
	aiprovider "admin/server/internal/module/ai/provider"
	"admin/server/internal/secretkey"
)

func TestTestDraftDiscoversChatModelBeforeSaving(t *testing.T) {
	keys := testKeyRing(t)
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
		repo:           &syncFakeRepository{},
		providerSource: syncFakeProvider{},
		keys:           keys,
		httpClient:     server.Client(),
		now:            time.Now,
	}
	result, err := service.TestDraft(context.Background(), TestDraftInput{
		Protocol: aiprovider.ProtocolOpenAI, BaseURL: server.URL + "/v1",
		APIKey: "sk-draft", TimeoutMS: 3000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Model != "gpt-4o-mini" || result.ModelsFound != 4 || result.Reply != "OK" {
		t.Fatalf("result = %+v", result)
	}
}

func TestTestDraftUsesStoredKeyAndModelWhenEditing(t *testing.T) {
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
			writer.WriteHeader(http.StatusNotFound)
		case "/v1/chat/completions":
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	providerID := int64(9)
	service := &Service{
		repo: &syncFakeRepository{chatModel: &Model{
			ID: 1, ProviderID: providerID, UpstreamModel: "saved-chat",
			ModelKind: pricing.KindChat,
		}},
		providerSource: syncFakeProvider{model: aiprovider.Model{
			APIKeyCiphertext: ciphertext,
		}},
		keys: keys, httpClient: server.Client(), now: time.Now,
	}
	result, err := service.TestDraft(context.Background(), TestDraftInput{
		ProviderID: &providerID, Protocol: aiprovider.ProtocolOpenAI,
		BaseURL: server.URL + "/v1", TimeoutMS: 3000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Model != "saved-chat" || result.Reply != "OK" {
		t.Fatalf("result = %+v", result)
	}
}

func TestChooseTestModelSkipsEmbeddingAndImage(t *testing.T) {
	model := chooseTestModel([]upstreamModel{
		{ID: "text-embedding-3-small"},
		{ID: "dall-e-3"},
		{ID: "claude-3-5-sonnet"},
		{ID: "gpt-4o"},
	})
	if model.ID != "claude-3-5-sonnet" {
		t.Fatalf("model = %+v", model)
	}
	if !strings.Contains(model.ID, "claude") {
		t.Fatal("expected a chat model")
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
