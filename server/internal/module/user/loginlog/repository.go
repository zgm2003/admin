package loginlog

import (
	"context"
	"fmt"
	"time"

	"admin/server/internal/shared/yesno"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Insert(ctx context.Context, event Event) error {
	if err := ValidateEvent(event); err != nil {
		return err
	}
	now := time.Now().UTC()
	value := LoginLog{UserID: event.UserID, SessionID: event.SessionID, PlatformID: event.PlatformID, LoginAccount: event.LoginAccount, EventType: event.EventType, LoginType: event.LoginType, IsSuccess: event.IsSuccess, ReasonCode: event.ReasonCode, ClientIP: event.ClientIP, UserAgent: event.UserAgent, CreatedAt: now, UpdatedAt: now}
	if err := r.db.WithContext(ctx).Create(&value).Error; err != nil {
		return fmt.Errorf("insert login log: %w", err)
	}
	return nil
}

type ListQuery struct {
	Page         int
	PageSize     int
	UserID       *int64
	PlatformID   *int64
	EventType    string
	LoginType    string
	IsSuccess    *yesno.Value
	LoginAccount string
	From         *time.Time
	To           *time.Time
}

type Item struct {
	ID           int64     `json:"id"`
	UserID       *int64    `json:"userId"`
	SessionID    *int64    `json:"sessionId"`
	Platform     string    `json:"platform"`
	LoginAccount string    `json:"loginAccount"`
	EventType    string    `json:"eventType"`
	LoginType    *string   `json:"loginType"`
	IsSuccess    int16     `json:"isSuccess"`
	ReasonCode   string    `json:"reasonCode"`
	ClientIP     string    `json:"clientIp"`
	UserAgent    string    `json:"userAgent"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Item, int64, error) {
	db := r.db.WithContext(ctx).Table("user_login_log AS login_log")
	if query.UserID != nil {
		db = db.Where("login_log.user_id = ?", *query.UserID)
	}
	if query.PlatformID != nil {
		db = db.Where("login_log.platform_id = ?", *query.PlatformID)
	}
	if query.EventType != "" {
		db = db.Where("login_log.event_type = ?", query.EventType)
	}
	if query.LoginType != "" {
		db = db.Where("login_log.login_type = ?", query.LoginType)
	}
	if query.IsSuccess != nil {
		db = db.Where("login_log.is_success = ?", *query.IsSuccess)
	}
	if query.LoginAccount != "" {
		db = db.Where("login_log.login_account ILIKE ?", query.LoginAccount+"%")
	}
	if query.From != nil {
		db = db.Where("login_log.created_at >= ?", *query.From)
	}
	if query.To != nil {
		db = db.Where("login_log.created_at <= ?", *query.To)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count login logs: %w", err)
	}
	items := make([]Item, 0, query.PageSize)
	if err := db.Select("login_log.id, login_log.user_id, login_log.session_id, COALESCE(platform.code, '') AS platform, login_log.login_account, login_log.event_type, login_log.login_type, login_log.is_success, login_log.reason_code, login_log.client_ip, login_log.user_agent, login_log.created_at").
		Joins("JOIN auth_platform AS platform ON platform.id = login_log.platform_id").
		Order("login_log.created_at DESC, login_log.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list login logs: %w", err)
	}
	return items, total, nil
}
