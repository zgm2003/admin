package officialmodel

import (
	ai "admin/server/internal/module/ai"
	"admin/server/internal/shared/apperror"
	configcache "admin/server/internal/shared/configCache"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"net/url"
	"time"
)

type Model struct {
	ID         int64           `json:"id,string"`
	VendorCode string          `json:"vendorCode"`
	ModelKey   string          `json:"modelKey"`
	Name       string          `json:"name"`
	Operations []string        `json:"operations" gorm:"-"`
	SourceURLs json.RawMessage `json:"sourceUrls"`
	Status     int16           `json:"status"`
	CreatedBy  int64           `json:"-"`
	UpdatedBy  int64           `json:"-"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}
type CreateRequest struct {
	VendorCode string          `json:"vendorCode"`
	ModelKey   string          `json:"modelKey"`
	Name       string          `json:"name"`
	Operations []string        `json:"operations"`
	SourceURLs json.RawMessage `json:"sourceUrls"`
	Status     int16           `json:"status"`
}
type UpdateRequest struct {
	Name       string          `json:"name"`
	Operations []string        `json:"operations"`
	SourceURLs json.RawMessage `json:"sourceUrls"`
}
type Repository interface {
	Mutate(context.Context, int64, func(Repository) error) (configcache.MutationResult, error)
	Create(context.Context, *Model) error
	Find(context.Context, int64) (*Model, error)
	Update(context.Context, *Model) error
	SetStatus(context.Context, int64, int16, int64) error
}
type RuntimeCoordinator interface {
	Mutate(context.Context, func(context.Context, int64) (configcache.MutationResult, error)) error
}
type Service struct {
	repository Repository
	runtime    RuntimeCoordinator
}

func NewService(r Repository) *Service                              { return &Service{repository: r} }
func (s *Service) SetRuntimeCoordinator(runtime RuntimeCoordinator) { s.runtime = runtime }
func (s *Service) mutate(ctx context.Context, fn func(Repository) error) error {
	if s.runtime == nil {
		return errors.New("AI runtime mutation coordinator unavailable")
	}
	return s.runtime.Mutate(ctx, func(ctx context.Context, base int64) (configcache.MutationResult, error) {
		return s.repository.Mutate(ctx, base, fn)
	})
}
func (s *Service) Create(ctx context.Context, req CreateRequest, actorID int64) (*Model, error) {
	if actorID <= 0 || req.VendorCode == "" || len(req.VendorCode) > 64 || req.ModelKey == "" || len(req.ModelKey) > 200 || (req.Status != 0 && req.Status != 1) {
		return nil, apperror.InvalidRequest(errors.New("AI official model identity invalid"))
	}
	if err := validateModel(req.Name, req.Operations, req.SourceURLs); err != nil {
		return nil, err
	}
	m := &Model{VendorCode: req.VendorCode, ModelKey: req.ModelKey, Name: req.Name, Operations: req.Operations, SourceURLs: req.SourceURLs, Status: req.Status, CreatedBy: actorID, UpdatedBy: actorID}
	if s.repository == nil {
		return nil, errors.New("AI official model repository unavailable")
	}
	if err := s.mutate(ctx, func(repo Repository) error { return repo.Create(ctx, m) }); err != nil {
		return nil, err
	}
	return m, nil
}
func (s *Service) Update(ctx context.Context, id int64, req UpdateRequest, actorID int64) (*Model, error) {
	if actorID <= 0 {
		return nil, apperror.InvalidRequest(errors.New("actor required"))
	}
	if err := validateModel(req.Name, req.Operations, req.SourceURLs); err != nil {
		return nil, err
	}
	m, err := s.repository.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Name = req.Name
	m.Operations = req.Operations
	m.SourceURLs = req.SourceURLs
	m.UpdatedBy = actorID
	if err := s.mutate(ctx, func(repo Repository) error { return repo.Update(ctx, m) }); err != nil {
		return nil, err
	}
	return m, nil
}
func (s *Service) SetStatus(ctx context.Context, id int64, status int16, actorID int64) error {
	if actorID <= 0 || (status != 0 && status != 1) {
		return apperror.InvalidRequest(errors.New("AI official model status invalid"))
	}
	return s.mutate(ctx, func(repo Repository) error { return repo.SetStatus(ctx, id, status, actorID) })
}
func validateModel(name string, ops []string, source []byte) error {
	if err := checkModel(name, ops, source); err != nil {
		return apperror.InvalidRequest(err)
	}
	return nil
}
func checkModel(name string, ops []string, source []byte) error {
	if name == "" || len(name) > 100 || len(ops) == 0 || len(ops) > 6 || len(source) > 16384 {
		return errors.New("AI official model fields invalid")
	}
	seen := map[string]struct{}{}
	for _, op := range ops {
		if !ai.Operation(op).Valid() {
			return errors.New("AI official model operation invalid")
		}
		if _, ok := seen[op]; ok {
			return errors.New("AI official model operation duplicated")
		}
		seen[op] = struct{}{}
	}
	var urls []string
	if json.Unmarshal(source, &urls) != nil || urls == nil || len(urls) > 32 {
		return errors.New("AI official model source URLs invalid")
	}
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return errors.New("AI official model source URL invalid")
		}
	}
	return nil
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }
func (r *GormRepository) Mutate(ctx context.Context, expected int64, fn func(Repository) error) (configcache.MutationResult, error) {
	var result configcache.MutationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := fn(NewRepository(tx)); err != nil {
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
func (r *GormRepository) Create(ctx context.Context, m *Model) error {
	ops, err := json.Marshal(m.Operations)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Raw(`INSERT INTO ai_official_model(vendor_code,model_key,name,operations,source_urls,status,created_by,updated_by) VALUES(?,?,?,ARRAY(SELECT jsonb_array_elements_text(?::jsonb)),?::jsonb,?,?,?) RETURNING id,created_at,updated_at`, m.VendorCode, m.ModelKey, m.Name, string(ops), string(m.SourceURLs), m.Status, m.CreatedBy, m.UpdatedBy).Scan(m).Error
}
func (r *GormRepository) Find(ctx context.Context, id int64) (*Model, error) {
	var row struct {
		ID                         int64
		VendorCode, ModelKey, Name string
		Operations                 []byte
		SourceURLs                 json.RawMessage
		Status                     int16
		CreatedBy, UpdatedBy       int64
		CreatedAt, UpdatedAt       time.Time
	}
	res := r.db.WithContext(ctx).Raw(`SELECT id,vendor_code,model_key,name,to_json(operations) AS operations,source_urls,status,created_by,updated_by,created_at,updated_at FROM ai_official_model WHERE id=?`, id).Scan(&row)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	var ops []string
	if err := json.Unmarshal(row.Operations, &ops); err != nil {
		return nil, err
	}
	return &Model{ID: row.ID, VendorCode: row.VendorCode, ModelKey: row.ModelKey, Name: row.Name, Operations: ops, SourceURLs: row.SourceURLs, Status: row.Status, CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}
func (r *GormRepository) Update(ctx context.Context, m *Model) error {
	ops, err := json.Marshal(m.Operations)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Exec(`UPDATE ai_official_model SET name=?,operations=ARRAY(SELECT jsonb_array_elements_text(?::jsonb)),source_urls=?::jsonb,updated_by=?,updated_at=now() WHERE id=?`, m.Name, string(ops), string(m.SourceURLs), m.UpdatedBy, m.ID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *GormRepository) SetStatus(ctx context.Context, id int64, status int16, actorID int64) error {
	res := r.db.WithContext(ctx).Exec(`UPDATE ai_official_model SET status=?,updated_by=?,updated_at=now() WHERE id=?`, status, actorID, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
