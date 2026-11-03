package run

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/ai"
	"admin/server/internal/module/ai/message"
	pr "admin/server/internal/module/ai/providerRequest"
	tc "admin/server/internal/module/ai/toolCall"
	"admin/server/internal/module/ai/usage"
	"admin/server/internal/module/ai/usage/charge"
	"admin/server/internal/module/system/scheduler"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// withExecution serializes a durable effect with its Run fencing token. Results
// may be recorded after cancellation/deadline, but starting I/O requires a live
// uncancelled lease. A replacement owner can never write either kind of fact.
func (r *Repository) withExecution(ctx context.Context, id int64, token string, start bool, now time.Time, fn func(*gorm.DB, Model) error) error {
	if r == nil || r.db == nil || r.events == nil || id < 1 || uuid.Validate(token) != nil {
		return ErrRunLeaseLost
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec(`SET LOCAL lock_timeout='1s'; SET LOCAL statement_timeout='2s'`).Error; e != nil {
			return e
		}
		var row Model
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND lease_token=?::uuid", id, token)
		if start {
			q = q.Where("execution_status=1 AND lease_until>? AND deadline_at>? AND cancel_requested_at IS NULL", now, now)
		}
		if e := q.First(&row).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return ErrRunLeaseLost
			}
			return e
		}
		return fn(tx, row)
	})
}

func (r *Repository) StartRequest(ctx context.Context, id int64, token string, sequence int, summary json.RawMessage, now time.Time) (pr.Fact, error) {
	var out pr.Fact
	normalized, e := normalizeObject(summary)
	if e != nil || len(normalized) > 256*1024 || sequence < 1 {
		return out, ErrInvalidSnapshot
	}
	hash := sha256.Sum256(normalized)
	e = r.withExecution(ctx, id, token, true, now, func(tx *gorm.DB, row Model) error {
		out = pr.Fact{RunID: id, Sequence: sequence, RequestKey: uuid.NewString(), RequestFingerprint: hex.EncodeToString(hash[:]), Status: 1, RequestSummary: normalized, ResponseItems: json.RawMessage(`[]`), Usage: json.RawMessage(`{}`), StartedAt: &now, CreatedAt: now, UpdatedAt: now}
		// A duplicate sequence is never interpreted as permission to resend a POST.
		return tx.Create(&out).Error
	})
	return out, e
}

func (r *Repository) FinishRequest(ctx context.Context, id int64, token string, requestID int64, response pr.TextResponse, requestErr error, now time.Time) error {
	items, e := json.Marshal(response.OutputItems)
	if e != nil {
		return e
	}
	if response.OutputItems == nil {
		items = []byte(`[]`)
	}
	if len(items) > 4*1024*1024 {
		return errors.New("AI response items exceed persistence limit")
	}
	u := response.Usage
	// A definite local rejection or HTTP rejection is the only no-usage result.
	unknown := errors.Is(requestErr, pr.ErrSubmissionUnknown)
	if requestErr != nil && !unknown && !u.Known && (errors.Is(requestErr, pr.ErrRequestInvalid) || errors.Is(requestErr, pr.ErrUpstreamRejected)) {
		u = pr.TokenUsage{Known: true}
	}
	usageJSON, e := json.Marshal(u)
	if e != nil || len(usageJSON) > 65536 {
		return ErrInvalidSnapshot
	}
	status := int16(2)
	if requestErr != nil {
		status = 3
	}
	if unknown {
		status = 4
	}
	usageStatus := int16(1)
	if !u.Known {
		usageStatus = 2
	}
	return r.withExecution(ctx, id, token, false, now, func(tx *gorm.DB, row Model) error {
		if row.ExecutionStatus != 1 {
			return ErrRunLeaseLost
		}
		updates := map[string]any{"status": status, "response_items": json.RawMessage(items), "usage": json.RawMessage(usageJSON), "usage_status": usageStatus, "finished_at": now, "updated_at": now}
		if response.RequestID != "" && len(response.RequestID) <= 200 {
			updates["upstream_request_id"] = response.RequestID
		}
		if requestErr != nil {
			updates["error_code"] = "PROVIDER_REQUEST_FAILED"
			updates["error_message"] = "供应商请求未成功完成"
		}
		res := tx.Model(&pr.Fact{}).Where("id=? AND run_id=? AND status=1", requestID, id).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrRunLeaseLost
		}
		return nil
	})
}

