package provider

import (
	price "admin/server/internal/module/ai/provider/model"
	"context"
	"encoding/json"
	"errors"
)

type PricingSource interface {
	FetchPricing(context.Context) (OpenAIPricingSnapshot, error)
}
type PricePreviewer interface {
	PreviewSync(context.Context, int64, price.SourceSnapshot, int64) (*price.PriceVersion, error)
}
type PriceSyncService struct {
	providers Repository
	source    PricingSource
	prices    PricePreviewer
}

func NewPriceSyncService(providers Repository, source PricingSource, prices PricePreviewer) *PriceSyncService {
	return &PriceSyncService{providers: providers, source: source, prices: prices}
}
func (s *PriceSyncService) Preview(ctx context.Context, providerID, providerModelID, actorID int64) (*price.PriceVersion, error) {
	if s.source == nil || s.prices == nil {
		return nil, errors.New("AI price source unavailable")
	}
	p, err := s.providers.Find(ctx, providerID)
	if err != nil {
		return nil, err
	}
	v, err := s.providers.CurrentVersion(ctx, providerID)
	if err != nil {
		return nil, err
	}
	models, err := s.providers.Models(ctx, providerID)
	if err != nil {
		return nil, err
	}
	var target *ProviderModel
	for i := range models {
		if models[i].ID == providerModelID {
			target = &models[i]
			break
		}
	}
	if target == nil {
		return nil, ErrNotFound
	}
	raw, err := s.source.FetchPricing(ctx)
	if err != nil {
		return nil, err
	}
	filtered, err := raw.ForModel(p.Code, v.BaseURL, target.UpstreamModelID)
	if err != nil {
		return nil, err
	}
	audit, err := json.Marshal(map[string]any{"sourceUrls": []string{OpenAIPricingURL, OpenAIImagePricingURL, OpenAITranscriptionPricingURL, OpenAISpeechPricingURL}, "responseDigest": filtered.Digest, "modelKey": target.UpstreamModelID, "document": string(filtered.Payload)})
	if err != nil {
		return nil, err
	}
	rates := make([]price.SourceRate, 0, len(filtered.Rates))
	for _, r := range filtered.Rates {
		rates = append(rates, price.SourceRate{Operation: r.Operation, UsageKey: r.UsageKey, Selector: r.Selector, Unit: r.Unit, UnitQuantity: r.UnitQuantity, SourceUnitPrice: r.SourceUnitPrice})
	}
	// PriceService acquires the shared mutation coordinator before taking model
	// locks and verifies this credential version inside its transaction.
	return s.prices.PreviewSync(ctx, providerModelID, price.SourceSnapshot{ExpectedProviderVersionID: v.ID, Digest: filtered.Digest, Payload: audit, SyncedAt: filtered.SyncedAt, Rates: rates}, actorID)
}
