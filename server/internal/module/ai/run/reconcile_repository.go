package run

import (
	"admin/server/internal/module/ai"
	pr "admin/server/internal/module/ai/providerRequest"
	"admin/server/internal/module/ai/run/event"
	"admin/server/internal/module/ai/usage/charge"
	"admin/server/internal/module/system/scheduler"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type ReconcileRepository struct {
	db     *gorm.DB
	jobs   *scheduler.Repository
	events *event.Repository
}

func NewReconcileRepository(db *gorm.DB, jobs *scheduler.Repository, events *event.Repository) *ReconcileRepository {
	return &ReconcileRepository{db, jobs, events}
}
func (r *ReconcileRepository) Review(ctx context.Context, id int64, apply bool, review func(ReconcileState) (ReconcilePlan, error)) (ReconcilePlan, error) {
	var out ReconcilePlan
	if r == nil || r.db == nil || review == nil || id < 1 || apply && (r.jobs == nil || r.events == nil) {
		return out, ErrReconcileEvidence
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	options := &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: !apply}
	e := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec(`SET LOCAL lock_timeout='1s'; SET LOCAL statement_timeout='2s'`).Error; e != nil {
			return e
		}
		lock := func(db *gorm.DB) *gorm.DB {
			if apply {
				return db.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			return db
		}
		var ch charge.Model
		if e := lock(tx).Where("run_id=?", id).First(&ch).Error; e != nil {
			return e
		}
		var row Model
		if e := lock(tx).Where("id=?", id).First(&row).Error; e != nil {
			return e
		}
		var policy Policy
		if json.Unmarshal(row.PolicySnapshot, &policy) != nil || policy.Validate() != nil {
			return ErrInvalidSnapshot
		}
		requests := make([]pr.Fact, 0)
		if e := lock(tx).Where("run_id=?", id).Order("sequence").Limit(int(ai.MaxModelRounds) + 1).Find(&requests).Error; e != nil {
			return e
		}
		var toolIDs []int64
		if e := tx.Table("ai_tool_call").Select("id").Where("run_id=? AND status IN(1,4)", id).Limit(1).Scan(&toolIDs).Error; e != nil {
			return e
		}
		var itemIDs []int64
		if e := tx.Table("ai_usage_charge_item").Select("id").Where("charge_id=?", ch.ID).Limit(1).Scan(&itemIDs).Error; e != nil {
			return e
		}
		var e error
		out, e = review(ReconcileState{row, ch, requests, len(toolIDs) > 0, len(itemIDs) > 0})
		if e != nil {
			return e
		}
		if !apply || out.Report.AlreadyApplied {
			return nil
		}
		now := time.Now().UTC()
		for _, request := range out.Requests {
			audit, e := json.Marshal(request.Audit)
			if e != nil {
				return e
			}
			updates := map[string]any{"status": request.Status, "usage": request.Usage, "usage_status": 1, "upstream_request_id": request.Audit.UpstreamRequestID, "request_summary": gorm.Expr("jsonb_set(request_summary,'{reconciliation}',?::jsonb,true)", string(audit)), "finished_at": gorm.Expr("coalesce(finished_at,?)", request.Audit.ObservedAt), "updated_at": now}
			result := tx.Model(&pr.Fact{}).Where("id=? AND run_id=? AND (status IN(1,4) OR usage_status<>1)", request.ID, id).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrReconcileConflict
			}
		}
		if !out.Complete {
			result := tx.Model(&Model{}).Where("id=? AND execution_status>=3 AND reservation_status=1", id).Updates(map[string]any{"state_version": gorm.Expr("state_version+1"), "updated_at": now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrReconcileConflict
			}
			payload, _ := json.Marshal(struct {
				UsageStatus   int16 `json:"usageStatus"`
				BillingStatus int16 `json:"billingStatus"`
			}{2, 4})
			_, e = r.events.AppendTx(ctx, tx, id, "billingUpdated", payload, now)
			return e
		}
		if out.ActualUnits == nil {
			return ErrReconcileConflict
		}
		for i := range out.Items {
			if e = tx.Create(&out.Items[i]).Error; e != nil {
				return e
			}
		}
		result := tx.Model(&charge.Model{}).Where("id=? AND status=4 AND usage_complete=0 AND actual_units IS NULL", ch.ID).Updates(map[string]any{"status": 1, "usage_complete": 1, "actual_units": *out.ActualUnits, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReconcileConflict
		}
		result = tx.Model(&Model{}).Where("id=? AND execution_status>=3 AND reservation_status=1", id).Updates(map[string]any{"usage_status": 1, "billing_status": 1, "state_version": gorm.Expr("state_version+1"), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReconcileConflict
		}
		payload, e := json.Marshal(struct {
			ChargeID      string `json:"chargeId"`
			UsageStatus   int16  `json:"usageStatus"`
			BillingStatus int16  `json:"billingStatus"`
		}{fmt.Sprint(ch.ID), 1, 1})
		if e != nil {
			return e
		}
		if _, e = r.events.AppendTx(ctx, tx, id, "billingUpdated", payload, now); e != nil {
			return e
		}
		payload, e = json.Marshal(struct {
			ChargeID int64 `json:"chargeId"`
		}{ch.ID})
		if e != nil {
			return e
		}
		job := scheduler.Job{TaskType: "ai.usage.settle", Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: fmt.Sprintf("ai.usage.settle:%d", ch.ID), ScheduledAt: now, AvailableAt: now, Status: scheduler.JobQueued, MaxAttempts: 1, CreatedAt: now, UpdatedAt: now}
		return r.jobs.CreateJobTx(ctx, tx, &job)
	}, options)
	if e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ReconcilePlan{}, ErrReconcileConflict
		}
		return ReconcilePlan{}, e
	}
	return out, nil
}
