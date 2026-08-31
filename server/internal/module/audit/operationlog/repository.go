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
		PlatformID: payload.PlatformID, Method: payload.Method, Route: payload.Route, Module: payload.Module,
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
	db := applyFilters(r.db.WithContext(ctx).Table("audit_operation_log"), query)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count operation logs: %w", err)
	}
	type row struct {
		OperationLog `gorm:"embedded"`
		Platform     string `gorm:"column:platform"`
		UserName     string `gorm:"column:user_name"`
	}
	rows := make([]row, 0, query.PageSize)
	if err := db.Select("audit_operation_log.*, COALESCE(auth_platform.code, '') AS platform, COALESCE(user_account.username, '') AS user_name").
		Joins("LEFT JOIN auth_platform ON auth_platform.id = audit_operation_log.platform_id").
		Joins("LEFT JOIN user_account ON user_account.id = audit_operation_log.user_id AND user_account.deleted_at IS NULL").
		Order("audit_operation_log.created_at DESC, audit_operation_log.id DESC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list operation logs: %w", err)
	}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, Item{
			ID: row.ID, RequestID: row.RequestID, UserID: row.UserID, SessionID: row.SessionID,
			UserName: row.UserName,
			Platform: row.Platform, Method: row.Method, Route: row.Route, Module: row.Module,
			Action: row.Action, ClientIP: row.ClientIP, UserAgent: row.UserAgent, StatusCode: row.StatusCode,
			IsSuccess: int16(row.IsSuccess), LatencyMs: row.LatencyMs, RequestData: row.RequestData,
			ResponseData: row.ResponseData, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, total, nil
}

func applyFilters(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.UserID != nil {
		db = db.Where("audit_operation_log.user_id = ?", *query.UserID)
	}
	if query.Action != "" {
		db = db.Where("audit_operation_log.action LIKE ? ESCAPE '\\'", prefixPattern(query.Action))
	}
	if query.Route != "" {
		db = db.Where("audit_operation_log.route LIKE ? ESCAPE '\\'", prefixPattern(query.Route))
	}
	if query.IsSuccess != nil {
		db = db.Where("audit_operation_log.is_success = ?", *query.IsSuccess)
	}
	if query.From != nil {
		db = db.Where("audit_operation_log.created_at >= ?", *query.From)
	}
	if query.To != nil {
		db = db.Where("audit_operation_log.created_at <= ?", *query.To)
	}
	return db
}

func prefixPattern(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value + "%"
}
