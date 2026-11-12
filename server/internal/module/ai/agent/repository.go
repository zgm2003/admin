package agent

import (
	"context"
	"time"

	"admin/server/internal/module/ai/pricing"
	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type UpdateValues struct {
	Name            string
	Description     string
	SystemPrompt    string
	Scene           Scene
	AgentType       pricing.ModelKind
	ProviderModelID int64
	TimeoutMS       int32
	Status          yesno.Value
	UpdatedAt       time.Time
}

func (r *Repository) Create(ctx context.Context, row Model) (int64, error) {
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (Model, error) {
	var row Model
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return Model{}, err
	}
	return row, nil
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Model, int64, error) {
	countDB := agentFilter(r.db.WithContext(ctx).Model(&Model{}), query)
	var total int64
	if err := countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []Model
	if err := agentFilter(r.db.WithContext(ctx).Model(&Model{}), query).
		Order("id DESC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func agentFilter(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.Keyword != "" {
		pattern := "%" + query.Keyword + "%"
		db = db.Where("name ILIKE ? OR description ILIKE ?", pattern, pattern)
	}
	if query.Type != nil {
		db = db.Where("agent_type = ?", *query.Type)
	}
	if query.Scene != nil {
		db = db.Where("scene = ?", *query.Scene)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	return db
}

func (r *Repository) Update(ctx context.Context, id int64, values UpdateValues) error {
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(map[string]any{
		"name": values.Name, "description": values.Description,
		"system_prompt": values.SystemPrompt, "scene": values.Scene, "agent_type": values.AgentType,
		"provider_model_id": values.ProviderModelID, "timeout_ms": values.TimeoutMS, "status": values.Status,
		"updated_at": values.UpdatedAt, "version": gorm.Expr("version + 1"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, status yesno.Value, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ?", id).Updates(map[string]any{
		"status": status, "updated_at": now, "version": gorm.Expr("version + 1"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&Model{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ModelRefsByIDs(ctx context.Context, ids []int64) (map[int64]ModelRef, error) {
	result := make(map[int64]ModelRef, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []ModelRef
	if err := r.db.WithContext(ctx).
		Table("ai_provider_model AS pm").
		Select(`pm.id, pm.provider_id, p.name AS provider_name, p.status AS provider_status,
			pm.model_key, pm.display_name, pm.model_kind, pm.pricing_mode, pm.official_model_id,
			pm.status, (CASE WHEN pm.status = 1 AND pm.deleted_at IS NULL
				AND p.status = 1 AND p.deleted_at IS NULL THEN TRUE ELSE FALSE END) AS available`).
		Joins("JOIN ai_provider p ON p.id = pm.provider_id").
		Where("pm.id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func (r *Repository) ListUsableModelOptions(ctx context.Context) ([]ModelRef, error) {
	var rows []ModelRef
	if err := r.db.WithContext(ctx).
		Table("ai_provider_model AS pm").
		Select(`pm.id, pm.provider_id, p.name AS provider_name, p.status AS provider_status,
			pm.model_key, pm.display_name, pm.model_kind, pm.pricing_mode, pm.official_model_id,
			pm.status, TRUE AS available`).
		Joins("JOIN ai_provider p ON p.id = pm.provider_id").
		Where(`pm.status = 1 AND pm.deleted_at IS NULL AND p.status = 1 AND p.deleted_at IS NULL
			AND (
				(pm.model_kind = ? AND pm.pricing_mode = ? AND pm.official_model_id IS NOT NULL)
				OR (pm.model_kind <> ? AND (
					(pm.pricing_mode = ? AND pm.official_model_id IS NOT NULL)
					OR (pm.pricing_mode = ? AND EXISTS (
						SELECT 1 FROM ai_provider_model_price price
						WHERE price.provider_model_id = pm.id
							AND price.status = 1 AND price.deleted_at IS NULL
					))
				))
			)`,
			pricing.KindChat, pricing.ModeOfficialToken, pricing.KindChat,
			pricing.ModeOfficialToken, pricing.ModeProviderUnit).
		Order("p.sort_order ASC, p.id ASC, pm.display_name ASC, pm.id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) FindUsableModel(ctx context.Context, id int64, kind pricing.ModelKind) (ModelRef, error) {
	var row ModelRef
	if err := r.db.WithContext(ctx).
		Table("ai_provider_model AS pm").
		Select(`pm.id, pm.provider_id, p.name AS provider_name, p.status AS provider_status,
			pm.model_key, pm.display_name, pm.model_kind, pm.pricing_mode, pm.official_model_id,
			pm.status, TRUE AS available`).
		Joins("JOIN ai_provider p ON p.id = pm.provider_id").
		Where(`pm.id = ? AND pm.model_kind = ? AND pm.status = 1 AND pm.deleted_at IS NULL
			AND p.status = 1 AND p.deleted_at IS NULL
			AND (
				(pm.model_kind = ? AND pm.pricing_mode = ? AND pm.official_model_id IS NOT NULL)
				OR (pm.model_kind <> ? AND (
					(pm.pricing_mode = ? AND pm.official_model_id IS NOT NULL)
					OR (pm.pricing_mode = ? AND EXISTS (
						SELECT 1 FROM ai_provider_model_price price
						WHERE price.provider_model_id = pm.id
							AND price.status = 1 AND price.deleted_at IS NULL
					))
				))
			)`,
			id, kind, pricing.KindChat, pricing.ModeOfficialToken, pricing.KindChat,
			pricing.ModeOfficialToken, pricing.ModeProviderUnit).
		Take(&row).Error; err != nil {
		return ModelRef{}, err
	}
	return row, nil
}
