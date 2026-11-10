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
	Vendor              Vendor
	ModelKey            string
	DisplayName         string
	ModelKind           int16
	InputUSDMicros      int64
	OutputUSDMicros     int64
	CacheReadUSDMicros  *int64
	CacheWriteUSDMicros *int64
	VideoPrices         []CatalogVideoPrice
	SourceURL           string
	VerifiedAt          time.Time
	SortOrder           int
}

type CatalogVideoPrice struct {
	TierKey        string
	TierLabel      string
	PriceCNYMicros *int64
	Unit           string
}

type catalogDocument struct {
	CatalogVersion string               `json:"catalogVersion"`
	SourceURL      string               `json:"sourceUrl"`
	VerifiedAt     time.Time            `json:"verifiedAt"`
	Models         []catalogModelRecord `json:"models"`
}

type catalogModelRecord struct {
	VendorCode    string                    `json:"vendorCode"`
	VendorName    string                    `json:"vendorName"`
	ModelKey      string                    `json:"modelKey"`
	DisplayName   string                    `json:"displayName"`
	ModelKind     int16                     `json:"modelKind"`
	InputUSD      string                    `json:"inputUsdPerMillion"`
	OutputUSD     string                    `json:"outputUsdPerMillion"`
	CacheReadUSD  string                    `json:"cacheReadUsdPerMillion"`
	CacheWriteUSD string                    `json:"cacheWriteUsdPerMillion"`
	VideoPrices   []catalogVideoPriceRecord `json:"videoPrices"`
	SourceURL     string                    `json:"sourceUrl"`
	VerifiedAt    time.Time                 `json:"verifiedAt"`
}

type catalogVideoPriceRecord struct {
	TierKey        string `json:"tierKey"`
	TierLabel      string `json:"tierLabel"`
	PriceCNYPerSec string `json:"priceCnyPerSecond"`
	Unit           string `json:"unit"`
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
		entry, err := normalizeCatalogModel(record, document.SourceURL, document.VerifiedAt)
		if err != nil {
			return Catalog{}, fmt.Errorf("models[%d]: %w", index, err)
		}
		entry.SortOrder = index
		key := fmt.Sprintf("%d\x00%s", entry.Vendor, entry.ModelKey)
		if _, exists := seen[key]; exists {
			return Catalog{}, fmt.Errorf("models[%d]: duplicate model %d/%s", index, entry.Vendor, entry.ModelKey)
		}
		seen[key] = struct{}{}
		catalog.Models = append(catalog.Models, entry)
	}
	return catalog, nil
}

func normalizeCatalogModel(record catalogModelRecord, fallbackSourceURL string, fallbackVerifiedAt time.Time) (CatalogModel, error) {
	vendorCode := strings.TrimSpace(record.VendorCode)
	modelKey := strings.TrimSpace(record.ModelKey)
	displayName := strings.TrimSpace(record.DisplayName)
	vendor, ok := vendorFromCode(vendorCode)
	if !ok {
		return CatalogModel{}, fmt.Errorf("vendorCode is invalid")
	}
	if vendorName := strings.TrimSpace(record.VendorName); vendorName != "" && vendorName != vendor.Label() {
		return CatalogModel{}, fmt.Errorf("vendorName does not match vendorCode")
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
	modelKind := record.ModelKind
	if modelKind == 0 {
		modelKind = 1
	}
	if modelKind < 1 || modelKind > 4 {
		return CatalogModel{}, fmt.Errorf("modelKind is invalid")
	}
	input, err := parseMicros(record.InputUSD)
	if err != nil {
		return CatalogModel{}, fmt.Errorf("inputUsdPerMillion: %w", err)
	}
	output, err := parseMicros(record.OutputUSD)
	if err != nil {
		return CatalogModel{}, fmt.Errorf("outputUsdPerMillion: %w", err)
	}
	cacheRead, err := parseCatalogOptionalMicros(record.CacheReadUSD)
	if err != nil {
		return CatalogModel{}, fmt.Errorf("cacheReadUsdPerMillion: %w", err)
	}
	cacheWrite, err := parseCatalogOptionalMicros(record.CacheWriteUSD)
	if err != nil {
		return CatalogModel{}, fmt.Errorf("cacheWriteUsdPerMillion: %w", err)
	}
	videoPrices, err := normalizeCatalogVideoPrices(record.VideoPrices)
	if err != nil {
		return CatalogModel{}, err
	}
	sourceURL := strings.TrimSpace(record.SourceURL)
	if sourceURL == "" {
		sourceURL = fallbackSourceURL
	}
	if err := validateSourceURL(sourceURL); err != nil {
		return CatalogModel{}, err
	}
	verifiedAt := record.VerifiedAt
	if verifiedAt.IsZero() {
		verifiedAt = fallbackVerifiedAt
	}
	if verifiedAt.IsZero() {
		return CatalogModel{}, fmt.Errorf("verifiedAt is required")
	}
	return CatalogModel{
		Vendor: vendor, ModelKey: modelKey, DisplayName: displayName, ModelKind: modelKind,
		InputUSDMicros: input, OutputUSDMicros: output,
		CacheReadUSDMicros: cacheRead, CacheWriteUSDMicros: cacheWrite,
		VideoPrices: videoPrices,
		SourceURL:   sourceURL, VerifiedAt: verifiedAt.UTC(),
	}, nil
}

func normalizeCatalogVideoPrices(records []catalogVideoPriceRecord) ([]CatalogVideoPrice, error) {
	if len(records) == 0 {
		return nil, nil
	}
	if len(records) > 40 {
		return nil, fmt.Errorf("videoPrices must contain at most 40 entries")
	}
	prices := make([]CatalogVideoPrice, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for index, record := range records {
		tierKey := strings.TrimSpace(record.TierKey)
		if !validText(tierKey, 64) {
			return nil, fmt.Errorf("videoPrices[%d].tierKey is invalid", index)
		}
		if _, exists := seen[tierKey]; exists {
			return nil, fmt.Errorf("videoPrices[%d].tierKey is duplicated", index)
		}
		seen[tierKey] = struct{}{}
		tierLabel := strings.TrimSpace(record.TierLabel)
		if tierLabel == "" {
			tierLabel = tierKey
		}
		if !validText(tierLabel, 64) {
			return nil, fmt.Errorf("videoPrices[%d].tierLabel is invalid", index)
		}
		unit := strings.TrimSpace(record.Unit)
		if unit == "" {
			unit = "second"
		}
		if unit != "second" && unit != "minute" && unit != "item" {
			return nil, fmt.Errorf("videoPrices[%d].unit is invalid", index)
		}
		price, err := parseCatalogOptionalMicros(record.PriceCNYPerSec)
		if err != nil {
			return nil, fmt.Errorf("videoPrices[%d].priceCnyPerSecond: %w", index, err)
		}
		prices = append(prices, CatalogVideoPrice{
			TierKey: tierKey, TierLabel: tierLabel, PriceCNYMicros: price, Unit: unit,
		})
	}
	return prices, nil
}

func parseCatalogOptionalMicros(value string) (*int64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := parseMicros(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
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
