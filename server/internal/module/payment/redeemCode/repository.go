package redeemcode

import (
	"context"
	"crypto/hmac"
	"errors"
	"fmt"
	"sort"
	"time"

	"admin/server/internal/module/payment/transaction"
	"admin/server/internal/module/payment/wallet"
	"admin/server/internal/shared/money"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db           *gorm.DB
	wallet       *wallet.Repository
	transactions *transaction.Repository
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, wallet: wallet.NewRepository(db), transactions: transaction.NewRepository(db)}
}
func (r *Repository) available() error {
	if r == nil || r.db == nil {
		return errors.New("redeem database missing")
	}
	return nil
}

// The unique actor/request coordinate is the cross-instance generation mutex.
// A losing INSERT waits for its winner, then reads and compares frozen facts.
func (r *Repository) CreateBatch(ctx context.Context, batch Batch, codes []Model, now time.Time) (Batch, []Model, error) {
	var out Batch
	var rows []Model
	if e := r.available(); e != nil {
		return out, nil, e
	}
	if batch.CreatedBy < 1 || batch.Quantity < 1 || batch.Quantity > 1000 || len(codes) != batch.Quantity || len(batch.RequestFingerprint) != 32 {
		return out, nil, ErrInvalidInput
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		candidate := batch
		inserted := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "created_by"}, {Name: "request_id"}}, DoNothing: true}).Create(&candidate)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 0 {
			if e := tx.Where("created_by = ? AND request_id = ?", batch.CreatedBy, batch.RequestID).First(&out).Error; e != nil {
				return e
			}
			if !hmac.Equal(out.RequestFingerprint, batch.RequestFingerprint) {
				return ErrRequestConflict
			}
		} else {
			if batch.ExpiresAt != nil && !batch.ExpiresAt.After(now) {
				return ErrInvalidInput
			}
			out = candidate
			for i := range codes {
				codes[i].BatchID = out.ID
			}
			if e := tx.CreateInBatches(&codes, 250).Error; e != nil {
				return e
			}
		}
		if e := tx.Where("batch_id = ?", out.ID).Order("id ASC").Find(&rows).Error; e != nil {
			return e
		}
		if len(rows) != out.Quantity {
			return ErrInvariant
		}
		return nil
	})
	return out, rows, err
}

func redeemQuery(db *gorm.DB, q ListQuery, now time.Time) *gorm.DB {
	db = db.Table("payment_redeem_code c").Joins("JOIN payment_redeem_code_batch b ON b.id=c.batch_id")
	if q.BatchID > 0 {
		db = db.Where("c.batch_id = ?", q.BatchID)
	}
	if q.Status != nil {
		switch *q.Status {
		case StatusUnused:
			db = db.Where("c.status = ? AND (b.expires_at IS NULL OR b.expires_at > ?)", StatusUnused, now)
		case StatusExpired:
			db = db.Where("c.status = ? AND b.expires_at IS NOT NULL AND b.expires_at <= ?", StatusUnused, now)
		default:
			db = db.Where("c.status = ?", *q.Status)
		}
	}
	return db
}
func (r *Repository) List(ctx context.Context, q ListQuery, now time.Time) ([]Model, int64, error) {
	if e := r.available(); e != nil {
		return nil, 0, e
	}
	var total int64
	if e := redeemQuery(r.db.WithContext(ctx), q, now).Count(&total).Error; e != nil {
		return nil, 0, e
	}
	rows := make([]Model, 0)
	e := redeemQuery(r.db.WithContext(ctx), q, now).Select("c.*,b.batch_no,b.amount_cents,b.expires_at").Order("c.created_at DESC,c.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error
	return rows, total, e
}
func (r *Repository) Get(ctx context.Context, id int64) (Model, error) {
	return r.get(ctx, "c.id = ?", id)
}
func (r *Repository) Lookup(ctx context.Context, hash []byte) (Model, error) {
	return r.get(ctx, "c.code_hash = ?", hash)
}
func (r *Repository) get(ctx context.Context, where string, value any) (Model, error) {
	var row Model
	if e := r.available(); e != nil {
		return row, e
	}
	err := r.db.WithContext(ctx).Table("payment_redeem_code c").Joins("JOIN payment_redeem_code_batch b ON b.id=c.batch_id").Select("c.*,b.batch_no,b.amount_cents,b.expires_at").Where(where, value).Take(&row).Error
	return row, err
}
func (r *Repository) Export(ctx context.Context, ids []int64) ([]Model, error) {
	if e := r.available(); e != nil {
		return nil, e
	}
	rows := make([]Model, 0)
	e := r.db.WithContext(ctx).Table("payment_redeem_code c").Joins("JOIN payment_redeem_code_batch b ON b.id=c.batch_id").Select("c.*,b.batch_no,b.amount_cents,b.expires_at").Where("c.id IN ?", ids).Order("c.id ASC").Scan(&rows).Error
	if e == nil && len(rows) != len(ids) {
		return nil, gorm.ErrRecordNotFound
	}
	return rows, e
}

// Ordered locks make overlapping bulk void requests deadlock-resistant and
// all-or-nothing. A used code never silently turns into a successful void.
func (r *Repository) Void(ctx context.Context, ids []int64, now time.Time) error {
	if e := r.available(); e != nil {
		return e
	}
	ordered := append([]int64(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []Model
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ordered).Order("id ASC").Find(&rows).Error; e != nil {
			return e
		}
		if len(rows) != len(ordered) {
			return gorm.ErrRecordNotFound
		}
		for _, row := range rows {
			if row.Status == StatusUsed {
				return ErrCodeUnavailable
			}
			if row.Status != StatusUnused && row.Status != StatusVoided {
				return ErrInvariant
			}
		}
		return tx.Model(&Model{}).Where("id IN ? AND status = ?", ordered, StatusUnused).Updates(map[string]any{"status": StatusVoided, "updated_at": now}).Error
	})
}

