package provider

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenAIPricingFixtureParsesOnlyStandardTokenRows(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("model", "testdata", "openai-pricing.md"))
	if err != nil {
		t.Fatal(err)
	}
	rates, err := ParsePricingMarkdown(body)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rates {
		if r.ModelKey == "gpt-5" && r.UsageKey == "input" && r.SourceUnitPrice == "1.25" {
			found = true
		}
	}
	if !found {
		t.Fatal("gpt-5 input standard price missing")
	}
}

func TestOpenAIPricingFixtureOnlyPublishesSupportedImageAndSpeechBilling(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("model", "testdata", "openai-pricing.md"))
	if err != nil {
		t.Fatal(err)
	}
	rates, err := ParsePricingMarkdown(body)
	if err != nil {
		t.Fatal(err)
	}
	var image, speech bool
	for _, rate := range rates {
		if (strings.HasPrefix(rate.ModelKey, "gpt-image-") || rate.ModelKey == "chatgpt-image-latest") && (rate.Operation == "text.generate" || rate.Unit == "token") {
			t.Fatalf("image reference tokens became fake capability or additive billing: %+v", rate)
		}
		switch {
		case rate.ModelKey == "gpt-image-1.5" && rate.Operation == "image.generate" && rate.Unit == "image":
			image = true
		case rate.ModelKey == "gpt-4o-mini-tts" && rate.Operation == "audio.speech" && rate.UsageKey == "output" && rate.Unit == "token" && rate.UnitQuantity == "1000000" && rate.SourceUnitPrice == "12.00":
			speech = true
		}
	}
	if !image || !speech {
		t.Fatalf("image=%v speech=%v", image, speech)
	}
}

func TestOpenAIPricingFixtureDoesNotAdvertiseVideo(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("model", "testdata", "openai-pricing.md"))
	if err != nil {
		t.Fatal(err)
	}
	rates, err := ParsePricingMarkdown(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, rate := range rates {
		if rate.Operation == "video.generate" {
			t.Fatal("video rate advertised despite official Videos API shutdown")
		}
	}
}
func TestOpenAISourceUsesFixedURLAndRejectsNon200(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != OpenAIPricingURL {
			t.Fatalf("unexpected URL %s", req.URL)
		}
		return &http.Response{StatusCode: 503, Body: http.NoBody, Header: make(http.Header)}, nil
	})}
	if _, err := (OpenAISource{Client: client}).FetchPricing(context.Background()); err == nil {
		t.Fatal("non-200 accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

var _ = time.Second
