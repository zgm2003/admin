package tool

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

type UpdateValues struct {
	APIName     string
	Name        string
	Description string
	Category    string
	Parameters  string
	Method      string
	URL         string
	TimeoutMS   int32
	Status      Status
	SortOrder   int32
	UpdatedAt   time.Time
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(ctx context.Context) ([]Tool, error) {
	var rows []Tool
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Order("sort_order, id").Find(&rows).Error
	return rows, err
}

func (r *Repository) Enabled(ctx context.Context) ([]Tool, error) {
	var rows []Tool
	err := r.db.WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL", StatusEnabled).
		Order("sort_order, id").Find(&rows).Error
	return rows, err
}

func (r *Repository) EnabledForAgent(ctx context.Context, agentID int64) ([]Tool, error) {
	var rows []Tool
	err := r.db.WithContext(ctx).
		Table("ai_tool AS tool").
		Select("tool.*").
		Joins("JOIN ai_agent_tool AS agent_tool ON agent_tool.tool_id = tool.id").
		Where("agent_tool.agent_id = ? AND tool.status = ? AND tool.deleted_at IS NULL", agentID, StatusEnabled).
		Order("tool.sort_order, tool.id").Find(&rows).Error
	return rows, err
}

func (r *Repository) FindByCode(ctx context.Context, code string) (Tool, error) {
	var row Tool
	err := r.db.WithContext(ctx).Where("code = ? AND deleted_at IS NULL", code).First(&row).Error
	return row, err
}

func (r *Repository) FindByIdentifier(ctx context.Context, identifier string) (Tool, error) {
	var row Tool
	err := r.db.WithContext(ctx).
		Where("(code = ? OR api_name = ?) AND deleted_at IS NULL", identifier, identifier).
		First(&row).Error
	return row, err
}

func (r *Repository) Create(ctx context.Context, row Tool) (int64, error) {
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *Repository) Update(ctx context.Context, code string, values UpdateValues) (int64, error) {
	result := r.db.WithContext(ctx).Model(&Tool{}).
		Where("code = ? AND deleted_at IS NULL", code).
		Updates(map[string]any{
			"api_name": values.APIName, "name": values.Name, "description": values.Description,
			"category": values.Category, "parameters": values.Parameters, "method": values.Method,
			"url": values.URL, "timeout_ms": values.TimeoutMS, "status": values.Status,
			"sort_order": values.SortOrder, "updated_at": values.UpdatedAt,
		})
	return result.RowsAffected, result.Error
}

func (r *Repository) UpdateStatus(ctx context.Context, code string, status Status, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&Tool{}).
		Where("code = ? AND deleted_at IS NULL", code).
		Updates(map[string]any{"status": status, "updated_at": now})
	return result.RowsAffected, result.Error
}

func (r *Repository) Delete(ctx context.Context, code string, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&Tool{}).
		Where("code = ? AND deleted_at IS NULL", code).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	return result.RowsAffected, result.Error
}
