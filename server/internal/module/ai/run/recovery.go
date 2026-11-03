package run

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Recovery acquires the Run before its request/tool facts, matching execution's
// lock order. It never resubmits a provider request or a side-effecting tool.
func (r *Repository) recoverDeadRuns(ctx context.Context, now time.Time, limit int) (int, error) {
	if r == nil || r.db == nil || r.events == nil || r.scheduler == nil || now.IsZero() || limit < 1 || limit > 100 {
		return 0, ErrRunLeaseLost
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	count := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SET LOCAL lock_timeout='1s';SET LOCAL statement_timeout='2s'`).Error; err != nil {
			return err
		}
		var rows []Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("execution_status IN(1,2) AND (lease_until<? OR (lease_until IS NULL AND deadline_at<?))", now, now).Order("id").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := tx.Exec(`UPDATE ai_provider_request SET status=4,usage_status=2,error_code='UNKNOWN_SUBMISSION',error_message='供应商提交结果未知，等待核对',finished_at=?,updated_at=? WHERE run_id=? AND status=1`, now, now, row.ID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`UPDATE ai_tool_call SET status=4,error_code='UNKNOWN_TOOL_OUTCOME',error_message='工具执行结果未知，等待核对',finished_at=?,updated_at=? WHERE run_id=? AND status=1`, now, now, row.ID).Error; err != nil {
				return err
			}
			var err error
			if row.Operation == "text.generate" {
				err = r.finishTextTx(ctx, tx, row, 4, "WORKER_INTERRUPTED", "", now)
			} else {
				err = r.finishMediaTx(ctx, tx, row, 4, "WORKER_INTERRUPTED", now)
			}
			if err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}
