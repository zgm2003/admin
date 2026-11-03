package model

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

type SaveModelRequest struct {
	ID              string          `json:"id"`
	UpstreamModelID string          `json:"upstreamModelId"`
	Name            string          `json:"name"`
	OfficialModelID *int64          `json:"officialModelId,omitempty,string"`
	Operations      []string        `json:"operations"`
	Limits          json.RawMessage `json:"limits"`
}

func (s *ModelService) Find(ctx context.Context, providerID, id int64) (*Model, error) {
	return s.repository.Find(ctx, providerID, id)
}
func (s *ModelService) List(ctx context.Context, providerID int64, page, size int) ([]Model, int64, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, errors.New("AI provider model pagination invalid")
	}
	r, ok := s.repository.(interface {
		List(context.Context, int64, int, int) ([]Model, int64, error)
	})
	if !ok {
		return nil, 0, errors.New("AI provider model list unavailable")
	}
	return r.List(ctx, providerID, page, size)
}
func (s *ModelService) Save(ctx context.Context, providerID int64, requests []SaveModelRequest) error {
	if len(requests) == 0 || len(requests) > 1000 {
		return errors.New("AI provider models size invalid")
	}
	for _, req := range requests {
		if err := validModel(req.Name, req.Operations, req.Limits); err != nil {
			return err
		}
		if req.ID == "" && (req.UpstreamModelID == "" || len(req.UpstreamModelID) > 200) {
			return errors.New("AI provider model identity invalid")
		}
	}
	return s.mutate(ctx, func(repo ModelRepository) error {
		seen := map[int64]struct{}{}
		for _, req := range requests {
			if req.ID == "" {
				m := &Model{ProviderID: providerID, UpstreamModelID: req.UpstreamModelID, Name: req.Name, OfficialModelID: req.OfficialModelID, Operations: req.Operations, Limits: req.Limits, Status: 0}
				if err := repo.Create(ctx, m); err != nil {
					return err
				}
				continue
			}
			id, err := strconv.ParseInt(req.ID, 10, 64)
			if err != nil || id < 1 {
				return errors.New("AI provider model id invalid")
			}
			if _, ok := seen[id]; ok {
				return errors.New("AI provider model duplicated")
			}
			seen[id] = struct{}{}
			m, err := repo.Find(ctx, providerID, id)
			if err != nil {
				return err
			}
			m.Name = req.Name
			m.Operations = req.Operations
			m.Limits = req.Limits
			m.OfficialModelID = req.OfficialModelID
			if err := repo.Update(ctx, m); err != nil {
				return err
			}
		}
		return nil
	})
}
func (r *GormModelRepository) List(ctx context.Context, providerID int64, page, size int) ([]Model, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Table("ai_provider_model").Where("provider_id=? AND deleted_at IS NULL", providerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ids []int64
	if err := r.db.WithContext(ctx).Table("ai_provider_model").Where("provider_id=? AND deleted_at IS NULL", providerID).Order("id").Offset((page-1)*size).Limit(size).Pluck("id", &ids).Error; err != nil {
		return nil, 0, err
	}
	out := make([]Model, 0, len(ids))
	for _, id := range ids {
		m, err := r.Find(ctx, providerID, id)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *m)
	}
	return out, total, nil
}
func (s *Service) List(ctx context.Context, modelID int64, page, size int) ([]*PriceVersion, int64, error) {
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, errors.New("AI price pagination invalid")
	}
	r, ok := s.repository.(interface {
		List(context.Context, int64, int, int) ([]*PriceVersion, int64, error)
	})
	if !ok {
		return nil, 0, errors.New("AI price list unavailable")
	}
	return r.List(ctx, modelID, page, size)
}
func (r *GormRepository) List(ctx context.Context, modelID int64, page, size int) ([]*PriceVersion, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&priceVersionRow{}).Where("provider_model_id=?", modelID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []priceVersionRow
	if err := r.db.WithContext(ctx).Where("provider_model_id=?", modelID).Order("version DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*PriceVersion, 0, len(rows))
	for _, row := range rows {
		p, err := r.load(ctx, row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, nil
}
