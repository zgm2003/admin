package officialmodel

import (
	"strings"
	"testing"
)

func TestEmbeddedCatalogIsValid(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog() error = %v", err)
	}
	if catalog.Version == "" || catalog.SourceURL == "" || len(catalog.Models) == 0 {
		t.Fatalf("catalog = %+v", catalog)
	}
	found := false
	vendors := make(map[Vendor]bool)
	for _, model := range catalog.Models {
		if !model.Vendor.Valid() {
			t.Fatalf("unexpected vendor %d", model.Vendor)
		}
		vendors[model.Vendor] = true
		if model.SourceURL == "" || model.VerifiedAt.IsZero() {
			t.Fatalf("model %s has incomplete source facts", model.ModelKey)
		}
		if model.ModelKey == "gpt-5.5" {
			found = true
			if model.InputUSDMicros != 5_000_000 || model.OutputUSDMicros != 30_000_000 {
				t.Fatalf("gpt-5.5 prices = %d/%d", model.InputUSDMicros, model.OutputUSDMicros)
			}
		}
		if model.ModelKey == "gpt-6-astra" {
			if model.CacheReadUSDMicros == nil || *model.CacheReadUSDMicros != 1_000_000 ||
				model.CacheWriteUSDMicros == nil || *model.CacheWriteUSDMicros != 12_500_000 {
				t.Fatalf("gpt-6-astra cache prices = %+v/%+v",
					model.CacheReadUSDMicros, model.CacheWriteUSDMicros)
			}
		}
		if model.ModelKey == "doubao-seedance-2-5-260628" {
			if model.Vendor != VendorVolcengine || model.ModelKind != 3 || len(model.VideoPrices) != 6 {
				t.Fatalf("seedance 2.5 = vendor %d kind %d prices %+v",
					model.Vendor, model.ModelKind, model.VideoPrices)
			}
			if model.VideoPrices[0].TierKey != "480p:no_input" ||
				model.VideoPrices[0].PriceCNYMicros == nil || *model.VideoPrices[0].PriceCNYMicros != 670_000 {
				t.Fatalf("seedance 2.5 480p price = %+v", model.VideoPrices[0])
			}
			if model.VideoPrices[1].TierKey != "480p:with_input" || model.VideoPrices[1].PriceCNYMicros != nil {
				t.Fatalf("seedance 2.5 with-input price = %+v", model.VideoPrices[1])
			}
		}
	}
	if !found {
		t.Fatal("gpt-5.5 not found in catalog")
	}
	for _, vendor := range []Vendor{VendorOpenAI, VendorAnthropic, VendorGoogle, VendorVolcengine} {
		if !vendors[vendor] {
			t.Fatalf("vendor %d not found in catalog", vendor)
		}
	}
}

func TestParseCatalogRejectsDuplicateAndInvalidPrices(t *testing.T) {
	duplicate := `{
		"catalogVersion":"v1",
		"sourceUrl":"https://example.com/pricing",
		"verifiedAt":"2026-11-09T00:00:00Z",
		"models":[
			{"vendorCode":"openai","vendorName":"OpenAI","modelKey":"gpt-test","displayName":"GPT Test","inputUsdPerMillion":"1","outputUsdPerMillion":"2"},
			{"vendorCode":"openai","vendorName":"OpenAI","modelKey":"gpt-test","displayName":"GPT Test","inputUsdPerMillion":"1","outputUsdPerMillion":"2"}
		]
	}`
	if _, err := parseCatalog([]byte(duplicate)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate catalog error = %v", err)
	}

	invalidPrice := strings.Replace(duplicate, `"inputUsdPerMillion":"1"`, `"inputUsdPerMillion":"-1"`, 1)
	if _, err := parseCatalog([]byte(invalidPrice)); err == nil {
		t.Fatal("negative price accepted")
	}

	unknownVendor := strings.Replace(duplicate, `"vendorCode":"openai"`, `"vendorCode":"unknown"`, 1)
	if _, err := parseCatalog([]byte(unknownVendor)); err == nil || !strings.Contains(err.Error(), "vendorCode") {
		t.Fatalf("unknown vendor error = %v", err)
	}

	invalidCache := strings.Replace(duplicate, `"outputUsdPerMillion":"2"`,
		`"outputUsdPerMillion":"2","cacheReadUsdPerMillion":"-1"`, 1)
	if _, err := parseCatalog([]byte(invalidCache)); err == nil || !strings.Contains(err.Error(), "cacheRead") {
		t.Fatalf("negative cache price error = %v", err)
	}

	invalidVideo := strings.Replace(duplicate, `"outputUsdPerMillion":"2"`,
		`"outputUsdPerMillion":"2","modelKind":3,"videoPrices":[{"tierKey":"480p","priceCnyPerSecond":"-1"}]`, 1)
	if _, err := parseCatalog([]byte(invalidVideo)); err == nil || !strings.Contains(err.Error(), "videoPrices") {
		t.Fatalf("negative video price error = %v", err)
	}
}
