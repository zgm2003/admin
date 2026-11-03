package transaction

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindByOrderTx(ctx context.Context, tx *gorm.DB, orderID int64) (Model, error) {
	if tx == nil || orderID < 1 {
		return Model{}, errors.New("payment transaction lookup input invalid")
	}
	var row Model
	err := tx.WithContext(ctx).Where("order_id = ?", orderID).First(&row).Error
	return row, err
}

func (r *Repository) CreateTx(ctx context.Context, tx *gorm.DB, row *Model) error {
	if r == nil || tx == nil || row == nil {
		return errors.New("payment transaction input invalid")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	return tx.WithContext(ctx).Create(row).Error
}

func transactionQuery(db *gorm.DB, query ListQuery) *gorm.DB {
	if query.Direction != 0 {
		db = db.Where("payment_transaction.direction = ?", query.Direction)
	}
	switch query.SourceType {
	case SourcePayment:
		db = db.Where("payment_transaction.order_id IS NOT NULL")
	case SourceRedeemCode:
		db = db.Where("payment_transaction.redeem_code_id IS NOT NULL")
	case SourceAI:
		db = db.Where("payment_transaction.ai_usage_charge_id IS NOT NULL")
	}
	if query.UserID > 0 {
		db = db.Where("payment_transaction.user_id = ?", query.UserID)
	}
	if query.PlatformID > 0 {
		db = db.Where("payment_transaction.platform_id = ?", query.PlatformID)
	}
	return db
}

func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("transaction database missing")
	}
	var total int64
	db := transactionQuery(r.db.WithContext(ctx).Model(&Model{}), query)
	return total, db.Count(&total).Error
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Model, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("transaction database missing")
	}
	var rows []Model
	db := transactionQuery(r.db.WithContext(ctx).Table("payment_transaction"), query)
	if query.Personal {
		db = db.Select("payment_transaction.*")
	} else {
		db = db.Select("payment_transaction.*, ua.username AS username, ap.code AS platform_code").
			Joins("LEFT JOIN user_account ua ON ua.id = payment_transaction.user_id").
			Joins("LEFT JOIN permission_auth_platform ap ON ap.id = payment_transaction.platform_id")
	}
	err := db.Order("payment_transaction.created_at DESC, payment_transaction.id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Scan(&rows).Error
	return rows, err
}

func (r *Repository) Find(ctx context.Context, id int64) (Model, error) {
	if r == nil || r.db == nil {
		return Model{}, errors.New("transaction database missing")
	}
	var row Model
	db := r.db.WithContext(ctx).Table("payment_transaction").Select("payment_transaction.*, ua.username AS username, ap.code AS platform_code").
		Joins("LEFT JOIN user_account ua ON ua.id = payment_transaction.user_id").
		Joins("LEFT JOIN permission_auth_platform ap ON ap.id = payment_transaction.platform_id")
	if err := db.Where("payment_transaction.id = ?", id).Scan(&row).Error; err != nil {
		return Model{}, err
	}
	if row.ID == 0 {
		return Model{}, gorm.ErrRecordNotFound
	}
	return row, nil
}
