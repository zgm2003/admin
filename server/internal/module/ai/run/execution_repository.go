package run

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) Claim(ctx context.Context, id int64, token string, now time.Time) (Model, error) {
	var row Model
	if r == nil || r.db == nil || r.events == nil || id < 1 || token == "" || now.IsZero() {
		return row, ErrRunLeaseLost
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec(`SET LOCAL lock_timeout='1s'; SET LOCAL statement_timeout='2s'`).Error; e != nil {
			return e
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND execution_status=0", id).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRunLeaseLost
			}
			return err
		}
		// Queue expiry must be decided under the same row lock as claiming. A
		// periodic recovery sweep cannot authorize a provider request between
		// the frozen queue deadline and its next sweep.
		var policy Policy
		code := ""
		status := int16(4)
		switch {
		case row.CancelRequestedAt != nil:
			code, status = "CANCELED", 5
		case json.Unmarshal(row.PolicySnapshot, &policy) != nil || policy.Validate() != nil:
			code = "INVALID_POLICY"
		case !row.AcceptedAt.Add(time.Duration(policy.MaxQueuedSeconds) * time.Second).After(now):
			code = "QUEUE_TIMEOUT"
		case !row.DeadlineAt.After(now):
			code = "RUN_TIMEOUT"
		}
		if code != "" {
			if err := r.finishTextTx(ctx, tx, row, status, code, "", now); err != nil {
				return err
			}
			// Return the committed terminal row, not an error that would roll back
			// the known-zero charge and leave this Run eligible for later I/O.
			return tx.First(&row, id).Error
		}
		res := tx.Raw(`UPDATE ai_run SET execution_status=1,state_version=state_version+1,lease_token=?::uuid,lease_until=?,heartbeat_at=?,started_at=?,updated_at=? WHERE id=? AND execution_status=0 AND cancel_requested_at IS NULL AND deadline_at>? RETURNING *`, token, now.Add(45*time.Second), now, now, now, id, now).Scan(&row)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrRunLeaseLost
		}
		_, e := r.events.AppendTx(ctx, tx, id, "started", json.RawMessage(`{}`), now)
		return e
	})
	return row, err
}
func (r *Repository) Renew(ctx context.Context, id int64, token string, now time.Time) error {
	if r == nil || r.db == nil || id < 1 || token == "" {
		return ErrRunLeaseLost
	}
	result := r.db.WithContext(ctx).Exec(`UPDATE ai_run SET lease_until=?,heartbeat_at=?,updated_at=? WHERE id=? AND lease_token=?::uuid AND execution_status=1 AND lease_until>? AND cancel_requested_at IS NULL`, now.Add(45*time.Second), now, now, id, token, now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRunLeaseLost
	}
	return nil
}
func (r *Repository) Fence(ctx context.Context, id int64, token string, now time.Time) error {
	var count int64
	if r == nil || r.db == nil {
		return ErrRunLeaseLost
	}
	err := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM ai_run WHERE id=? AND lease_token=?::uuid AND execution_status=1 AND lease_until>? AND cancel_requested_at IS NULL AND deadline_at>?`, id, token, now, now).Scan(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrRunLeaseLost
	}
	return nil
}
func (r *Repository) Cancel(ctx context.Context, id, userID, platformID int64, now time.Time) error {
	if r == nil || r.db == nil || id < 1 || userID < 1 || platformID < 1 {
		return errors.New("AI cancel request invalid")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND user_id=? AND platform_id=?", id, userID, platformID).Take(&row).Error; err != nil {
			return err
		}
		if row.ExecutionStatus >= 3 {
			return nil
		}
		if err := tx.Model(&Model{}).Where("id=?", row.ID).Updates(map[string]any{"cancel_requested_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		row.CancelRequestedAt = &now
		if row.ExecutionStatus == 0 {
			return r.finishTextTx(ctx, tx, row, 5, "CANCELED", "", now)
		}
		return nil
	})
}
