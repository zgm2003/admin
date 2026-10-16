package order

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]ClaimedOrder, error) {
	if ctx == nil || r == nil || r.db == nil || now.IsZero() || limit < 1 || limit > reconcileBatchSize || lease <= 0 || lease > reconcileLease {
		return nil, errors.New("payment reconcile claim input invalid")
	}
	leaseUntil := now.UTC().Add(lease)
	rows := make([]Model, 0, limit)
	claimed := make([]ClaimedOrder, 0, limit)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Separate expiry and ordinary scans let PostgreSQL use the two pending
		// partial indexes. A NULL schedule from older orders is due after 60s.
		base := func() *gorm.DB {
			return tx.WithContext(ctx).Model(&Model{}).
				Select("id,order_no,config_id,config_version,amount_cents,status,expires_at,created_at,next_check_at").
				Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
				Where("status = ? AND (next_check_at <= ? OR (next_check_at IS NULL AND created_at <= ?)) AND (check_lease_until IS NULL OR check_lease_until <= ?)", StatusPending, now.UTC(), now.UTC().Add(-time.Minute), now.UTC())
		}
		if err := base().Where("expires_at <= ?", now.UTC()).Order("expires_at ASC, id ASC").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) < limit {
			var ordinary []Model
			if err := base().Where("expires_at > ?", now.UTC()).Order("next_check_at ASC NULLS FIRST, id ASC").Limit(limit - len(rows)).Find(&ordinary).Error; err != nil {
				return err
			}
			rows = append(rows, ordinary...)
		}
		for _, row := range rows {
			token, err := uuid.NewRandom()
			if err != nil {
				return err
			}
			value := token.String()
			if err := tx.WithContext(ctx).Model(&Model{}).Where("id = ?", row.ID).Updates(map[string]any{"check_token": value, "check_lease_until": leaseUntil, "updated_at": now.UTC()}).Error; err != nil {
				return err
			}
			row.CheckToken, row.CheckLeaseUntil = &value, &leaseUntil
			claimed = append(claimed, ClaimedOrder{Model: row, Token: value})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func (r *Repository) RenewCheck(ctx context.Context, id int64, token string, now time.Time, lease time.Duration) error {
	if ctx == nil || r == nil || r.db == nil || id < 1 || !validCheckToken(token) || now.IsZero() || lease <= 0 || lease > reconcileLease {
		return errors.New("payment reconcile renewal input invalid")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND status = ? AND check_token = ? AND check_lease_until > ?", id, StatusPending, token, now.UTC()).Updates(map[string]any{"check_lease_until": now.UTC().Add(lease), "updated_at": now.UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrReconcileLeaseLost
	}
	return nil
}

func (r *Repository) AdvanceCheck(ctx context.Context, id int64, token string, next, now time.Time) error {
	if ctx == nil || r == nil || r.db == nil || id < 1 || !validCheckToken(token) || next.IsZero() || now.IsZero() || !next.After(now) {
		return errors.New("payment reconcile advance input invalid")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND status = ? AND check_token = ? AND check_lease_until > ?", id, StatusPending, token, now.UTC()).Updates(map[string]any{"next_check_at": next.UTC(), "check_token": nil, "check_lease_until": nil, "updated_at": now.UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrReconcileLeaseLost
	}
	return nil
}

func (r *Repository) CloseClaimed(ctx context.Context, id int64, token string, now time.Time) error {
	if ctx == nil || r == nil || r.db == nil || id < 1 || !validCheckToken(token) || now.IsZero() {
		return errors.New("payment reconcile close input invalid")
	}
	result := r.db.WithContext(ctx).Model(&Model{}).Where("id = ? AND status = ? AND check_token = ? AND check_lease_until > ?", id, StatusPending, token, now.UTC()).Updates(map[string]any{"status": StatusClosed, "closed_at": now.UTC(), "next_check_at": nil, "check_token": nil, "check_lease_until": nil, "updated_at": now.UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrReconcileLeaseLost
	}
	return nil
}

var ErrReconcileLeaseLost = errors.New("payment reconcile lease lost")

func validCheckToken(token string) bool {
	value, err := uuid.Parse(token)
	return err == nil && value != uuid.Nil && value.String() == token
}
