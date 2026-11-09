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
	for _, model := range catalog.Models {
		if model.VendorCode != "openai" {
			t.Fatalf("unexpected vendor %q", model.VendorCode)
		}
		if model.ModelKey == "gpt-5.5" {
			found = true
			if model.InputUSDMicros != 5_000_000 || model.OutputUSDMicros != 30_000_000 {
				t.Fatalf("gpt-5.5 prices = %d/%d", model.InputUSDMicros, model.OutputUSDMicros)
			}
		}
	}
	if !found {
		t.Fatal("gpt-5.5 not found in catalog")
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
}
