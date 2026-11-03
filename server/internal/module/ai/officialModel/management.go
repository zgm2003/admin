package officialmodel

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type PriceView struct {
	ProviderModelID string          `json:"providerModelId"`
	ProviderID      string          `json:"providerId"`
	ProviderName    string          `json:"providerName"`
	UpstreamModelID string          `json:"upstreamModelId"`
	PriceVersionID  string          `json:"priceVersionId"`
	Version         string          `json:"version"`
	EffectiveAt     time.Time       `json:"effectiveAt"`
	Currency        string          `json:"currency"`
	SourceCurrency  string          `json:"sourceCurrency"`
	Rates           json.RawMessage `json:"rates"`
}

func (s *Service) Detail(ctx context.Context, id int64) (*Model, error) {
	return s.repository.Find(ctx, id)
}
func (s *Service) List(ctx context.Context, page, size int) ([]Model, int64, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, errors.New("AI official model pagination invalid")
	}
	r, ok := s.repository.(interface {
		List(context.Context, int, int) ([]Model, int64, error)
	})
	if !ok {
		return nil, 0, errors.New("AI official model list unavailable")
	}
	return r.List(ctx, page, size)
}
func (s *Service) Prices(ctx context.Context, id int64, page, size int) ([]PriceView, int64, error) {
	if _, err := s.repository.Find(ctx, id); err != nil {
		return nil, 0, err
	}
	r, ok := s.repository.(interface {
		Prices(context.Context, int64, int, int) ([]PriceView, int64, error)
	})
	if !ok {
		return nil, 0, errors.New("AI official model prices unavailable")
	}
	return r.Prices(ctx, id, page, size)
}
func (r *GormRepository) List(ctx context.Context, page, size int) ([]Model, int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Table("ai_official_model").Count(&count).Error; err != nil {
		return nil, 0, err
	}
	var ids []int64
	if err := r.db.WithContext(ctx).Table("ai_official_model").Order("updated_at DESC,id DESC").Offset((page-1)*size).Limit(size).Pluck("id", &ids).Error; err != nil {
		return nil, 0, err
	}
	out := make([]Model, 0, len(ids))
	for _, id := range ids {
		m, err := r.Find(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *m)
	}
	return out, count, nil
}
func (r *GormRepository) Prices(ctx context.Context, id int64, page, size int) ([]PriceView, int64, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, errors.New("AI official model price pagination invalid")
	}
	var count int64
	if err := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM ai_provider_model m WHERE m.official_model_id=? AND m.deleted_at IS NULL AND EXISTS(SELECT 1 FROM ai_provider_model_price_version v WHERE v.provider_model_id=m.id AND v.status=1 AND v.effective_at<=now())`, id).Scan(&count).Error; err != nil {
		return nil, 0, err
	}
	rows := []PriceView{}
	err := r.db.WithContext(ctx).Raw(`SELECT m.id::text AS provider_model_id,p.id::text AS provider_id,p.name AS provider_name,m.upstream_model_id,v.id::text AS price_version_id,v.version::text AS version,v.effective_at,v.currency,v.source_currency,COALESCE((SELECT jsonb_agg(jsonb_build_object('id',r.id::text,'operation',r.operation,'usageKey',r.usage_key,'selector',r.selector,'unit',r.unit,'unitQuantity',r.unit_quantity::text,'sourceUnitPrice',r.source_unit_price::text,'billingUnitPrice',(r.billing_unit_price_units::numeric/100000000)::text) ORDER BY r.id) FROM ai_provider_model_price_rate r WHERE r.price_version_id=v.id),'[]'::jsonb) AS rates FROM ai_provider_model m JOIN ai_provider p ON p.id=m.provider_id JOIN LATERAL(SELECT * FROM ai_provider_model_price_version WHERE provider_model_id=m.id AND status=1 AND effective_at<=now() ORDER BY effective_at DESC,id DESC LIMIT 1)v ON true WHERE m.official_model_id=? AND m.deleted_at IS NULL ORDER BY m.id LIMIT ? OFFSET ?`, id, size, (page-1)*size).Scan(&rows).Error
	return rows, count, err
}