// FinishMediaRequest persists a non-text provider response without coercing
// image/audio usage into token counters. Unknown submission outcomes are kept
// as usage_status=2 and therefore cannot be silently settled.
func (r *Repository) FinishMediaRequest(ctx context.Context, id int64, token string, requestID int64, requestIDHeader string, items json.RawMessage, usage pr.MediaUsage, requestErr error, now time.Time) error {
	return r.finishMediaRequest(ctx, id, token, requestID, requestIDHeader, items, usage, requestErr, nil, now)
}

func (r *Repository) finishMediaRequest(ctx context.Context, id int64, token string, requestID int64, requestIDHeader string, items json.RawMessage, usage pr.MediaUsage, requestErr error, files []MediaResultFile, now time.Time) error {
	if !json.Valid(items) || len(items) > 4*1024*1024 || requestID < 1 || now.IsZero() {
		return ErrInvalidSnapshot
	}
	usageJSON, err := json.Marshal(usage)
	if err != nil || len(usageJSON) > 65536 {
		return ErrInvalidSnapshot
	}
	status := int16(2)
	if requestErr != nil {
		status = 3
		if errors.Is(requestErr, pr.ErrSubmissionUnknown) {
			status = 4
		}
	}
	usageStatus := int16(1)
	if !usage.Known {
		usageStatus = 2
	}
	return r.withExecution(ctx, id, token, false, now, func(tx *gorm.DB, row Model) error {
		if row.ExecutionStatus != 1 {
			return ErrRunLeaseLost
		}
		updates := map[string]any{"status": status, "response_items": items, "usage": usageJSON, "usage_status": usageStatus, "finished_at": now, "updated_at": now}
		if requestIDHeader != "" && len(requestIDHeader) <= 200 {
			updates["upstream_request_id"] = requestIDHeader
		}
		if requestErr != nil {
			updates["error_code"] = "PROVIDER_REQUEST_FAILED"
			updates["error_message"] = "供应商媒体请求未成功完成"
		}
		res := tx.Model(&pr.Fact{}).Where("id=? AND run_id=? AND status=1", requestID, id).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrRunLeaseLost
		}
		return r.createMediaResultFilesTx(ctx, tx, id, files, now)
	})
}

func (r *Repository) StartTool(ctx context.Context, id int64, token string, requestID int64, snapshot ToolSnapshot, call pr.FunctionCall, now time.Time) (tc.Model, error) {
	var out tc.Model
	args, e := ai.DecodeJSONObject(json.RawMessage(call.Arguments))
	if e != nil || len(args) > 65536 || call.CallID == "" || len(call.CallID) > 200 || call.Name != snapshot.Name {
		return out, ErrInvalidSnapshot
	}
	e = r.withExecution(ctx, id, token, true, now, func(tx *gorm.DB, row Model) error {
		var request pr.Fact
		if e := tx.Where("id=? AND run_id=? AND status=2", requestID, id).First(&request).Error; e != nil {
			return e
		}
		out = tc.Model{RunID: id, ProviderRequestID: requestID, ToolVersionID: snapshot.VersionID, CallID: call.CallID, ExposedName: call.Name, Arguments: args, Status: 1, IdempotencyKey: uuid.NewString(), ExecutionAttempts: 1, StartedAt: &now, CreatedAt: now, UpdatedAt: now}
		if e := tx.Create(&out).Error; e != nil {
			return e
		}
		payload, _ := json.Marshal(map[string]any{"callId": call.CallID, "name": call.Name})
		_, e := r.events.AppendTx(ctx, tx, id, "toolStarted", payload, now)
		return e
	})
	return out, e
}

