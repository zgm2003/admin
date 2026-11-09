package providermodel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	aiprovider "admin/server/internal/module/ai/provider"
)

func TestListUpstreamModelsByProtocol(t *testing.T) {
	cases := []struct {
		name      string
		protocol  aiprovider.Protocol
		body      string
		assert    func(*testing.T, *http.Request)
		wantFirst string
	}{
		{
			name: "openai chat", protocol: aiprovider.ProtocolOpenAI,
			body: `{"data":[{"id":"gpt-4o"}]}`,
			assert: func(t *testing.T, request *http.Request) {
				if request.Header.Get("Authorization") != "Bearer secret" {
					t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
				}
			},
			wantFirst: "gpt-4o",
		},
		{
			name: "anthropic", protocol: aiprovider.ProtocolAnthropic,
			body: `{"data":[{"id":"claude-sonnet"}]}`,
			assert: func(t *testing.T, request *http.Request) {
				if request.Header.Get("x-api-key") != "secret" ||
					request.Header.Get("anthropic-version") == "" {
					t.Fatalf("anthropic headers = %v", request.Header)
				}
			},
			wantFirst: "claude-sonnet",
		},
		{
			name: "gemini", protocol: aiprovider.ProtocolGemini,
			body: `{"models":[{"name":"models/gemini-2.5-pro"}]}`,
			assert: func(t *testing.T, request *http.Request) {
				if request.Header.Get("x-goog-api-key") != "secret" {
					t.Fatalf("gemini header = %q", request.Header.Get("x-goog-api-key"))
				}
			},
			wantFirst: "gemini-2.5-pro",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/models" {
					t.Fatalf("path = %q", request.URL.Path)
				}
				tc.assert(t, request)
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(tc.body))
			}))
			defer server.Close()
			service := &Service{httpClient: server.Client()}
			models, err := service.listUpstreamModels(context.Background(), aiprovider.Model{
				Protocol: tc.protocol, BaseURL: server.URL, TimeoutMS: 3000,
			}, "secret")
			if err != nil {
				t.Fatal(err)
			}
			if len(models) != 1 || models[0].ID != tc.wantFirst {
				t.Fatalf("models = %+v", models)
			}
		})
	}
}

func TestListUpstreamModelsRejectsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	service := &Service{httpClient: server.Client()}
	if _, err := service.listUpstreamModels(context.Background(), aiprovider.Model{
		Protocol: aiprovider.ProtocolOpenAI, BaseURL: server.URL, TimeoutMS: 3000,
	}, "secret"); err == nil {
		t.Fatal("upstream 401 was accepted")
	}
}
