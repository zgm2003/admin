package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOpenAIModelsReturnsIdentityWithoutInventingCapabilitiesOrPrices(t *testing.T) {
	c := OpenAIModelsClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.openai.com/v1/models" || req.Header.Get("Authorization") != "Bearer TOKEN" {
			t.Fatal("models request contract")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"object":"list","data":[{"id":"fixture-model","object":"model","created":1,"owned_by":"openai"}]}`))}, nil
	})}}
	items, err := c.List(context.Background(), "https://api.openai.com/v1", "TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(items)
	if len(items) != 1 || items[0].ID != "fixture-model" || strings.Contains(string(raw), "price") || strings.Contains(string(raw), "operation") {
		t.Fatalf("catalog invented behavior: %s", raw)
	}
}
func TestOpenAIModelsRejectsMalformedAndOversizedResponses(t *testing.T) {
	for _, body := range []string{`null`, `{"data":null}`, `{"data":[{"id":"x"},{"id":"x"}]}`, strings.Repeat("x", (1<<20)+1)} {
		client := OpenAIModelsClient{Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})}}
		if _, err := client.List(context.Background(), "https://api.openai.com/v1", "TOKEN"); err == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
}
