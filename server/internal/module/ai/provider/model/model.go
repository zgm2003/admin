package model

import (
	ai "admin/server/internal/module/ai"
	"admin/server/internal/shared/apperror"
	configcache "admin/server/internal/shared/configCache"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type Model struct {
	ID              int64           `json:"id,string"`
	ProviderID      int64           `json:"providerId,string"`
	UpstreamModelID string          `json:"upstreamModelId"`
	Name            string          `json:"name"`
	OfficialModelID *int64          `json:"officialModelId,omitempty,string"`
	Operations      []string        `json:"operations" gorm:"-"`
	Limits          json.RawMessage `json:"limits"`
	Status          int16           `json:"status"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt  `json:"-"`
}
type CreateModelRequest struct {
	UpstreamModelID, Name string
	OfficialModelID       *int64
	Operations            []string
	Limits                json.RawMessage
}
type UpdateModelRequest struct {
	Name            string
	OfficialModelID *int64
	Operations      []string
	Limits          json.RawMessage
}

var ErrModelNotFound = errors.New("AI provider model not found")

type ModelRepository interface {
	Mutate(context.Context, int64, func(ModelRepository) error) (configcache.MutationResult, error)
	Create(context.Context, *Model) error
	Find(context.Context, int64, int64) (*Model, error)
	Update(context.Context, *Model) error
	SetStatus(context.Context, int64, int64, int16) error
}
type ModelService struct {
	repository ModelRepository
	runtime    RuntimeCoordinator
}

func NewModelService(r ModelRepository) *ModelService                    { return &ModelService{repository: r} }
func (s *ModelService) SetRuntimeCoordinator(runtime RuntimeCoordinator) { s.runtime = runtime }
func (s *ModelService) mutate(ctx context.Context, fn func(ModelRepository) error) error {
	if s.runtime == nil {
		return errors.New("AI runtime mutation coordinator unavailable")
	}
	return s.runtime.Mutate(ctx, func(ctx context.Context, base int64) (configcache.MutationResult, error) {
		return s.repository.Mutate(ctx, base, fn)
	})
}
func (s *ModelService) Create(ctx context.Context, providerID int64, req CreateModelRequest) (*Model, error) {
	if providerID <= 0 || req.UpstreamModelID == "" || len(req.UpstreamModelID) > 200 {
		return nil, apperror.InvalidRequest(errors.New("AI provider model identity invalid"))
	}
	if err := validModel(req.Name, req.Operations, req.Limits); err != nil {
		return nil, err
	}
	m := &Model{ProviderID: providerID, UpstreamModelID: req.UpstreamModelID, Name: req.Name, OfficialModelID: req.OfficialModelID, Operations: req.Operations, Limits: req.Limits, Status: 0}
	if s.repository == nil {
		return nil, errors.New("AI model repository unavailable")
	}
	if err := s.mutate(ctx, func(repo ModelRepository) error { return repo.Create(ctx, m) }); err != nil {
		return nil, err
	}
	return m, nil
}
func (s *ModelService) Update(ctx context.Context, providerID, id int64, req UpdateModelRequest) (*Model, error) {
	if err := validModel(req.Name, req.Operations, req.Limits); err != nil {
		return nil, err
	}
	m, err := s.repository.Find(ctx, providerID, id)
	if err != nil {
		return nil, err
	}
	if m.Status == 1 && req.OfficialModelID == nil {
		return nil, apperror.InvalidRequest(errors.New("enabled AI model requires official mapping"))
	}
	m.Name = req.Name
	m.OfficialModelID = req.OfficialModelID
	m.Operations = req.Operations
	m.Limits = req.Limits
	if err := s.mutate(ctx, func(repo ModelRepository) error { return repo.Update(ctx, m) }); err != nil {
		return nil, err
	}
	return m, nil
}
func (s *ModelService) SetStatus(ctx context.Context, providerID, id int64, status int16) error {
	if status != 0 && status != 1 {
		return apperror.InvalidRequest(errors.New("AI provider model status invalid"))
	}
	m, err := s.repository.Find(ctx, providerID, id)
	if err != nil {
		return err
	}
	if status == 1 && m.OfficialModelID == nil {
		return apperror.InvalidRequest(errors.New("AI provider model mapping required"))
	}
	return s.mutate(ctx, func(repo ModelRepository) error { return repo.SetStatus(ctx, providerID, id, status) })
}
func validModel(name string, operations []string, limits []byte) error {
	if err := checkModel(name, operations, limits); err != nil {
		return apperror.InvalidRequest(err)
	}
	return nil
}
func checkModel(name string, operations []string, limits []byte) error {
	if name == "" || len(name) > 100 || len(operations) == 0 || len(operations) > 6 || len(limits) > 65536 {
		return errors.New("AI provider model fields invalid")
	}
	seen := map[string]struct{}{}
	for _, op := range operations {
		if !ai.Operation(op).Valid() {
			return errors.New("AI provider model operation invalid")
		}
		if _, ok := seen[op]; ok {
			return errors.New("AI provider model operation duplicated")
		}
		seen[op] = struct{}{}
	}
	_, err := ai.DecodeJSONObject(limits)
	return err
}

type GormModelRepository struct{ db *gorm.DB }

func NewModelRepository(db *gorm.DB) *GormModelRepository { return &GormModelRepository{db: db} }
func (r *GormModelRepository) Mutate(ctx context.Context, expected int64, fn func(ModelRepository) error) (configcache.MutationResult, error) {
	var result configcache.MutationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := fn(NewModelRepository(tx)); err != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, err)
		}
		event, err := configcache.NewRepository(tx).AdvanceTx(ctx, tx, configcache.Scope{Namespace: "ai.runtime", ScopeKey: "global"}, expected, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, err)
		}
		result = configcache.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	return result, err
}
func (r *GormModelRepository) Create(ctx context.Context, m *Model) error {
	ops, err := json.Marshal(m.Operations)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Raw(`INSERT INTO ai_provider_model(provider_id,upstream_model_id,name,official_model_id,operations,limits,status) VALUES(?,?,?,?,ARRAY(SELECT jsonb_array_elements_text(?::jsonb)),?::jsonb,0) RETURNING id,created_at,updated_at`, m.ProviderID, m.UpstreamModelID, m.Name, m.OfficialModelID, string(ops), string(m.Limits)).Scan(m).Error
}
func (r *GormModelRepository) Find(ctx context.Context, providerID, id int64) (*Model, error) {
	var row struct {
		ID, ProviderID        int64
		UpstreamModelID, Name string
		OfficialModelID       *int64
		Operations            []byte
		Limits                json.RawMessage
		Status                int16
		CreatedAt, UpdatedAt  time.Time
	}
	res := r.db.WithContext(ctx).Raw(`SELECT id,provider_id,upstream_model_id,name,official_model_id,to_json(operations) AS operations,limits,status,created_at,updated_at FROM ai_provider_model WHERE provider_id=? AND id=? AND deleted_at IS NULL`, providerID, id).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, ErrModelNotFound
	}
	var ops []string
	if err := json.Unmarshal(row.Operations, &ops); err != nil {
		return nil, err
	}
	return &Model{ID: row.ID, ProviderID: row.ProviderID, UpstreamModelID: row.UpstreamModelID, Name: row.Name, OfficialModelID: row.OfficialModelID, Operations: ops, Limits: row.Limits, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}
func (r *GormModelRepository) Update(ctx context.Context, m *Model) error {
	ops, err := json.Marshal(m.Operations)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Exec(`UPDATE ai_provider_model SET name=?,official_model_id=?,operations=ARRAY(SELECT jsonb_array_elements_text(?::jsonb)),limits=?::jsonb,updated_at=now() WHERE provider_id=? AND id=? AND deleted_at IS NULL`, m.Name, m.OfficialModelID, string(ops), string(m.Limits), m.ProviderID, m.ID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrPriceNotFound
	}
	return nil
}
func (r *GormModelRepository) SetStatus(ctx context.Context, providerID, id int64, status int16) error {
	res := r.db.WithContext(ctx).Exec(`UPDATE ai_provider_model SET status=?,updated_at=now() WHERE provider_id=? AND id=? AND deleted_at IS NULL`, status, providerID, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrPriceNotFound
	}
	return nil
}
