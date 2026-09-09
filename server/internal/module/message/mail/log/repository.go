package log

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type ProviderResult struct {
	RequestID string
	MessageID string
}

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreatePending(ctx context.Context, value *Model) (Model, error) {
	err := r.db.WithContext(ctx).Create(value).Error
	return *value, err
}

func (r *Repository) FindActiveChallenge(ctx context.Context, platformID int64, challenge string) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).Where("platform_id = ? AND challenge_id = ? AND deleted_at IS NULL", platformID, challenge).Take(&value).Error
	return value, err
}

func (r *Repository) MarkSent(ctx context.Context, platformID, id int64, result ProviderResult, latencyMs int64) error {
	now := time.Now().UTC()
	query := r.db.WithContext(ctx).Model(&Model{}).
		Where("id = ? AND platform_id = ? AND status = ? AND deleted_at IS NULL", id, platformID, "pending").
		Updates(map[string]any{
			"status": "sent", "request_id": result.RequestID, "message_id": result.MessageID,
			"latency_ms": latencyMs, "sent_at": now, "updated_at": now,
		})
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, platformID, id int64, errorCode, errorSummary string, latencyMs int64) error {
	query := r.db.WithContext(ctx).Model(&Model{}).
		Where("id = ? AND platform_id = ? AND status = ? AND deleted_at IS NULL", id, platformID, "pending").
		Updates(map[string]any{
			"status": "failed", "error_code": errorCode, "error_summary": errorSummary,
			"latency_ms": latencyMs, "updated_at": time.Now().UTC(),
		})
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context, platformID int64, page, size int) ([]Model, int64, error) {
	var values []Model
	var total int64
	query := r.db.WithContext(ctx).Where("platform_id = ? AND deleted_at IS NULL", platformID)
	if err := query.Model(&Model{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *Repository) Find(ctx context.Context, platformID, id int64) (Model, error) {
	var value Model
	err := r.db.WithContext(ctx).Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).Take(&value).Error
	return value, err
}

func (r *Repository) Delete(ctx context.Context, platformID, id int64) error {
	query := r.db.WithContext(ctx).Model(&Model{}).
		Where("platform_id = ? AND id = ? AND deleted_at IS NULL", platformID, id).
		Update("deleted_at", time.Now().UTC())
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) DeleteMany(ctx context.Context, platformID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&Model{}).
		Where("platform_id = ? AND id IN ? AND deleted_at IS NULL", platformID, ids).
		Update("deleted_at", time.Now().UTC()).Error
}
