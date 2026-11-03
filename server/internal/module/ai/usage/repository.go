package usage

import (
	pr "admin/server/internal/module/ai/providerRequest"
	chargeItem "admin/server/internal/module/ai/usage/chargeItem"
	"admin/server/internal/module/system/scheduler"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/ai/run/event"
	"admin/server/internal/module/ai/usage/charge"
	ledger "admin/server/internal/module/payment/transaction"
	"admin/server/internal/module/payment/wallet"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type eventWriter interface {
	AppendTx(context.Context, *gorm.DB, int64, string, json.RawMessage, time.Time) (event.Model, error)
}
type Repository struct {
	db     *gorm.DB
	wallet *wallet.Repository
	ledger *ledger.Repository
	events eventWriter
	jobs   *scheduler.Repository
}

func (r *Repository) SetScheduler(jobs *scheduler.Repository) { r.jobs = jobs }

func NewRepository(db *gorm.DB, walletRepository *wallet.Repository, ledgerRepository *ledger.Repository, events ...eventWriter) *Repository {
	var e eventWriter
	if len(events) > 0 {
		e = events[0]
	}
	return &Repository{db: db, wallet: walletRepository, ledger: ledgerRepository, events: e}
}

type settlementTx struct {
	tx     *gorm.DB
	row    charge.Model
	wallet *wallet.Repository
	ledger *ledger.Repository
	events eventWriter
}

func (r *Repository) WithCharge(ctx context.Context, id int64, fn func(SettlementTx) error) error {
	if r == nil || r.db == nil || r.wallet == nil || r.ledger == nil || id < 1 || fn == nil {
		return errors.New("AI settlement repository is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SET LOCAL lock_timeout='1s'; SET LOCAL statement_timeout='2s'`).Error; err != nil {
			return err
		}
		// Charge is the canonical first lock. Wallet and Run are always acquired after it.
		var current charge.Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, id).Error; err != nil {
			return err
		}
		var balance wallet.Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&balance, "user_id=?", current.UserID).Error; err != nil {
			return err
		}
		var run struct {
			ID, UserID, PlatformID, ProviderModelID, PriceVersionID, BindingID int64
			ExecutionStatus, BillingStatus, ReservationStatus                  int16
			MultiplierPPM                                                      int64
			ConfigSnapshot, PolicySnapshot                                     []byte
		}
		if err := tx.Raw(`SELECT id,user_id,platform_id,provider_model_id,price_version_id,binding_id,execution_status,billing_status,reservation_status,multiplier_ppm,config_snapshot,policy_snapshot FROM ai_run WHERE id=? FOR UPDATE`, current.RunID).Scan(&run).Error; err != nil {
			return err
		}
		if run.ID == 0 || run.UserID != current.UserID || run.PlatformID != current.PlatformID {
			return ErrLedgerConflict
		}
		return fn(&settlementTx{tx: tx, row: current, wallet: r.wallet, ledger: r.ledger, events: r.events})
	})
}

// MarkFailure preserves a settlement retry schedule instead of relying only
// on an Asynq redelivery. Unknown usage is terminal and remains blocked for
// operator reconciliation; transient ledger/COS/database failures back off.
func (r *Repository) MarkFailure(ctx context.Context, id int64, cause error, now time.Time) error {
	if r == nil || r.db == nil || id < 1 || now.IsZero() {
		return errors.New("AI settlement failure input invalid")
	}
	if cause == nil {
		cause = errors.New("AI settlement failed")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row charge.Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return err
		}
		if row.Status == 2 || row.Status == 4 {
			return nil
		}
		status := int16(3)
		var next *time.Time
		if errors.Is(cause, ErrUsageUnknown) {
			status = 4
		} else if row.SettlementAttempts+1 < 10 {
			value := now.Add(settlementBackoff(row.SettlementAttempts + 1))
			next = &value
		}
		message := "AI 扣款未完成，等待重试或核对"
		if err := tx.Model(&charge.Model{}).Where("id=?", id).Updates(map[string]any{"status": status, "settlement_attempts": gorm.Expr("settlement_attempts+1"), "next_settle_at": next, "error_code": "SETTLEMENT_FAILED", "error_message": message, "updated_at": now}).Error; err != nil {
			return err
		}
		changed := tx.Table("ai_run").Where("id=? AND user_id=? AND platform_id=? AND execution_status IN(3,4,5) AND billing_status<>2", row.RunID, row.UserID, row.PlatformID).Updates(map[string]any{"billing_status": status, "state_version": gorm.Expr("state_version+1"), "updated_at": now})
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return ErrLedgerConflict
		}
		if r.events != nil {
			payload, _ := json.Marshal(map[string]any{"chargeId": row.ID, "billingStatus": status, "automaticRetryExhausted": status == 3 && next == nil})
			if _, err := r.events.AppendTx(ctx, tx, row.RunID, "billingUpdated", payload, now); err != nil {
				return err
			}
		}
		if next != nil {
			row.SettlementAttempts++
			return r.ensureSettlementTx(ctx, tx, row, *next, now)
		}
		return nil
	})
}