func (r *Repository) FinishTool(ctx context.Context, id int64, token string, callID int64, result json.RawMessage, unknown bool, now time.Time) error {
	if !json.Valid(result) || len(result) > 65536 {
		return ErrInvalidSnapshot
	}
	return r.withExecution(ctx, id, token, false, now, func(tx *gorm.DB, row Model) error {
		if row.ExecutionStatus != 1 {
			return ErrRunLeaseLost
		}
		status := 2
		if unknown {
			status = 4
		}
		res := tx.Model(&tc.Model{}).Where("id=? AND run_id=? AND status=1", callID, id).Updates(map[string]any{"status": status, "result": result, "finished_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrRunLeaseLost
		}
		payload, _ := json.Marshal(map[string]any{"toolCallId": fmt.Sprint(callID), "status": status})
		_, e := r.events.AppendTx(ctx, tx, id, "toolFinished", payload, now)
		return e
	})
}

type chargeItem struct {
	ID                                                int64 `gorm:"primaryKey"`
	ChargeID, ProviderRequestID, PriceRateID          int64
	ItemKey, Operation, UsageKey                      string
	SelectorSnapshot                                  json.RawMessage `gorm:"type:jsonb"`
	Quantity, UnitQuantity, Unit                      string
	BillingUnitPriceUnits, MultiplierPPM, AmountUnits int64
	RawUsageDigest                                    string
	CreatedAt, UpdatedAt                              time.Time
}

func (chargeItem) TableName() string { return "ai_usage_charge_item" }

// FinishText derives the charge from all durable request usage, rather than a
// caller-supplied amount. Execution completion and the settlement job are atomic.
func (r *Repository) FinishText(ctx context.Context, id int64, token string, status int16, code, text string, now time.Time) error {
	if status < 3 || status > 5 || len(text) > 4*1024*1024 || len(code) > 64 {
		return ErrInvalidSnapshot
	}
	return r.withExecution(ctx, id, token, false, now, func(tx *gorm.DB, row Model) error {
		return r.finishTextTx(ctx, tx, row, status, code, text, now)
	})
}

func (r *Repository) finishTextTx(ctx context.Context, tx *gorm.DB, row Model, status int16, code, text string, now time.Time) error {
	id := row.ID
	if row.ExecutionStatus >= 3 {
		return nil
	}
	if row.ExecutionStatus != 1 && row.ExecutionStatus != 0 && row.ExecutionStatus != 2 {
		return ErrRunLeaseLost
	}
	var snap RuntimeSnapshot
	if json.Unmarshal(row.ConfigSnapshot, &snap) != nil {
		return ErrInvalidSnapshot
	}
	digestBytes := sha256.Sum256(append(append([]byte{}, bytes.TrimSpace(row.ConfigSnapshot)...), append([]byte{0}, bytes.TrimSpace(row.PolicySnapshot)...)...))
	digest := hex.EncodeToString(digestBytes[:])
	var e error
	requests := make([]pr.Fact, 0)
	if e = tx.Where("run_id=?", id).Order("sequence").Find(&requests).Error; e != nil {
		return e
	}
	var frozen Policy
	_ = json.Unmarshal(row.PolicySnapshot, &frozen)
	if frozen.MaxModelRounds > 0 && int64(len(requests)) > frozen.MaxModelRounds {
		return ErrInvalidSnapshot
	}
	var unknownTools int64
	if e := tx.Table("ai_tool_call").Where("run_id=? AND status IN(1,4)", id).Count(&unknownTools).Error; e != nil {
		return e
	}
	known := unknownTools == 0
	lines := make([]ChargeLine, 0)
	for _, request := range requests {
		var u pr.TokenUsage
		if request.Status == 1 || request.Status == 4 || request.UsageStatus != 1 || json.Unmarshal(request.Usage, &u) != nil || !u.Known {
			known = false
			continue
		}
		l, err := textUsageLines(snap, request.ID, u)
		if err != nil {
			known = false
			continue
		}
		lines = append(lines, l...)
	}
	chargeStatus := int16(4)
	usageComplete := int16(0)
	var actual *int64
	amounts := map[string]int64{}
	if known {
		total := int64(0)
		if len(lines) > 0 {
			calc := make([]usage.LineInput, 0, len(lines))
			for _, l := range lines {
				calc = append(calc, l.Calculation)
			}
			c, err := usage.Calculate(calc)
			if err != nil {
				return err
			}
			total = c.TotalUnits
			for _, v := range c.Items {
				amounts[v.ItemKey] = v.AmountUnits
			}
		}
		actual = &total
		usageComplete = 1
		chargeStatus = 1
	}
	chargeRow := charge.Model{RunID: id, UserID: row.UserID, PlatformID: row.PlatformID, Status: chargeStatus, Currency: "CNY", PriceVersionID: row.PriceVersionID, MultiplierPPM: row.MultiplierPPM, ActualUnits: actual, UsageComplete: usageComplete, SnapshotDigest: digest, CreatedAt: now, UpdatedAt: now}
	if e = tx.Create(&chargeRow).Error; e != nil {
		return e
	}
	if known {
		for _, l := range lines {
			item := chargeItem{ChargeID: chargeRow.ID, ProviderRequestID: l.RequestID, PriceRateID: l.Rate.ID, ItemKey: l.Calculation.ItemKey, Operation: l.Rate.Operation, UsageKey: l.Rate.UsageKey, SelectorSnapshot: l.Rate.Selector, Quantity: l.Calculation.Quantity, UnitQuantity: l.Rate.UnitQuantity, Unit: l.Rate.Unit, BillingUnitPriceUnits: l.Rate.BillingUnitPriceUnits, MultiplierPPM: snap.MultiplierPPM, AmountUnits: amounts[l.Calculation.ItemKey], RawUsageDigest: l.RawUsageDigest, CreatedAt: now, UpdatedAt: now}
			if e = tx.Create(&item).Error; e != nil {
				return e
			}
		}
	}
	usageStatus := 1
	billingStatus := 1
	if !known {
		usageStatus = 2
		billingStatus = 4
	}
	updates := map[string]any{"execution_status": status, "usage_status": usageStatus, "billing_status": billingStatus, "state_version": gorm.Expr("state_version+1"), "finished_at": now, "updated_at": now}
	if code != "" {
		updates["error_code"] = code
		updates["error_message"] = "AI 任务未成功完成"
	}
	if row.CancelRequestedAt != nil {
		updates["execution_status"] = 5
	}
	if e = tx.Model(&Model{}).Where("id=?", id).Updates(updates).Error; e != nil {
		return e
	}
	if row.ConversationID != nil && text != "" {
		content, _ := json.Marshal([]map[string]string{{"type": "output_text", "text": text}})
		if _, e = message.NewRepository(tx).AppendTx(ctx, tx, *row.ConversationID, id, 2, content, 1, now); e != nil {
			return e
		}
	}
	payload, _ := json.Marshal(map[string]any{"status": updates["execution_status"], "usageStatus": usageStatus, "billingStatus": billingStatus})
	if _, e = r.events.AppendTx(ctx, tx, id, "finished", payload, now); e != nil {
		return e
	}
	if known {
		p, _ := json.Marshal(map[string]any{"chargeId": chargeRow.ID})
		job := scheduler.Job{TaskType: "ai.usage.settle", Payload: p, TriggerSource: scheduler.TriggerBusiness, SourceKey: fmt.Sprintf("ai.usage.settle:%d", chargeRow.ID), ScheduledAt: now, AvailableAt: now, Status: scheduler.JobQueued, MaxAttempts: 1, CreatedAt: now, UpdatedAt: now}
		return r.scheduler.CreateJobTx(ctx, tx, &job)
	}
	return nil
}

// FinishMedia closes an image/audio Run after all durable media files have
// been imported. It uses media-specific usage dimensions and the same charge
// and settlement invariants as text Runs.
func (r *Repository) FinishMedia(ctx context.Context, id int64, token string, status int16, code string, now time.Time) error {
	if status < 3 || status > 5 || len(code) > 64 {
		return ErrInvalidSnapshot
	}
	return r.withExecution(ctx, id, token, false, now, func(tx *gorm.DB, row Model) error {
		return r.finishMediaTx(ctx, tx, row, status, code, now)
	})
}

func (r *Repository) finishMediaTx(ctx context.Context, tx *gorm.DB, row Model, status int16, code string, now time.Time) error {
	id := row.ID
	if row.ExecutionStatus >= 3 {
		return nil
	}
	if row.ExecutionStatus != 0 && row.ExecutionStatus != 1 && row.ExecutionStatus != 2 {
		return ErrRunLeaseLost
	}
	var snap RuntimeSnapshot
	if json.Unmarshal(row.ConfigSnapshot, &snap) != nil {
		return ErrInvalidSnapshot
	}
	digestBytes := sha256.Sum256(append(append([]byte{}, bytes.TrimSpace(row.ConfigSnapshot)...), append([]byte{0}, bytes.TrimSpace(row.PolicySnapshot)...)...))
	digest := hex.EncodeToString(digestBytes[:])
	var requests []pr.Fact
	if err := tx.Where("run_id=?", id).Order("sequence").Find(&requests).Error; err != nil {
		return err
	}
	known := true
	lines := make([]ChargeLine, 0)
	for _, request := range requests {
		var usageValue pr.MediaUsage
		if request.Status == 1 || request.Status == 4 || request.UsageStatus != 1 || json.Unmarshal(request.Usage, &usageValue) != nil || !usageValue.Known {
			known = false
			continue
		}
		var input RunInput
		if json.Unmarshal(row.InputSnapshot, &input) != nil {
			return ErrInvalidSnapshot
		}
		part, err := mediaUsageLines(snap, request.ID, usageValue, row.Operation, input)
		if err != nil {
			known = false
			continue
		}
		lines = append(lines, part...)
	}
	chargeStatus := int16(4)
	usageComplete := int16(0)
	var actual *int64
	amounts := map[string]int64{}
	if known {
		calc := make([]usage.LineInput, 0, len(lines))
		for _, line := range lines {
			calc = append(calc, line.Calculation)
		}
		var calculated usage.CalculatedCharge
		if len(calc) > 0 {
			var err error
			calculated, err = usage.Calculate(calc)
			if err != nil {
				return err
			}
		}
		total := calculated.TotalUnits
		actual = &total
		usageComplete = 1
		chargeStatus = 1
		for _, item := range calculated.Items {
			amounts[item.ItemKey] = item.AmountUnits
		}
	}
	chargeRow := charge.Model{RunID: id, UserID: row.UserID, PlatformID: row.PlatformID, Status: chargeStatus, Currency: "CNY", PriceVersionID: row.PriceVersionID, MultiplierPPM: row.MultiplierPPM, ActualUnits: actual, UsageComplete: usageComplete, SnapshotDigest: digest, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&chargeRow).Error; err != nil {
		return err
	}
	if known {
		for _, line := range lines {
			item := chargeItem{ChargeID: chargeRow.ID, ProviderRequestID: line.RequestID, PriceRateID: line.Rate.ID, ItemKey: line.Calculation.ItemKey, Operation: line.Rate.Operation, UsageKey: line.Rate.UsageKey, SelectorSnapshot: line.Rate.Selector, Quantity: line.Calculation.Quantity, UnitQuantity: line.Rate.UnitQuantity, Unit: line.Rate.Unit, BillingUnitPriceUnits: line.Rate.BillingUnitPriceUnits, MultiplierPPM: snap.MultiplierPPM, AmountUnits: amounts[line.Calculation.ItemKey], RawUsageDigest: line.RawUsageDigest, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
	}
	usageStatus, billingStatus := 1, 1
	if !known {
		usageStatus, billingStatus = 2, 4
	}
	var transfer struct{ Total, Queued, Active, Ready, Failed int64 }
	if err := tx.Raw(`SELECT count(*) AS total,count(*) FILTER(WHERE transfer_status IN(0,3)) AS queued,count(*) FILTER(WHERE transfer_status=1) AS active,count(*) FILTER(WHERE transfer_status=2) AS ready,count(*) FILTER(WHERE transfer_status=4) AS failed FROM ai_media_file WHERE run_id=? AND role=2`, id).Scan(&transfer).Error; err != nil {
		return err
	}
	transferStatus := 0
	if transfer.Total > 0 {
		transferStatus = 1
		if transfer.Active > 0 {
			transferStatus = 2
		} else if transfer.Ready == transfer.Total {
			transferStatus = 3
		} else if transfer.Queued == 0 && transfer.Failed > 0 {
			transferStatus = 4
			if transfer.Ready == 0 {
				transferStatus = 5
			}
		}
	}
	updates := map[string]any{"execution_status": status, "usage_status": usageStatus, "billing_status": billingStatus, "transfer_status": transferStatus, "state_version": gorm.Expr("state_version+1"), "finished_at": now, "updated_at": now}
	if code != "" {
		updates["error_code"] = code
		updates["error_message"] = "AI 媒体任务未成功完成"
	}
	if row.CancelRequestedAt != nil {
		updates["execution_status"] = 5
	}
	if err := tx.Model(&Model{}).Where("id=?", id).Updates(updates).Error; err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"status": updates["execution_status"], "usageStatus": usageStatus, "billingStatus": billingStatus})
	if _, err := r.events.AppendTx(ctx, tx, id, "finished", payload, now); err != nil {
		return err
	}
	if known {
		payload, _ := json.Marshal(map[string]any{"chargeId": chargeRow.ID})
		job := scheduler.Job{TaskType: "ai.usage.settle", Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: fmt.Sprintf("ai.usage.settle:%d", chargeRow.ID), ScheduledAt: now, AvailableAt: now, Status: scheduler.JobQueued, MaxAttempts: 1, CreatedAt: now, UpdatedAt: now}
		return r.scheduler.CreateJobTx(ctx, tx, &job)
	}
	return nil
}
