package operationlog

import (
	"context"
	"fmt"
	"strings"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, payload TaskPayload) error {
	value := OperationLog{
		EventID: payload.EventID, RequestID: payload.RequestID, UserID: payload.UserID, SessionID: payload.SessionID,
		Platform: payload.Platform, Method: payload.Method, Route: payload.Route, Module: payload.Module,
		Action: payload.Action, ClientIP: payload.ClientIP, UserAgent: payload.UserAgent,
		StatusCode: int32(payload.StatusCode), IsSuccess: yesno.Value(payload.IsSuccess), LatencyMs: payload.LatencyMs,
		RequestData: payload.RequestData, ResponseData: payload.ResponseData,
		CreatedAt: payload.CreatedAt.UTC(), UpdatedAt: payload.CreatedAt.UTC(),
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).Create(&value)
	if result.Error != nil {
		return fmt.Errorf("insert operation log: %w", result.Error)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Item, int64, error) {
	db := applyFilters(r.db.WithContext(ctx).Model(&OperationLog{}), query)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count operation logs: %w", err)
	}
	rows := make([]OperationLog, 0, query.PageSize)
	if err := db.Order("created_at DESC, id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list operation logs: %w", err)
	}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, Item{
			ID: row.ID, RequestID: row.RequestID, UserID: row.UserID, SessionID: row.SessionID,
			Platform: valueOrEmpty(row.Platform), Method: row.Method, Route: row.Route, Module: row.Module,
			Action: row.Action, ClientIP: row.ClientIP, UserAgent: row.UserAgent, StatusCode: row.StatusCode,
			IsSuccess: int16(row.IsSuccess), LatencyMs: row.LatencyMs, RequestData: row.RequestData,
			ResponseData: row.ResponseData, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, total, nil
}

func applyFilters(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.UserID != nil {
		db = db.Where("user_id = ?", *query.UserID)
	}
	if query.Action != "" {
		db = db.Where("action LIKE ? ESCAPE '\\'", prefixPattern(query.Action))
	}
	if query.Route != "" {
		db = db.Where("route LIKE ? ESCAPE '\\'", prefixPattern(query.Route))
	}
	if query.IsSuccess != nil {
		db = db.Where("is_success = ?", *query.IsSuccess)
	}
	if query.From != nil {
		db = db.Where("created_at >= ?", *query.From)
	}
	if query.To != nil {
		db = db.Where("created_at <= ?", *query.To)
	}
	return db
}

func prefixPattern(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value + "%"
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
