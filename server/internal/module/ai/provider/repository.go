package provider

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }
func (r *GormRepository) WithProvider(ctx context.Context, id int64, fn func(Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p Provider
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).First(&p).Error; err != nil {
			return err
		}
		return fn(NewRepository(tx))
	})
}
func (r *GormRepository) Create(ctx context.Context, p *Provider, v *ProviderVersion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(p).Error; err != nil {
			return err
		}
		v.ProviderID = p.ID
		if err := tx.Create(v).Error; err != nil {
			return err
		}
		return nil
	})
}
func (r *GormRepository) Find(ctx context.Context, id int64) (*Provider, error) {
	var p Provider
	if err := r.db.WithContext(ctx).Where("id=? AND deleted_at IS NULL", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var configured int64
	if err := r.db.WithContext(ctx).Raw("SELECT count(*) FROM ai_provider_version WHERE provider_id=? AND length(api_key_ciphertext)>0", id).Scan(&configured).Error; err != nil {
		return nil, err
	}
	p.APIKeyConfigured = configured > 0
	return &p, nil
}
func (r *GormRepository) CurrentVersion(ctx context.Context, id int64) (*ProviderVersion, error) {
	var p Provider
	if err := r.db.WithContext(ctx).Select("current_version").First(&p, id).Error; err != nil {
		return nil, err
	}
	var v ProviderVersion
	if err := r.db.WithContext(ctx).Where("provider_id=? AND version=?", id, p.CurrentVersion).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// Version reads a previously frozen credential version, never the current pointer.
func (r *GormRepository) Version(ctx context.Context, id int64) (*ProviderVersion, error) {
	var v ProviderVersion
	if err := r.db.WithContext(ctx).First(&v, id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}
func (r *GormRepository) SaveRevision(ctx context.Context, p *Provider, v *ProviderVersion) error {
	if err := r.db.WithContext(ctx).Create(v).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&Provider{}).Where("id=?", p.ID).Updates(map[string]any{"name": p.Name, "description": p.Description, "current_version": p.CurrentVersion, "updated_by": p.UpdatedBy}).Error
}
func (r *GormRepository) SetStatus(ctx context.Context, id int64, status int16) error {
	res := r.db.WithContext(ctx).Model(&Provider{}).Where("id=? AND deleted_at IS NULL", id).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrNotFound
	}
	return nil
}
func (r *GormRepository) Models(ctx context.Context, id int64) ([]ProviderModel, error) {
	var rows []struct {
		ID, ProviderID        int64
		UpstreamModelID, Name string
		OfficialModelID       *int64
		Operations            []byte
		Limits                []byte
		Status                int16
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT id,provider_id,upstream_model_id,name,official_model_id,to_json(operations) AS operations,limits,status FROM ai_provider_model WHERE provider_id=? AND deleted_at IS NULL ORDER BY id`, id).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ProviderModel, 0, len(rows))
	for _, row := range rows {
		var ops []string
		if err := json.Unmarshal(row.Operations, &ops); err != nil {
			return nil, err
		}
		out = append(out, ProviderModel{ID: row.ID, ProviderID: row.ProviderID, UpstreamModelID: row.UpstreamModelID, Name: row.Name, OfficialModelID: row.OfficialModelID, Operations: ops, Limits: row.Limits, Status: row.Status})
	}
	return out, nil
}
func (r *GormRepository) SaveCatalog(ctx context.Context, providerID int64, models []ProviderModel) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, m := range models {
			ops, err := json.Marshal(m.Operations)
			if err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO ai_provider_model(provider_id,upstream_model_id,name,official_model_id,operations,limits,status) VALUES(?,?,?,?,ARRAY(SELECT jsonb_array_elements_text(?::jsonb)),?::jsonb,0)`, providerID, m.UpstreamModelID, m.Name, m.OfficialModelID, string(ops), string(m.Limits)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
