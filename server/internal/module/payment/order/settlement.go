package order

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	paymentmodule "admin/server/internal/module/payment"
	transactionmodule "admin/server/internal/module/payment/transaction"
	walletmodule "admin/server/internal/module/payment/wallet"
	"admin/server/internal/shared/money"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type callbackEvidence struct {
	ID            int64  `gorm:"column:id"`
	ConfigID      int64  `gorm:"column:config_id"`
	ConfigVersion int64  `gorm:"column:config_version"`
	OrderID       *int64 `gorm:"column:order_id"`
	ProcessStatus int16  `gorm:"column:process_status"`
	Payload       []byte `gorm:"column:payload"`
}

func matchCallbackEvidence(row callbackEvidence, payment ConfirmedPayment) error {
	// Callback status 1 is pending and 2 is already committed. The order
	// repository reads this projection under a lock; other outcomes cannot credit.
	if row.ID < 1 || row.ConfigID != payment.ConfigID || row.ConfigVersion != payment.ConfigVersion || (row.ProcessStatus != 1 && row.ProcessStatus != 2) {
		return ErrCallbackMismatch
	}
	var fields struct {
		NotifyID    *string    `json:"notifyId"`
		OrderNo     string     `json:"orderNo"`
		TradeNo     *string    `json:"tradeNo"`
		TradeStatus string     `json:"tradeStatus"`
		AppID       string     `json:"appId"`
		SellerID    *string    `json:"sellerId"`
		TotalAmount string     `json:"totalAmount"`
		PaidAt      *time.Time `json:"paidAt"`
	}
	if err := json.Unmarshal(row.Payload, &fields); err != nil || fields.OrderNo != payment.OrderNo || fields.TradeNo == nil || *fields.TradeNo != payment.TradeNo || fields.TradeStatus != payment.Status || fields.AppID != payment.AppID || fields.SellerID == nil || payment.SellerID == nil || *fields.SellerID != *payment.SellerID {
		return ErrCallbackMismatch
	}
	amount, err := money.ParseRMBCents(fields.TotalAmount)
	if err != nil || amount != payment.AmountCents || (fields.PaidAt == nil) != (payment.PaidAt == nil) || (fields.PaidAt != nil && !fields.PaidAt.Equal(*payment.PaidAt)) {
		return ErrCallbackMismatch
	}
	return nil
}

var (
	ErrOrderNotFound       = errors.New("payment order not found")
	ErrOrderMismatch       = errors.New("payment confirmation does not match order")
	ErrConfigMismatch      = errors.New("payment confirmation config or environment mismatch")
	ErrIdentityMismatch    = errors.New("payment confirmation merchant identity mismatch")
	ErrAmountMismatch      = errors.New("payment confirmation amount mismatch")
	ErrCallbackMismatch    = errors.New("payment confirmation callback facts mismatch")
	ErrTradeConflict       = errors.New("provider trade number belongs to another order")
	ErrBalanceOverflow     = errors.New("payment wallet balance overflow")
	ErrSettlementInvariant = errors.New("payment settlement invariant failed")
)

type Repository struct {
	db           *gorm.DB
	wallet       *walletmodule.Repository
	transactions *transactionmodule.Repository
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, wallet: walletmodule.NewRepository(db), transactions: transactionmodule.NewRepository(db)}
}

