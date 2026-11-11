package providermodel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"admin/server/internal/module/ai/pricing"
	aiprovider "admin/server/internal/module/ai/provider"
	"admin/server/internal/secretkey"
)

func TestProviderHelloSupportsChatProtocols(t *testing.T) {
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, _, err := secretkey.EncryptProviderValue(keys.ProviderEncryptionKey(), "sk-test")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		protocol   aiprovider.Protocol
		basePath   string
		authHeader string
		authValue  string
		bodyPart   string
		reply      string
	}{
		{
			name: "openai chat", protocol: aiprovider.ProtocolOpenAI,
			basePath: "/v1/chat/completions", authHeader: "Authorization",
			authValue: "Bearer sk-test", bodyPart: `"messages"`,
			reply: `{"choices":[{"message":{"content":"OK"}}]}`,
		},
		{
			name: "openai responses", protocol: aiprovider.ProtocolOpenAIResponses,
			basePath: "/v1/responses", authHeader: "Authorization",
			authValue: "Bearer sk-test", bodyPart: `"input"`,
			reply: `{"output":[{"content":[{"type":"output_text","text":"OK"}]}]}`,
		},
		{
			name: "anthropic", protocol: aiprovider.ProtocolAnthropic,
			basePath: "/v1/messages", authHeader: "x-api-key",
			authValue: "sk-test", bodyPart: `"max_tokens"`,
			reply: `{"content":[{"type":"text","text":"OK"}]}`,
		},
		{
			name: "gemini", protocol: aiprovider.ProtocolGemini,
			basePath: "/v1beta/models/test-chat:generateContent", authHeader: "x-goog-api-key",
			authValue: "sk-test", bodyPart: `"contents"`,
			reply: `{"candidates":[{"content":{"parts":[{"text":"OK"}]}}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != tc.basePath {
					t.Errorf("path = %s, want %s", request.URL.Path, tc.basePath)
					writer.WriteHeader(http.StatusNotFound)
					return
				}
				if got := request.Header.Get(tc.authHeader); got != tc.authValue {
					t.Errorf("%s = %q, want %q", tc.authHeader, got, tc.authValue)
				}
				body, _ := io.ReadAll(request.Body)
				if !strings.Contains(string(body), tc.bodyPart) ||
					(tc.protocol != aiprovider.ProtocolGemini &&
						!strings.Contains(string(body), `"test-chat"`)) {
					t.Errorf("body = %s", body)
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(tc.reply))
			}))
			defer server.Close()

			service := &Service{
				repo: &syncFakeRepository{chatModel: &Model{
					ID: 9, ProviderID: 3, UpstreamModel: "test-chat", DisplayName: "Test Chat",
					ModelKind: pricing.KindChat,
				}},
				providerSource: syncFakeProvider{model: aiprovider.Model{
					BaseURL: server.URL + map[aiprovider.Protocol]string{
						aiprovider.ProtocolOpenAI:          "/v1",
						aiprovider.ProtocolOpenAIResponses: "/v1",
						aiprovider.ProtocolAnthropic:       "/v1",
						aiprovider.ProtocolGemini:          "/v1beta",
					}[tc.protocol],
					Protocol: tc.protocol, APIKeyCiphertext: ciphertext,
					TimeoutMS: 3000, Vendor: aiprovider.VendorOpenAI,
				}},
				keys: keys, httpClient: server.Client(), now: time.Now,
			}
			result, err := service.Test(context.Background(), 3)
			if err != nil {
				t.Fatal(err)
			}
			if !result.OK || result.Reply != "OK" || result.HTTPStatus != http.StatusOK ||
				result.Model != "test-chat" {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestProviderHelloReportsUpstreamErrorWithoutLeakingKey(t *testing.T) {
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, _, err := secretkey.EncryptProviderValue(keys.ProviderEncryptionKey(), "sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":{"message":"invalid API key sk-secret"}}`))
	}))
	defer server.Close()
	service := &Service{
		repo: &syncFakeRepository{chatModel: &Model{
			ID: 9, ProviderID: 3, UpstreamModel: "test-chat", ModelKind: pricing.KindChat,
		}},
		providerSource: syncFakeProvider{model: aiprovider.Model{
			BaseURL: server.URL + "/v1", Protocol: aiprovider.ProtocolOpenAI,
			APIKeyCiphertext: ciphertext, TimeoutMS: 3000,
		}},
		keys: keys, httpClient: server.Client(), now: time.Now,
	}
	result, err := service.Test(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || result.HTTPStatus != http.StatusUnauthorized ||
		!strings.Contains(result.Error, "HTTP 401") ||
		strings.Contains(result.Error, "sk-secret") ||
		!strings.Contains(result.Error, "[redacted]") {
		t.Fatalf("result = %+v", result)
	}
}

func TestProviderHelloRequiresEnabledChatModel(t *testing.T) {
	keys, err := secretkey.New(strings.Repeat("s", 64))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{
		repo:           &syncFakeRepository{},
		providerSource: syncFakeProvider{model: aiprovider.Model{}},
		keys:           keys, now: time.Now,
	}
	result, err := service.Test(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !strings.Contains(result.Error, "对话模型") {
		t.Fatalf("result = %+v", result)
	}
}
