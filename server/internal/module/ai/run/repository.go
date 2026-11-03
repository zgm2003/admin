package run

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"admin/server/internal/module/ai"
	"admin/server/internal/module/ai/conversation"
	"admin/server/internal/module/ai/message"
	runevent "admin/server/internal/module/ai/run/event"
	"admin/server/internal/module/payment/wallet"
	"admin/server/internal/module/system/scheduler"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RuntimeReader interface {
	Get(context.Context, int64, time.Time) (RuntimeSnapshot, error)
}
type Repository struct {
	db            *gorm.DB
	wallet        *wallet.Repository
	scheduler     *scheduler.Repository
	events        *runevent.Repository
	conversations *conversation.Repository
}

func (r *Repository) SetEvents(events *runevent.Repository) { r.events = events }
func (r *Repository) SetConversations(conversations *conversation.Repository) {
	r.conversations = conversations
}

func NewRepository(db *gorm.DB, w *wallet.Repository, s *scheduler.Repository) *Repository {
	return &Repository{db: db, wallet: w, scheduler: s}
}

var ErrIdempotencyConflict = errors.New("AI request key was already used with different input")

func fingerprint(in CreateInput) (string, error) {
	normalized, err := normalizeObject(in.Input)
	if err != nil {
		return "", err
	}
	// File snapshots are a server-owned freeze artifact. They must be stored in
	// the Run input but not change client idempotency when the same request is
	// replayed after validation.
	var input map[string]json.RawMessage
	if json.Unmarshal(normalized, &input) == nil {
		delete(input, "fileSnapshots")
		if normalized, err = json.Marshal(input); err != nil {
			return "", err
		}
	}
	raw, err := json.Marshal(struct {
		AgentID        int64           `json:"agentId"`
		ConversationID *int64          `json:"conversationId"`
		Operation      string          `json:"operation"`
		Input          json.RawMessage `json:"input"`
	}{in.AgentID, in.ConversationID, in.Operation, normalized})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func (r *Repository) FindByRequest(ctx context.Context, userID, platformID int64, key string) (Model, bool, error) {
	var row Model
	if r == nil || r.db == nil {
		return row, false, errors.New("AI run database unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := r.db.WithContext(ctx).Where("user_id=? AND platform_id=? AND request_key=?", userID, platformID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	return row, err == nil, err
}
func (r *Repository) Find(ctx context.Context, id int64) (Model, error) {
	var row Model
	if r == nil || r.db == nil {
		return row, errors.New("AI run database unavailable")
	}
	err := r.db.WithContext(ctx).First(&row, id).Error
	return row, err
}

func (r *Repository) Create(ctx context.Context, in CreateInput, snap RuntimeSnapshot, policy json.RawMessage, now time.Time) (Model, bool, error) {
	var out Model
	created := false
	if r == nil || r.db == nil || r.wallet == nil || r.scheduler == nil || r.events == nil {
		return out, false, errors.New("AI run repository is not configured")
	}
	fp, err := fingerprint(in)
	if err != nil {
		return out, false, err
	}
	if _, err = ai.DecodeJSONObject(policy); err != nil {
		return out, false, err
	}
	var p struct {
		UserInflightLimit int64 `json:"userInflightLimit"`
	}
	if err = json.Unmarshal(policy, &p); err != nil || p.UserInflightLimit < 1 || p.UserInflightLimit >= math.MaxInt32 {
		return out, false, errors.New("AI inflight policy invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec(`SET LOCAL lock_timeout='1s'; SET LOCAL statement_timeout='2s'`).Error; e != nil {
			return e
		}
		replay := func() (bool, error) {
			var old Model
			e := tx.Where("user_id=? AND platform_id=? AND request_key=?", in.UserID, in.PlatformID, in.RequestKey).First(&old).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return false, nil
			}
			if e != nil {
				return false, e
			}
			if old.RequestFingerprint != fp {
				return false, ErrIdempotencyConflict
			}
			out = old
			return true, nil
		}
		if found, e := replay(); e != nil || found {
			return e
		}
		balance, e := r.wallet.LockOrCreate(ctx, tx, in.UserID, now)
		if e != nil {
			return e
		}
		// The wallet lock serializes admissions across all platforms for this user.
		// Re-read idempotency after acquiring it so competing same-key requests return
		// the winner without conflicting INSERTs or charging either request twice.
		if found, e := replay(); e != nil || found {
			return e
		}
		type inflightRow struct {
			EstimatedUnits int64
			UsageStatus    int16
			UnknownRequest bool
		}
		rows := make([]inflightRow, 0)
		if e = tx.Raw(`SELECT r.estimated_units,r.usage_status,(EXISTS(SELECT 1 FROM ai_provider_request q WHERE q.run_id=r.id AND (q.status=4 OR q.usage_status=2)) OR EXISTS(SELECT 1 FROM ai_tool_call t WHERE t.run_id=r.id AND t.status=4)) AS unknown_request FROM ai_run r WHERE r.user_id=? AND r.reservation_status=1 ORDER BY r.id LIMIT ?`, in.UserID, p.UserInflightLimit+1).Scan(&rows).Error; e != nil {
			return e
		}
		total := int64(0)
		unknown := false
		for _, v := range rows {
			if v.EstimatedUnits < 0 || total > math.MaxInt64-v.EstimatedUnits {
				return ErrAdmissionDenied
			}
			total += v.EstimatedUnits
			unknown = unknown || v.UsageStatus == 2 || v.UnknownRequest
		}
		if _, e = Admit(AdmissionInput{BalanceUnits: balance.BalanceUnits, EstimatedUnits: in.EstimatedUnits, InflightEstimatedUnits: total, InflightCount: len(rows), InflightLimit: int(p.UserInflightLimit), UnknownInflight: unknown, VariableUsage: in.VariableUsage, Now: now}); e != nil {
			return e
		}
		if in.ConversationID != nil {
			if r.conversations == nil {
				return ErrRuntimeUnavailable
			}
			conversation, err := r.conversations.LockOwnedTx(ctx, tx, *in.ConversationID, in.UserID, in.PlatformID)
			if err != nil {
				return err
			}
			if conversation.AgentID != in.AgentID || in.Operation != "text.generate" {
				return ErrInvalidSnapshot
			}
			var active int64
			if err = tx.Raw(`SELECT count(*) FROM ai_run WHERE conversation_id=? AND execution_status IN(0,1,2)`, *in.ConversationID).Scan(&active).Error; err != nil {
				return err
			}
			if active != 0 {
				return ErrAdmissionDenied
			}
		}
		normalized, e := NewRuntimeSnapshot(snap)
		if e != nil {
			return e
		}
		if normalized.AgentID != in.AgentID {
			return ErrInvalidSnapshot
		}
		config, e := json.Marshal(normalized)
		if e != nil {
			return e
		}
		if len(config) > 256*1024 {
			return ErrInvalidSnapshot
		}
		if !in.DeadlineAt.After(now) || in.UserID < 1 || in.PlatformID < 1 {
			return ErrAdmissionDenied
		}
		input, e := lockRunInputs(ctx, tx, in)
		if e != nil {
			return e
		}
		out = Model{UserID: in.UserID, PlatformID: in.PlatformID, RequestKey: in.RequestKey, RequestFingerprint: fp, ConversationID: in.ConversationID, AgentID: snap.AgentID, BindingID: snap.BindingID, ProviderModelID: snap.ProviderModelID, ProviderVersionID: snap.ProviderVersionID, OfficialModelID: snap.OfficialModelID, PriceVersionID: snap.PriceVersionID, Operation: in.Operation, InputSnapshot: in.Input, ConfigSnapshot: config, PolicySnapshot: policy, MultiplierPPM: snap.MultiplierPPM, EstimatedUnits: in.EstimatedUnits, ReservationStatus: 1, StateVersion: 1, AcceptedAt: snap.AcceptedAt.UTC(), DeadlineAt: in.DeadlineAt, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
		if e = tx.Create(&out).Error; e != nil {
			return e
		}
		if e = appendRunInputs(ctx, tx, out.ID, input, now); e != nil {
			return e
		}
		if in.ConversationID != nil {
			data, err := decodeInput(in.Input)
			if err != nil {
				return err
			}
			content, err := json.Marshal([]map[string]string{{"type": "input_text", "text": data.Text}})
			if err != nil {
				return err
			}
			if _, err = message.NewRepository(tx).AppendTx(ctx, tx, *in.ConversationID, out.ID, 1, content, 1, now); err != nil {
				return err
			}
		}
		payload, e := json.Marshal(map[string]any{"runId": out.ID})
		if e != nil {
			return e
		}
		queued, e := r.events.AppendTx(ctx, tx, out.ID, "queued", payload, now)
		if e != nil {
			return e
		}
		out.NextEventSequence = queued.Sequence
		job := scheduler.Job{TaskType: "ai.run.execute", Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: fmt.Sprintf("ai.run.execute:%d:1", out.ID), ScheduledAt: now, AvailableAt: now, Status: scheduler.JobQueued, MaxAttempts: 1, CreatedAt: now, UpdatedAt: now}
		if e = r.scheduler.CreateJobTx(ctx, tx, &job); e != nil {
			return e
		}
		created = true
		return nil
	})
	if err != nil {
		return Model{}, false, err
	}
	return out, created, nil
}

func (r *Repository) CountCapacity(ctx context.Context, queuedLimit, activeLimit int64) (CapacityCounts, error) {
	var result CapacityCounts
	if r == nil || r.db == nil || queuedLimit < 1 || activeLimit < 1 {
		return result, ErrAdmissionDenied
	}
	err := r.db.WithContext(ctx).Raw(`SELECT (SELECT count(*) FROM (SELECT 1 FROM ai_run WHERE execution_status=0 LIMIT ?) q) AS queued,(SELECT count(*) FROM (SELECT 1 FROM ai_run WHERE execution_status IN(1,2) LIMIT ?) a) AS active,(SELECT count(*) FROM (SELECT 1 FROM realtime_event_outbox WHERE published_at IS NULL LIMIT 100001) o) AS backlog`, queuedLimit, activeLimit).Scan(&result).Error
	return result, err
}

// RecoverExpired closes leases whose owner disappeared. It never reclaims the
// provider POST: an in-flight result is unknown and therefore remains blocked
// from new consumption until an operator reconciles it.
func (r *Repository) RecoverExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	return r.recoverDeadRuns(ctx, now, limit)
}

// RecoverQueued closes runs that exceeded the queue deadline and reopens a
// failed scheduler envelope for still-valid queued runs. The unique source
// key is reused, so recovery cannot create duplicate execute jobs.
func (r *Repository) RecoverQueued(ctx context.Context, now time.Time, limit int) (int, error) {
	if r == nil || r.db == nil || r.scheduler == nil || r.events == nil || now.IsZero() || limit < 1 || limit > 100 {
		return 0, ErrRunLeaseLost
	}
	count := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("execution_status=? AND accepted_at<?", 0, now).Order("accepted_at,id").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			code := ""
			terminalStatus := int16(4)
			var policy Policy
			if row.CancelRequestedAt != nil {
				code = "CANCELED"
				terminalStatus = 5
			} else if json.Unmarshal(row.PolicySnapshot, &policy) != nil || policy.Validate() != nil {
				code = "INVALID_POLICY"
			} else if !row.AcceptedAt.Add(time.Duration(policy.MaxQueuedSeconds) * time.Second).After(now) {
				code = "QUEUE_TIMEOUT"
			}
			if code != "" {
				if err := r.finishTextTx(ctx, tx, row, terminalStatus, code, "", now); err != nil {
					return err
				}
				count++
				continue
			}
			sourceKey := fmt.Sprintf("ai.run.execute:%d:1", row.ID)
			var status *int16
			if err := tx.Table("system_scheduler_job").Select("status").Where("task_type=? AND source_key=?", "ai.run.execute", sourceKey).Order("id DESC").Limit(1).Scan(&status).Error; err != nil {
				return err
			}
			if status != nil && (*status == int16(scheduler.JobScheduled) || *status == int16(scheduler.JobQueued) || *status == int16(scheduler.JobRunning)) {
				continue
			}
			payload, _ := json.Marshal(map[string]any{"runId": row.ID})
			if status != nil {
				if err := tx.Table("system_scheduler_job").Where("task_type=? AND source_key=?", "ai.run.execute", sourceKey).Updates(map[string]any{"payload": payload, "available_at": now, "status": scheduler.JobQueued, "attempt_count": 0, "last_error": nil, "error_class": nil, "updated_at": now}).Error; err != nil {
					return err
				}
			} else {
				job := scheduler.Job{TaskType: "ai.run.execute", Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: sourceKey, ScheduledAt: now, AvailableAt: now, Status: scheduler.JobQueued, MaxAttempts: 1, CreatedAt: now, UpdatedAt: now}
				if err := r.scheduler.CreateJobTx(ctx, tx, &job); err != nil {
					return err
				}
			}
			count++
		}
		return nil
	})
	return count, err
}

// RecoverStaleEffects closes provider/tool effects whose lease owner vanished.
// Provider POSTs are never retried; their usage remains unknown until an
// operator reconciles the upstream fact.
func (r *Repository) RecoverStaleEffects(ctx context.Context, now time.Time, limit int) (int, error) {
	// A long provider request is not stale while its Run owner renews its lease.
	return r.recoverDeadRuns(ctx, now, limit)
}