func (r *Repository) ensureSettlementTx(ctx context.Context, tx *gorm.DB, row charge.Model, available, now time.Time) error {
	if r.jobs == nil {
		return errors.New("AI settlement scheduler unavailable")
	}
	key := fmt.Sprintf("ai.usage.settle:%d", row.ID)
	if row.SettlementAttempts > 0 {
		key = fmt.Sprintf("%s:retry:%d", key, row.SettlementAttempts)
	}
	var job scheduler.Job
	err := tx.Where("source_key=? AND task_type=?", key, "ai.usage.settle").Take(&job).Error
	if err == nil {
		if job.Status == scheduler.JobScheduled || job.Status == scheduler.JobQueued || job.Status == scheduler.JobRunning {
			return nil
		}
		return tx.Model(&scheduler.Job{}).Where("id=?", job.ID).Updates(map[string]any{"status": scheduler.JobQueued, "attempt_count": 0, "available_at": available, "last_error": nil, "error_class": nil, "updated_at": now}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	payload, _ := json.Marshal(map[string]int64{"chargeId": row.ID})
	job = scheduler.Job{TaskType: "ai.usage.settle", Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: key, ScheduledAt: available, AvailableAt: available, Status: scheduler.JobQueued, MaxAttempts: 1, CreatedAt: now, UpdatedAt: now}
	return r.jobs.CreateJobTx(ctx, tx, &job)
}

func (r *Repository) RecoverSettlements(ctx context.Context, now time.Time, limit int) (int, error) {
	if r == nil || r.db == nil || r.jobs == nil || limit < 1 || limit > 100 {
		return 0, errors.New("AI settlement recovery unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	count := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []charge.Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status IN(1,3) AND usage_complete=1 AND settlement_attempts<10 AND (next_settle_at IS NULL OR next_settle_at<=?)", now).Order("next_settle_at NULLS FIRST,id").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := r.ensureSettlementTx(ctx, tx, row, now, now); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

func settlementBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 5 * time.Second
	for i := 1; i < attempt && delay < 5*time.Minute; i++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func (tx *settlementTx) FindFacts(ctx context.Context) (SettlementFacts, error) {
	var run struct {
		ID, UserID, PlatformID, PriceVersionID int64
		ExecutionStatus                        int16
		MultiplierPPM                          int64
		ConfigSnapshot, PolicySnapshot         []byte
		Operation                              string
		InputSnapshot                          json.RawMessage
	}
	if e := tx.tx.WithContext(ctx).Raw(`SELECT id,user_id,platform_id,price_version_id,execution_status,multiplier_ppm,config_snapshot,policy_snapshot,operation,input_snapshot FROM ai_run WHERE id=?`, tx.row.RunID).Scan(&run).Error; e != nil {
		return SettlementFacts{}, e
	}
	if run.ID == 0 {
		return SettlementFacts{}, ErrFrozenSnapshotMismatch
	}
	var requests []pr.Fact
	if e := tx.tx.WithContext(ctx).Where("run_id=?", tx.row.RunID).Order("sequence").Limit(1001).Find(&requests).Error; e != nil {
		return SettlementFacts{}, e
	}
	if len(requests) > 1000 {
		return SettlementFacts{}, ErrFrozenSnapshotMismatch
	}
	var unknownTools int64
	if e := tx.tx.WithContext(ctx).Table("ai_tool_call").Where("run_id=? AND status IN(1,4)", tx.row.RunID).Count(&unknownTools).Error; e != nil {
		return SettlementFacts{}, e
	}
	if unknownTools > 0 {
		return SettlementFacts{}, ErrUsageUnknown
	}
	rows, e := tx.tx.WithContext(ctx).Raw(`SELECT id,operation,usage_key,selector,unit,unit_quantity::text,billing_unit_price_units FROM ai_provider_model_price_rate WHERE price_version_id=?`, tx.row.PriceVersionID).Rows()
	if e != nil {
		return SettlementFacts{}, e
	}
	defer rows.Close()
	rates := make([]FrozenRate, 0)
	for rows.Next() {
		var r FrozenRate
		if e := rows.Scan(&r.ID, &r.Operation, &r.UsageKey, &r.Selector, &r.Unit, &r.UnitQuantity, &r.BillingUnitPriceUnits); e != nil {
			return SettlementFacts{}, e
		}
		rates = append(rates, r)
	}
	if e := rows.Err(); e != nil {
		return SettlementFacts{}, e
	}
	digest := digestSnapshot(run.ConfigSnapshot, run.PolicySnapshot)
	facts := SettlementFacts{RunID: run.ID, UserID: run.UserID, PlatformID: run.PlatformID, PriceVersionID: run.PriceVersionID, MultiplierPPM: run.MultiplierPPM, SnapshotDigest: digest, Terminal: run.ExecutionStatus >= 3, Rates: rates}
	if err := attachRequestFacts(&facts, run.Operation, run.InputSnapshot, requests); err != nil {
		return SettlementFacts{}, err
	}
	return facts, nil
}
func digestSnapshot(config, policy []byte) string {
	h := sha256.New()
	h.Write(bytes.TrimSpace(config))
	h.Write([]byte{0})
	h.Write(bytes.TrimSpace(policy))
	return hex.EncodeToString(h.Sum(nil))
}
func (tx *settlementTx) FindItems(ctx context.Context) ([]chargeItem.Model, error) {
	var rows []chargeItem.Model
	e := tx.tx.WithContext(ctx).Where("charge_id=?", tx.row.ID).Order("id").Find(&rows).Error
	return rows, e
}
func (tx *settlementTx) Charge() charge.Model { return tx.row }
func (tx *settlementTx) FindLedger(ctx context.Context) (ledger.Model, bool, error) {
	var row ledger.Model
	err := tx.tx.WithContext(ctx).First(&row, "ai_usage_charge_id=?", tx.row.ID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	return row, err == nil, err
}
func (tx *settlementTx) Debit(ctx context.Context, amount int64, now time.Time) (int64, int64, error) {
	after, before, err := tx.wallet.DebitTx(ctx, tx.tx, tx.row.UserID, amount, now)
	return before, after.BalanceUnits, err
}
func (tx *settlementTx) AppendLedger(ctx context.Context, row ledger.Model) error {
	return tx.ledger.CreateTx(ctx, tx.tx, &row)
}
func (tx *settlementTx) MarkSettled(ctx context.Context, now time.Time) error {
	result := tx.tx.WithContext(ctx).Model(&charge.Model{}).Where("id=? AND status IN(1,3) AND usage_complete=1", tx.row.ID).Updates(map[string]any{"status": 2, "settlement_attempts": gorm.Expr("settlement_attempts+1"), "settled_at": now, "next_settle_at": nil, "error_code": nil, "error_message": nil, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLedgerConflict
	}
	result = tx.tx.WithContext(ctx).Table("ai_run").Where("id=? AND user_id=? AND platform_id=? AND execution_status IN(3,4,5)", tx.row.RunID, tx.row.UserID, tx.row.PlatformID).Updates(map[string]any{"billing_status": 2, "reservation_status": 2, "state_version": gorm.Expr("state_version+1"), "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLedgerConflict
	}
	if tx.events != nil {
		payload, _ := json.Marshal(map[string]any{"chargeId": tx.row.ID, "amountUnits": tx.row.ActualUnits})
		if _, e := tx.events.AppendTx(ctx, tx.tx, tx.row.RunID, "billingUpdated", payload, now); e != nil {
			return e
		}
	}
	return nil
}