func (r *Repository) Confirm(ctx context.Context, payment ConfirmedPayment, callbackEventID *int64, now time.Time) (SettlementResult, error) {
	if r == nil || r.db == nil {
		return SettlementResult{}, errors.New("payment settlement database missing")
	}
	if err := validateConfirmation(payment, callbackEventID); err != nil {
		return SettlementResult{}, err
	}
	var result SettlementResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var callback callbackEvidence
		if callbackEventID != nil {
			if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Table("payment_callback_event").Select("id,config_id,config_version,order_id,process_status,payload").Where("id = ?", *callbackEventID).Take(&callback).Error; err != nil {
				return err
			}
			if callback.ID != *callbackEventID {
				return ErrCallbackMismatch
			}
			if err := matchCallbackEvidence(callback, payment); err != nil {
				return err
			}
		}
		var order Model
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", payment.OrderNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOrderNotFound
			}
			return err
		}
		if order.ConfigID != payment.ConfigID || order.ConfigVersion != payment.ConfigVersion {
			return ErrConfigMismatch
		}
		if callbackEventID != nil && callback.OrderID != nil && *callback.OrderID != order.ID {
			return ErrCallbackMismatch
		}
		if order.AmountCents != payment.AmountCents {
			return ErrAmountMismatch
		}
		if order.OrderNo != payment.OrderNo || order.Status < StatusPending || order.Status > StatusClosed {
			return ErrOrderMismatch
		}
		var channel struct {
			AppID       string                    `gorm:"column:app_id"`
			SellerID    string                    `gorm:"column:seller_id"`
			Environment paymentmodule.Environment `gorm:"column:environment"`
		}
		if err := tx.WithContext(ctx).Table("payment_config").Select("app_id,seller_id,environment").Where("id = ?", order.ConfigID).Take(&channel).Error; err != nil {
			return err
		}
		if channel.Environment != payment.Environment {
			return ErrConfigMismatch
		}
		if channel.AppID != payment.AppID || (payment.SellerID != nil && *payment.SellerID != channel.SellerID) || (payment.ProofSource == ProofSourceCallback && (payment.SellerID == nil || *payment.SellerID != channel.SellerID)) {
			return ErrIdentityMismatch
		}
		var credentialCount int64
		if err := tx.WithContext(ctx).Table("payment_config_version").Where("config_id = ? AND version = ?", order.ConfigID, order.ConfigVersion).Count(&credentialCount).Error; err != nil {
			return err
		}
		if credentialCount != 1 {
			return ErrConfigMismatch
		}
		units, err := money.CentsToUnits(payment.AmountCents)
		if err != nil || units <= 0 {
			return ErrAmountMismatch
		}
		if order.ProviderTradeNo != nil && *order.ProviderTradeNo != payment.TradeNo {
			return ErrTradeConflict
		}
		var tradeOwnerCount int64
		if err := tx.WithContext(ctx).Table("payment_order").Where("config_id = ? AND provider_trade_no = ? AND id <> ?", order.ConfigID, payment.TradeNo, order.ID).Count(&tradeOwnerCount).Error; err != nil {
			return err
		}
		if tradeOwnerCount != 0 {
			return ErrTradeConflict
		}

		existing, findErr := r.transactions.FindByOrderTx(ctx, tx, order.ID)
		if findErr == nil {
			if existing.UserID != order.UserID || existing.PlatformID != order.PlatformID || existing.AmountUnits != units || existing.OrderID == nil || *existing.OrderID != order.ID {
				return ErrSettlementInvariant
			}
			if order.Status != StatusPaid || order.CreditedAt == nil {
				return ErrSettlementInvariant
			}
			result = SettlementResult{OrderID: order.ID, TransactionID: existing.ID, BalanceUnits: existing.BalanceAfterUnits, Replayed: true}
			return markCallbackProcessed(ctx, tx, callbackEventID, order.ID, now)
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if order.Status == StatusPaid {
			return ErrSettlementInvariant
		}

		wallet, _, err := r.wallet.CreditTx(ctx, tx, order.UserID, units, now)
		if err != nil {
			if errors.Is(err, walletmodule.ErrBalanceOverflow) {
				return ErrBalanceOverflow
			}
			return err
		}
		orderID := order.ID
		transaction := &transactionmodule.Model{UserID: order.UserID, PlatformID: order.PlatformID, AmountUnits: units, BalanceAfterUnits: wallet.BalanceUnits, OrderID: &orderID, CreatedAt: now, UpdatedAt: now}
		transaction.TransactionNo, err = transactionmodule.NewTransactionNo()
		if err != nil {
			return err
		}
		if err := r.transactions.CreateTx(ctx, tx, transaction); err != nil {
			return err
		}
		updates := map[string]any{"status": StatusPaid, "provider_trade_no": payment.TradeNo, "paid_at": payment.PaidAt, "credited_at": now, "next_check_at": nil, "check_token": nil, "check_lease_until": nil, "updated_at": now}
		if err := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := markCallbackProcessed(ctx, tx, callbackEventID, order.ID, now); err != nil {
			return err
		}
		result = SettlementResult{OrderID: order.ID, TransactionID: transaction.ID, BalanceUnits: wallet.BalanceUnits}
		return nil
	})
	return result, err
}

func markCallbackProcessed(ctx context.Context, tx *gorm.DB, callbackEventID *int64, orderID int64, now time.Time) error {
	if callbackEventID == nil {
		return nil
	}
	result := tx.WithContext(ctx).Table("payment_callback_event").Where("id = ? AND process_status IN (1,2) AND (order_id IS NULL OR order_id = ?)", *callbackEventID, orderID).Updates(map[string]any{"order_id": orderID, "process_status": 2, "error_code": nil, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSettlementInvariant
	}
	return nil
}
