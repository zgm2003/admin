package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Find(ctx context.Context, userID int64) (Model, bool, error) {
	var row Model
	if r == nil || r.db == nil {
		return row, false, errors.New("wallet database missing")
	}
	result := r.db.WithContext(ctx).First(&row, "user_id = ?", userID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return Model{}, false, nil
	}
	return row, result.Error == nil, result.Error
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("wallet database missing")
	}
	var total int64
	err := r.db.WithContext(ctx).Model(&Model{}).Count(&total).Error
	return total, err
}

func (r *Repository) FindAdmin(ctx context.Context, userID int64) (Model, bool, error) {
	if r == nil || r.db == nil {
		return Model{}, false, errors.New("wallet database missing")
	}
	var row Model
	err := r.db.WithContext(ctx).Table("payment_wallet").Select("payment_wallet.*, ua.username AS username").
		Joins("LEFT JOIN user_account ua ON ua.id = payment_wallet.user_id").
		Where("payment_wallet.user_id = ?", userID).Scan(&row).Error
	return row, row.UserID != 0 && err == nil, err
}

func (r *Repository) List(ctx context.Context, offset, limit int) ([]Model, error) {
	if r == nil || r.db == nil || offset < 0 || limit < 1 || limit > 100 {
		return nil, fmt.Errorf("wallet pagination invalid")
	}
	rows := make([]Model, 0)
	err := r.db.WithContext(ctx).Table("payment_wallet").Select("payment_wallet.*, ua.username AS username").
		Joins("LEFT JOIN user_account ua ON ua.id = payment_wallet.user_id").
		Order("payment_wallet.updated_at DESC, payment_wallet.user_id DESC").Offset(offset).Limit(limit).Scan(&rows).Error
	return rows, err
}

func (r *Repository) LockOrCreate(ctx context.Context, tx *gorm.DB, userID int64, now time.Time) (Model, error) {
	if tx == nil || userID < 1 {
		return Model{}, errors.New("wallet transaction is invalid")
	}
	row := Model{UserID: userID, CreatedAt: now, UpdatedAt: now}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return Model{}, err
	}
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "user_id = ?", userID).Error; err != nil {
		return Model{}, err
	}
	return row, nil
}

func (r *Repository) CreditTx(ctx context.Context, tx *gorm.DB, userID, amountUnits int64, now time.Time) (Model, int64, error) {
	if r == nil || tx == nil || userID < 1 || amountUnits < 1 {
		return Model{}, 0, errors.New("wallet credit input invalid")
	}
	row, err := r.LockOrCreate(ctx, tx, userID, now)
	if err != nil {
		return Model{}, 0, err
	}
	before := row.BalanceUnits
	after, err := checkedAdd(before, amountUnits)
	if err != nil {
		return Model{}, 0, err
	}
	result := tx.WithContext(ctx).Model(&Model{}).Where("user_id = ?", userID).Updates(map[string]any{"balance_units": after, "updated_at": now})
	if result.Error != nil {
		return Model{}, 0, result.Error
	}
	if result.RowsAffected != 1 {
		return Model{}, 0, gorm.ErrRecordNotFound
	}
	row.BalanceUnits, row.UpdatedAt = after, now
	return row, before, nil
}
