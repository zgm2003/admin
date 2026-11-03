package model

import (
	configcache "admin/server/internal/shared/configCache"
	"context"
	"errors"
	"time"
)

type PriceVersion struct {
	ID              int64
	ProviderModelID int64
	Version         int64
	Status          int16
	EffectiveAt     *time.Time
	Currency        string
	SourceCurrency  string
	SourceDigest    string
	SnapshotDigest  string
	SourcePayload   []byte
	SyncedAt        time.Time
	CreatedBy       int64
	PublishedBy     *int64
	PublishedAt     *time.Time
	Rates           []PriceRate
}
type PriceRate struct {
	ID                    int64
	PriceVersionID        int64
	Operation             string
	UsageKey              string
	Selector              []byte
	SelectorHash          string
	Unit                  string
	UnitQuantity          string
	SourceUnitPrice       string
	BillingUnitPriceUnits *int64
}
type SourceSnapshot struct {
	ExpectedProviderVersionID int64
	Digest                    string
	Payload                   []byte
	SyncedAt                  time.Time
	Rates                     []SourceRate
	EffectiveAt               *time.Time
}
type SourceRate struct {
	Operation, UsageKey                 string
	Selector                            []byte
	Unit, UnitQuantity, SourceUnitPrice string
}
type UpdatePriceRequest struct {
	Rates       []UpdateRate
	EffectiveAt *time.Time
}
type UpdateRate struct {
	RateID                int64
	BillingUnitPriceUnits *int64
}

var ErrPriceNotFound = errors.New("AI price version not found")
var ErrPublishedImmutable = errors.New("AI published price version is immutable")
var ErrPriceDigestMismatch = errors.New("AI price preview digest mismatch")
var ErrIncompleteBillingPrice = errors.New("AI price preview has incomplete billing rates")
var ErrSourceVersionChanged = errors.New("AI provider credentials changed during price sync")

type Repository interface {
	CurrentProviderVersionID(context.Context, int64) (int64, error)
	Mutate(context.Context, int64, func(Repository) error) (configcache.MutationResult, error)
	WithModel(context.Context, int64, func(Repository) error) error
	CreatePreview(context.Context, *PriceVersion) error
	Latest(context.Context, int64) (*PriceVersion, error)
	Find(context.Context, int64, int64) (*PriceVersion, error)
	UpdatePreview(context.Context, *PriceVersion) error
	Publish(context.Context, int64, int64, string, int64) error
	Effective(context.Context, int64, time.Time) (*PriceVersion, error)
}
