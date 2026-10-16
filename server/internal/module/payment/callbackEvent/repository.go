package callbackevent

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindByDedupe(ctx context.Context, configID, version int64, key []byte) (Model, error) {
	if r == nil || r.db == nil || configID < 1 || version < 1 || len(key) != 32 {
		return Model{}, errors.New("callback event lookup input invalid")
	}
	var row Model
	err := r.db.WithContext(ctx).Where("config_id = ? AND config_version = ? AND dedupe_key = ?", configID, version, key).First(&row).Error
	return row, err
}

func (r *Repository) Create(ctx context.Context, row *Model) error {
	if r == nil || r.db == nil || row == nil || row.ConfigID < 1 || row.ConfigVersion < 1 || len(row.DedupeKey) != 32 || len(row.Payload) == 0 || !row.ProcessStatus.Valid() || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return errors.New("callback event create input invalid")
	}
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *Repository) UpdateResult(ctx context.Context, id int64, status ProcessStatus, code *ErrorCode, now time.Time) error {
	if r == nil || r.db == nil || id < 1 || !status.Valid() || status == ProcessSuccess || (code != nil && !code.Valid()) || now.IsZero() {
		return errors.New("callback event update input invalid")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND process_status = ?", id, ProcessPending).Updates(map[string]any{"process_status": status, "error_code": code, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) FindOrderID(ctx context.Context, orderNo string) (*int64, error) {
	if r == nil || r.db == nil || !validFact(orderNo, 32) {
		return nil, errors.New("callback order lookup input invalid")
	}
	var row struct{ ID int64 }
	err := r.db.WithContext(ctx).Table("payment_order").Select("id").Where("order_no = ?", orderNo).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.ID < 1 {
		return nil, errors.New("stored callback order id invalid")
	}
	return &row.ID, nil
}

func (r *Repository) Find(ctx context.Context, id int64) (Model, error) {
	if r == nil || r.db == nil || id < 1 {
		return Model{}, errors.New("callback event lookup input invalid")
	}
	var row Model
	err := r.db.WithContext(ctx).First(&row, id).Error
	return row, err
}

func (r *Repository) CountByOrder(ctx context.Context, orderID int64) (int64, error) {
	if r == nil || r.db == nil || orderID < 1 {
		return 0, errors.New("callback event order lookup input invalid")
	}
	var total int64
	err := r.db.WithContext(ctx).Model(&Model{}).Where("order_id = ?", orderID).Count(&total).Error
	return total, err
}

func (r *Repository) ListByOrder(ctx context.Context, orderID int64, offset, limit int) ([]Model, error) {
	if r == nil || r.db == nil || orderID < 1 || offset < 0 || limit < 1 || limit > 100 {
		return nil, errors.New("callback event order pagination invalid")
	}
	rows := make([]Model, 0)
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, err
}