// Lock order is redeem code -> wallet. Amount and expiry come only from its
// immutable batch inside this transaction, never from a prior lookup/client.
func (r *Repository) Redeem(ctx context.Context, userID, platformID int64, hash []byte, now time.Time) (transaction.Model, error) {
	var out transaction.Model
	if e := r.available(); e != nil {
		return out, e
	}
	if userID < 1 || platformID < 1 || len(hash) != 32 {
		return out, ErrInvalidInput
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Model
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code_hash = ?", hash).First(&row).Error; e != nil {
			return e
		}
		var batch Batch
		if e := tx.First(&batch, row.BatchID).Error; e != nil {
			return e
		}
		units, e := money.CentsToUnits(batch.AmountCents)
		if e != nil || units < 1 {
			return ErrInvariant
		}
		if row.Status == StatusUsed {
			if row.UsedBy == nil || *row.UsedBy != userID || row.UsedAt == nil {
				return ErrCodeUnavailable
			}
			if e := tx.Where("redeem_code_id = ?", row.ID).First(&out).Error; e != nil {
				return e
			}
			if out.RedeemCodeID == nil || *out.RedeemCodeID != row.ID || out.OrderID != nil || out.UserID != userID || out.AmountUnits != units || out.PlatformID < 1 {
				return ErrInvariant
			}
			return nil
		}
		if row.Status != StatusUnused {
			return ErrCodeUnavailable
		}
		if row.UsedBy != nil || row.UsedAt != nil {
			return ErrInvariant
		}
		if batch.ExpiresAt != nil && !batch.ExpiresAt.After(now) {
			return ErrCodeExpired
		}
		w, _, e := r.wallet.CreditTx(ctx, tx, userID, units, now)
		if e != nil {
			return e
		}
		rid := row.ID
		out = transaction.Model{UserID: userID, PlatformID: platformID, AmountUnits: units, BalanceAfterUnits: w.BalanceUnits, RedeemCodeID: &rid, CreatedAt: now, UpdatedAt: now}
		out.TransactionNo, e = transaction.NewTransactionNo()
		if e != nil {
			return e
		}
		if e = r.transactions.CreateTx(ctx, tx, &out); e != nil {
			return e
		}
		changed := tx.Model(&Model{}).Where("id = ? AND status = ?", row.ID, StatusUnused).Updates(map[string]any{"status": StatusUsed, "used_by": userID, "used_at": now, "updated_at": now})
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return fmt.Errorf("%w: code state changed", ErrInvariant)
		}
		return nil
	})
	return out, err
}
