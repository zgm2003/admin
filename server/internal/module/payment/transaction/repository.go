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
	if r == nil || tx == nil || row == nil || row.UserID < 1 || row.PlatformID < 1 || row.AmountUnits < 1 || (row.OrderID == nil && row.RedeemCodeID == nil) || (row.OrderID != nil && row.RedeemCodeID != nil) {
		return errors.New("payment transaction input invalid")
	}
	return tx.WithContext(ctx).Create(row).Error
}

func (r *Repository) Count(ctx context.Context, query ListQuery) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("transaction database missing")
	}
	if query.Personal {
		return r.countPersonalLedger(ctx, query)
	}
	return r.countAdminLedger(ctx, query)
}

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Model, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("transaction database missing")
	}
	if query.Personal {
		return r.listPersonalLedger(ctx, query)
	}
	return r.listAdminLedger(ctx, query)
}

func (r *Repository) countAdminLedger(ctx context.Context, query ListQuery) (int64, error) {
	payments := r.db.WithContext(ctx).Table("payment_transaction")
	charges := r.db.WithContext(ctx).Table("ai_usage_charge")
	if query.UserID > 0 {
		payments = payments.Where("user_id = ?", query.UserID)
		charges = charges.Where("user_id = ?", query.UserID)
	}
	if query.PlatformID > 0 {
		payments = payments.Where("platform_id = ?", query.PlatformID)
		charges = charges.Where("platform_id = ?", query.PlatformID)
	}
	var paymentTotal, chargeTotal int64
	if err := payments.Count(&paymentTotal).Error; err != nil {
		return 0, err
	}
	if err := charges.
		Where("status = 1 AND amount_units > 0").
		Where("balance_before_units IS NOT NULL AND balance_after_units IS NOT NULL").
		Count(&chargeTotal).Error; err != nil {
		return 0, err
	}
	return paymentTotal + chargeTotal, nil
}

func (r *Repository) listAdminLedger(ctx context.Context, query ListQuery) ([]Model, error) {
	var rows []Model
	err := r.db.WithContext(ctx).Raw(`
		SELECT ledger.*, ua.username AS username, ap.code AS platform_code
		FROM (
			SELECT
				t.id AS id,
				t.transaction_no AS transaction_no,
				t.user_id AS user_id,
				t.platform_id AS platform_id,
				t.amount_units AS amount_units,
				NULL::bigint AS balance_before_units,
				t.balance_after_units AS balance_after_units,
				t.order_id AS order_id,
				t.redeem_code_id AS redeem_code_id,
				t.created_at AS created_at,
				t.updated_at AS updated_at,
				CASE WHEN t.order_id IS NOT NULL THEN 1 ELSE 2 END AS source_type,
				NULL::bigint AS ai_usage_charge_id,
				NULL::bigint AS run_id,
				NULL::text AS agent_name,
				NULL::text AS model_name,
				0::bigint AS input_tokens,
				0::bigint AS output_tokens
			FROM payment_transaction t
			WHERE (? = 0 OR t.user_id = ?) AND (? = 0 OR t.platform_id = ?)
			UNION ALL
			SELECT
				c.id AS id,
				('AI-' || c.id) AS transaction_no,
				c.user_id AS user_id,
				c.platform_id AS platform_id,
				(-c.amount_units) AS amount_units,
				c.balance_before_units AS balance_before_units,
				c.balance_after_units AS balance_after_units,
				NULL::bigint AS order_id,
				NULL::bigint AS redeem_code_id,
				c.created_at AS created_at,
				c.created_at AS updated_at,
				3 AS source_type,
				c.id AS ai_usage_charge_id,
				c.run_id AS run_id,
				r.agent_name AS agent_name,
				r.model_name AS model_name,
				c.input_tokens AS input_tokens,
				c.output_tokens AS output_tokens
			FROM ai_usage_charge c
			JOIN ai_chat_run r ON r.id = c.run_id
			WHERE (? = 0 OR c.user_id = ?) AND (? = 0 OR c.platform_id = ?)
			  AND c.status = 1 AND c.amount_units > 0
			  AND c.balance_before_units IS NOT NULL AND c.balance_after_units IS NOT NULL
		) ledger
		LEFT JOIN user_account ua ON ua.id = ledger.user_id
		LEFT JOIN permission_auth_platform ap ON ap.id = ledger.platform_id
		ORDER BY ledger.created_at DESC, ledger.source_type DESC, ledger.id DESC
		LIMIT ? OFFSET ?
	`,
		query.UserID, query.UserID, query.PlatformID, query.PlatformID,
		query.UserID, query.UserID, query.PlatformID, query.PlatformID,
		query.PageSize, (query.Page-1)*query.PageSize,
	).Scan(&rows).Error
	return rows, err
}

