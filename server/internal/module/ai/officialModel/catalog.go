package officialmodel

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

//go:embed catalog/official-models.json
var embeddedCatalog []byte

type Catalog struct {
	Version    string
	SourceURL  string
	VerifiedAt time.Time
	Models     []CatalogModel
}

type CatalogModel struct {
	VendorCode      string
	VendorName      string
	ModelKey        string
	DisplayName     string
	InputUSDMicros  int64
	OutputUSDMicros int64
	SortOrder       int
}

type catalogDocument struct {
	CatalogVersion string               `json:"catalogVersion"`
	SourceURL      string               `json:"sourceUrl"`
	VerifiedAt     time.Time            `json:"verifiedAt"`
	Models         []catalogModelRecord `json:"models"`
}

type catalogModelRecord struct {
	VendorCode  string `json:"vendorCode"`
	VendorName  string `json:"vendorName"`
	ModelKey    string `json:"modelKey"`
	DisplayName string `json:"displayName"`
	InputUSD    string `json:"inputUsdPerMillion"`
	OutputUSD   string `json:"outputUsdPerMillion"`
}

func LoadCatalog() (Catalog, error) {
	return parseCatalog(embeddedCatalog)
}

func parseCatalog(raw []byte) (Catalog, error) {
	var document catalogDocument
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Catalog{}, fmt.Errorf("decode official model catalog: %w", err)
	}
	if strings.TrimSpace(document.CatalogVersion) == "" || utf8.RuneCountInString(document.CatalogVersion) > 64 {
		return Catalog{}, fmt.Errorf("catalogVersion is invalid")
	}
	if err := validateSourceURL(document.SourceURL); err != nil {
		return Catalog{}, err
	}
	if document.VerifiedAt.IsZero() {
		return Catalog{}, fmt.Errorf("verifiedAt is required")
	}
	if len(document.Models) == 0 || len(document.Models) > 2000 {
		return Catalog{}, fmt.Errorf("catalog must contain between 1 and 2000 models")
	}
	catalog := Catalog{
		Version:    strings.TrimSpace(document.CatalogVersion),
		SourceURL:  strings.TrimSpace(document.SourceURL),
		VerifiedAt: document.VerifiedAt.UTC(),
		Models:     make([]CatalogModel, 0, len(document.Models)),
	}
	seen := make(map[string]struct{}, len(document.Models))
	for index, record := range document.Models {
		entry, err := normalizeCatalogModel(record)
		if err != nil {
			return Catalog{}, fmt.Errorf("models[%d]: %w", index, err)
		}
		entry.SortOrder = index
		key := entry.VendorCode + "\x00" + entry.ModelKey
		if _, exists := seen[key]; exists {
			return Catalog{}, fmt.Errorf("models[%d]: duplicate model %s/%s", index, entry.VendorCode, entry.ModelKey)
		}
		seen[key] = struct{}{}
		catalog.Models = append(catalog.Models, entry)
	}
	return catalog, nil
}

func normalizeCatalogModel(record catalogModelRecord) (CatalogModel, error) {
	vendorCode := strings.TrimSpace(record.VendorCode)
	vendorName := strings.TrimSpace(record.VendorName)
	modelKey := strings.TrimSpace(record.ModelKey)
	displayName := strings.TrimSpace(record.DisplayName)
	if !validCode(vendorCode, 64) {
		return CatalogModel{}, fmt.Errorf("vendorCode is invalid")
	}
	if !validText(vendorName, 128) {
		return CatalogModel{}, fmt.Errorf("vendorName is invalid")
	}
	if !validText(modelKey, 200) {
		return CatalogModel{}, fmt.Errorf("modelKey is invalid")
	}
	if displayName == "" {
		displayName = modelKey
	}
	if !validText(displayName, 200) {
		return CatalogModel{}, fmt.Errorf("displayName is invalid")
	}
	input, err := parseMicros(record.InputUSD)
	if err != nil {
		return CatalogModel{}, fmt.Errorf("inputUsdPerMillion: %w", err)
	}
	output, err := parseMicros(record.OutputUSD)
	if err != nil {
		return CatalogModel{}, fmt.Errorf("outputUsdPerMillion: %w", err)
	}
	return CatalogModel{
		VendorCode: vendorCode, VendorName: vendorName, ModelKey: modelKey, DisplayName: displayName,
		InputUSDMicros: input, OutputUSDMicros: output,
	}, nil
}

func validateSourceURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 {
		return fmt.Errorf("sourceUrl is invalid")
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("sourceUrl must be an http(s) URL")
	}
	return nil
}

func validCode(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func validText(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
