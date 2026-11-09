package officialmodel

import (
	"fmt"
	"time"
)

type Actions struct {
	Update bool `json:"update"`
	Reset  bool `json:"reset"`
}

type SafeValue struct {
	ID                      int64     `json:"id"`
	Vendor                  Vendor    `json:"vendor"`
	VendorName              string    `json:"vendorName"`
	ModelKey                string    `json:"modelKey"`
	DisplayName             string    `json:"displayName"`
	InputUSDPerMillion      string    `json:"inputUsdPerMillion"`
	OutputUSDPerMillion     string    `json:"outputUsdPerMillion"`
	InputCNYPerMillion      string    `json:"inputCnyPerMillion"`
	OutputCNYPerMillion     string    `json:"outputCnyPerMillion"`
	ModelKind               int16     `json:"modelKind"`
	CacheReadCNYPerMillion  *string   `json:"cacheReadCnyPerMillion"`
	CacheWriteCNYPerMillion *string   `json:"cacheWriteCnyPerMillion"`
	CacheReadUSDPerMillion  *string   `json:"cacheReadUsdPerMillion"`
	CacheWriteUSDPerMillion *string   `json:"cacheWriteUsdPerMillion"`
	CatalogVersion          string    `json:"catalogVersion"`
	SourceURL               string    `json:"sourceUrl"`
	VerifiedAt              time.Time `json:"verifiedAt"`
	Version                 int64     `json:"version"`
	Actions                 Actions   `json:"actions"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

type SyncResult struct {
	Inserted  int `json:"inserted"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
}

func safeValue(row Model) (SafeValue, error) {
	if row.ID < 1 || row.Version < 1 || !row.Vendor.Valid() ||
		!validText(row.ModelKey, 200) || !validText(row.DisplayName, 200) ||
		!validText(row.CatalogVersion, 64) || row.SourceURL == "" ||
		row.VerifiedAt.IsZero() ||
		row.InputUSDMicros < 0 || row.OutputUSDMicros < 0 ||
		row.InputCNYMicros < 0 || row.OutputCNYMicros < 0 ||
		row.ModelKind < 1 || row.ModelKind > 4 ||
		(row.CacheReadCNY != nil && *row.CacheReadCNY < 0) ||
		(row.CacheWriteCNY != nil && *row.CacheWriteCNY < 0) ||
		(row.CacheReadUSD != nil && *row.CacheReadUSD < 0) ||
		(row.CacheWriteUSD != nil && *row.CacheWriteUSD < 0) {
		return SafeValue{}, fmt.Errorf("stored official model is invalid")
	}
	if err := validateSourceURL(row.SourceURL); err != nil {
		return SafeValue{}, fmt.Errorf("stored official model source is invalid")
	}
	return SafeValue{
		ID: row.ID, Vendor: row.Vendor, VendorName: row.Vendor.Label(),
		ModelKey: row.ModelKey, DisplayName: row.DisplayName,
		InputUSDPerMillion:      formatMicros(row.InputUSDMicros),
		OutputUSDPerMillion:     formatMicros(row.OutputUSDMicros),
		InputCNYPerMillion:      formatMicros(row.InputCNYMicros),
		OutputCNYPerMillion:     formatMicros(row.OutputCNYMicros),
		ModelKind:               row.ModelKind,
		CacheReadCNYPerMillion:  formatOptionalMicros(row.CacheReadCNY),
		CacheWriteCNYPerMillion: formatOptionalMicros(row.CacheWriteCNY),
		CacheReadUSDPerMillion:  formatOptionalMicros(row.CacheReadUSD),
		CacheWriteUSDPerMillion: formatOptionalMicros(row.CacheWriteUSD),
		CatalogVersion:          row.CatalogVersion, SourceURL: row.SourceURL, VerifiedAt: row.VerifiedAt.UTC(),
		Version: row.Version, Actions: Actions{Update: true, Reset: true},
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}
