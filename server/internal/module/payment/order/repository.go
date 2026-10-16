package order

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

func (r *Repository) FindByRequest(ctx context.Context, userID, platformID int64, requestID string) (Model, error) {
	if r == nil || r.db == nil || userID < 1 || platformID < 1 || requestID == "" {
		return Model{}, errors.New("payment order request lookup input invalid")
	}
	var row Model
	err := r.db.WithContext(ctx).Where("user_id = ? AND platform_id = ? AND request_id = ?", userID, platformID, requestID).First(&row).Error
	return row, err
}

func (r *Repository) Create(ctx context.Context, row *Model) error {
	if r == nil || r.db == nil || row == nil || row.UserID < 1 || row.PlatformID < 1 || row.PackageID < 1 || row.ConfigID < 1 || row.ConfigVersion < 1 || row.AmountCents < 1 || row.OrderNo == "" || row.RequestID == "" || len(row.RequestFingerprint) != 32 || row.Status != StatusPending || row.ExpiresAt.IsZero() || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return errors.New("payment order create input invalid")
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	if row.ID < 1 {
		return errors.New("payment order id missing after create")
	}
	return nil
}

func (r *Repository) Find(ctx context.Context, id int64) (Model, error) {
	if r == nil || r.db == nil || id < 1 {
		return Model{}, errors.New("payment order lookup input invalid")
	}
	var row Model
	err := r.db.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Model{}, ErrOrderNotFound
	}
	return row, err
}

func orderQuery(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.UserID > 0 {
		db = db.Where("payment_order.user_id = ?", query.UserID)
	}
	if query.PlatformID > 0 {
		db = db.Where("payment_order.platform_id = ?", query.PlatformID)
	}
	return db
}

func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("payment order database missing")
	}
	var total int64
	err := orderQuery(r.db.WithContext(ctx).Model(&Model{}), query).Count(&total).Error
	return total, err
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Model, error) {
	if r == nil || r.db == nil || query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 {
		return nil, errors.New("payment order pagination invalid")
	}
	rows := make([]Model, 0)
	db := orderQuery(r.db.WithContext(ctx).Table("payment_order"), query)
	if !query.Personal {
		db = db.Select("payment_order.*, ua.username AS username, ap.code AS platform_code").
			Joins("LEFT JOIN user_account ua ON ua.id = payment_order.user_id").
			Joins("LEFT JOIN permission_auth_platform ap ON ap.id = payment_order.platform_id")
	}
	err := db.Order("payment_order.created_at DESC, payment_order.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error
	return rows, err
}
