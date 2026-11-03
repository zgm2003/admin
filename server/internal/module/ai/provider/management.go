package provider

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type DetailResponse struct {
	Provider           *Provider       `json:"provider"`
	BaseURL            string          `json:"baseUrl"`
	MediaHostAllowlist json.RawMessage `json:"mediaHostAllowlist"`
	RequestTimeoutMS   int             `json:"requestTimeoutMs"`
	HeadersConfigured  bool            `json:"headersConfigured"`
}
type OfficialModelCandidate struct {
	ID         int64           `json:"id,string"`
	VendorCode string          `json:"vendorCode"`
	ModelKey   string          `json:"modelKey"`
	Name       string          `json:"name"`
	Operations json.RawMessage `json:"operations"`
	Status     int16           `json:"status"`
}

var ErrReferenced = errors.New("AI provider is referenced; disable it instead")

func (s *Service) Configuration(ctx context.Context, id int64) (DetailResponse, error) {
	p, err := s.repository.Find(ctx, id)
	if err != nil {
		return DetailResponse{}, err
	}
	v, err := s.repository.CurrentVersion(ctx, id)
	if err != nil {
		return DetailResponse{}, err
	}
	return DetailResponse{p, v.BaseURL, v.MediaHostAllowlist, v.RequestTimeoutMS, v.HeadersCiphertext != nil}, nil
}
func (s *Service) List(ctx context.Context, page, size int) ([]Provider, int64, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, ErrInvalidConfiguration
	}
	r, ok := s.repository.(interface {
		List(context.Context, int, int) ([]Provider, int64, error)
	})
	if !ok {
		return nil, 0, errors.New("AI provider list repository unavailable")
	}
	return r.List(ctx, page, size)
}
func (s *Service) Delete(ctx context.Context, id int64, actorID int64) error {
	if id < 1 || actorID < 1 {
		return ErrInvalidConfiguration
	}
	return s.mutate(ctx, func(repo Repository) error {
		r, ok := repo.(interface {
			Delete(context.Context, int64, int64) error
		})
		if !ok {
			return errors.New("AI provider delete repository unavailable")
		}
		return r.Delete(ctx, id, actorID)
	})
}
func (s *Service) Candidates(ctx context.Context, keyword string, page, size int) ([]OfficialModelCandidate, int64, error) {
	if len(keyword) > 200 || page < 1 || size < 1 || size > 100 {
		return nil, 0, ErrInvalidConfiguration
	}
	r, ok := s.repository.(interface {
		Candidates(context.Context, string, int, int) ([]OfficialModelCandidate, int64, error)
	})
	if !ok {
		return nil, 0, errors.New("AI provider candidate repository unavailable")
	}
	return r.Candidates(ctx, keyword, page, size)
}
func (r *GormRepository) List(ctx context.Context, page, size int) ([]Provider, int64, error) {
	q := r.db.WithContext(ctx).Model(&Provider{}).Where("deleted_at IS NULL")
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []Provider{}
	if err := q.Select("ai_provider.*,true AS api_key_configured").Order("updated_at DESC,id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for i := range rows {
		rows[i].APIKeyConfigured = true
	}
	return rows, total, nil
}
func (r *GormRepository) Delete(ctx context.Context, id, actor int64) error {
	return r.WithProvider(ctx, id, func(repo Repository) error {
		tx := repo.(*GormRepository)
		var refs int64
		if err := tx.db.WithContext(ctx).Raw(`SELECT count(*) FROM ai_provider_model WHERE provider_id=?`, id).Scan(&refs).Error; err != nil {
			return err
		}
		if refs > 0 {
			return ErrReferenced
		}
		result := tx.db.WithContext(ctx).Model(&Provider{}).Where("id=? AND deleted_at IS NULL", id).Updates(map[string]any{"deleted_at": time.Now().UTC(), "updated_by": actor, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotFound
		}
		return nil
	})
}
func (r *GormRepository) Candidates(ctx context.Context, keyword string, page, size int) ([]OfficialModelCandidate, int64, error) {
	q := r.db.WithContext(ctx).Table("ai_official_model")
	if keyword != "" {
		q = q.Where("name ILIKE ? OR model_key ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	out := []OfficialModelCandidate{}
	if err := q.Select("id,vendor_code,model_key,name,to_json(operations) AS operations,status").Order("id").Offset((page - 1) * size).Limit(size).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