func (r *Repository) countPersonalLedger(ctx context.Context, query ListQuery) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			(SELECT COUNT(*) FROM payment_transaction WHERE user_id = ? AND platform_id = ?) +
			(SELECT COUNT(*) FROM ai_usage_charge
			 WHERE user_id = ? AND platform_id = ? AND status = 1 AND amount_units > 0
			   AND balance_before_units IS NOT NULL AND balance_after_units IS NOT NULL)
	`, query.UserID, query.PlatformID, query.UserID, query.PlatformID).Scan(&total).Error
	return total, err
}

func (r *Repository) listPersonalLedger(ctx context.Context, query ListQuery) ([]Model, error) {
	var rows []Model
	err := r.db.WithContext(ctx).Raw(`
		SELECT ledger.* FROM (
			SELECT
				t.id AS id,
				t.transaction_no AS transaction_no,
				t.user_id AS user_id,
				t.platform_id AS platform_id,
				t.amount_units AS amount_units,
				NULL::bigint AS balance_before_units,
				t.balance_after_units AS balance_after_units,
				t.order_id AS order_id,
				t.redeem_code_id AS redeem_code_id,
				t.created_at AS created_at,
				t.updated_at AS updated_at,
				CASE WHEN t.order_id IS NOT NULL THEN 1 ELSE 2 END AS source_type,
				NULL::bigint AS ai_usage_charge_id,
				NULL::bigint AS run_id,
				NULL::text AS agent_name,
				NULL::text AS model_name,
				0::bigint AS input_tokens,
				0::bigint AS output_tokens
			FROM payment_transaction t
			WHERE t.user_id = ? AND t.platform_id = ?
			UNION ALL
			SELECT
				c.id AS id,
				('AI-' || c.id) AS transaction_no,
				c.user_id AS user_id,
				c.platform_id AS platform_id,
				(-c.amount_units) AS amount_units,
				c.balance_before_units AS balance_before_units,
				c.balance_after_units AS balance_after_units,
				NULL::bigint AS order_id,
				NULL::bigint AS redeem_code_id,
				c.created_at AS created_at,
				c.created_at AS updated_at,
				3 AS source_type,
				c.id AS ai_usage_charge_id,
				c.run_id AS run_id,
				r.agent_name AS agent_name,
				r.model_name AS model_name,
				c.input_tokens AS input_tokens,
				c.output_tokens AS output_tokens
			FROM ai_usage_charge c
			JOIN ai_chat_run r ON r.id = c.run_id
			WHERE c.user_id = ? AND c.platform_id = ? AND c.status = 1 AND c.amount_units > 0
			  AND c.balance_before_units IS NOT NULL AND c.balance_after_units IS NOT NULL
		) ledger
		ORDER BY ledger.created_at DESC, ledger.source_type DESC, ledger.id DESC
		LIMIT ? OFFSET ?
	`, query.UserID, query.PlatformID, query.UserID, query.PlatformID, query.PageSize, (query.Page-1)*query.PageSize).Scan(&rows).Error
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
