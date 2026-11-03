package media

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"admin/server/internal/module/system/scheduler"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrUnavailable = errors.New("AI media repository unavailable")
	ErrConflict    = errors.New("AI media state conflict")
)

type Repository struct {
	db        *gorm.DB
	scheduler *scheduler.Repository
}

func NewRepository(db *gorm.DB, jobs ...*scheduler.Repository) *Repository {
	r := &Repository{db: db}
	if len(jobs) > 0 {
		r.scheduler = jobs[0]
	}
	return r
}

func (r *Repository) ClaimPoll(ctx context.Context, now time.Time) (Task, bool, error) {
	if r == nil || r.db == nil || now.IsZero() {
		return Task{}, false, ErrUnavailable
	}
	var out Task
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status=? AND (next_poll_at IS NULL OR next_poll_at<=?)", TaskPolling, now).
			Order("id").Limit(1).Take(&out).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		found = true
		if !out.DeadlineAt.After(now) {
			out.Status = TaskExpired
			return tx.Model(&Task{}).Where("id=? AND status=?", out.ID, TaskPolling).Updates(map[string]any{"status": TaskExpired, "error_code": "MEDIA_DEADLINE", "error_message": "媒体任务超过转存截止时间", "updated_at": now}).Error
		}
		next := now.Add(time.Minute)
		if err := tx.Model(&Task{}).Where("id=? AND status=?", out.ID, TaskPolling).Updates(map[string]any{"next_poll_at": next, "poll_attempts": gorm.Expr("poll_attempts+1"), "updated_at": now}).Error; err != nil {
			return err
		}
		out.NextPollAt = &next
		out.PollAttempts++
		return nil
	})
	return out, found, err
}

func (r *Repository) MarkPoll(ctx context.Context, taskID int64, status int16, progress *int16, result json.RawMessage, errorCode, errorMessage string, next *time.Time, now time.Time) error {
	if taskID < 1 || now.IsZero() || len(result) == 0 || status < TaskPolling || status > TaskExpired {
		return ErrConflict
	}
	values := map[string]any{"status": status, "progress": progress, "upstream_result": result, "updated_at": now}
	if errorCode != "" {
		values["error_code"] = errorCode
	}
	if errorMessage != "" {
		values["error_message"] = errorMessage
	}
	values["next_poll_at"] = next
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		out := tx.Model(&Task{}).Where("id=? AND status=?", taskID, TaskPolling).Updates(values)
		if out.Error != nil {
			return out.Error
		}
		if out.RowsAffected != 1 {
			return ErrConflict
		}
		if status == TaskPolling && next != nil {
			var task Task
			if err := tx.First(&task, taskID).Error; err != nil {
				return err
			}
			return r.schedulePollTx(ctx, tx, task, *next, now)
		}
		return nil
	})
}

func (r *Repository) CreateGeneratedFile(ctx context.Context, f *File) error {
	if r == nil || r.db == nil {
		return ErrUnavailable
	}
	if f == nil || f.RunID < 1 || f.ImportRequestKey == nil || f.SourceKind < SourceURL || f.SourceKind > SourceStream || f.TransferDeadline == nil || f.Metadata == nil {
		return ErrConflict
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(f)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing File
			findErr := tx.Where("import_request_key=?", *f.ImportRequestKey).First(&existing).Error
			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				findErr = tx.Where("run_id=? AND role=? AND ordinal=?", f.RunID, f.Role, f.Ordinal).First(&existing).Error
			}
			if findErr != nil {
				return findErr
			}
			if existing.RunID != f.RunID || existing.Role != f.Role || existing.Ordinal != f.Ordinal {
				return ErrConflict
			}
			if existing.ImportRequestKey == nil || *existing.ImportRequestKey != *f.ImportRequestKey {
				return ErrConflict
			}
			*f = existing
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return r.scheduleImportTx(ctx, tx, *f, f.CreatedAt)

	})
}

func (r *Repository) ClaimImport(ctx context.Context, now time.Time) (File, bool, error) {
	if r == nil || r.db == nil || now.IsZero() {
		return File{}, false, ErrUnavailable
	}
	var out File
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("transfer_status IN (?,?,?) AND (next_transfer_at IS NULL OR next_transfer_at<=?)", TransferQueued, TransferRetry, Transfering, now).
			Order("id").Limit(1).Take(&out).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		found = true
		if out.TransferDeadline == nil || !out.TransferDeadline.After(now) || out.TransferAttempts >= 3 || out.SourceRecoverable != 1 {
			out.TransferStatus = TransferFailed
			if err := tx.Model(&File{}).Where("id=? AND transfer_status IN (?,?,?)", out.ID, TransferQueued, TransferRetry, Transfering).Updates(map[string]any{"transfer_status": TransferFailed, "next_transfer_at": nil, "error_code": "TRANSFER_UNAVAILABLE", "error_message": "媒体已过期、重试已耗尽或来源不可恢复", "updated_at": now}).Error; err != nil {
				return err
			}
			return refreshRunTransferTx(ctx, tx, out.RunID, now)
		}
		if err := tx.Model(&File{}).Where("id=? AND transfer_status IN (?,?,?)", out.ID, TransferQueued, TransferRetry, Transfering).Updates(map[string]any{"transfer_status": Transfering, "transfer_attempts": gorm.Expr("transfer_attempts+1"), "next_transfer_at": importClaimDeadline(now, out.TransferDeadline), "updated_at": now}).Error; err != nil {
			return err
		}
		out.TransferStatus = Transfering
		out.TransferAttempts++
		return refreshRunTransferTx(ctx, tx, out.RunID, now)
	})
	return out, found, err
}

func (r *Repository) FinishImport(ctx context.Context, id, storageFileID int64, status int16, errorCode, errorMessage string, next *time.Time, now time.Time, mimeType *string, size *int64, attempts ...int) error {
	if id < 1 || storageFileID < 0 || now.IsZero() {
		return ErrConflict
	}
	values := map[string]any{"transfer_status": status, "storage_file_id": nil, "next_transfer_at": next, "updated_at": now}
	if storageFileID > 0 {
		values["storage_file_id"] = storageFileID
	}
	if mimeType != nil {
		values["mime_type"] = *mimeType
	}
	if size != nil {
		values["size_bytes"] = *size
	}
	if errorCode != "" {
		values["error_code"] = errorCode
	}
	if errorMessage != "" {
		values["error_message"] = errorMessage
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var file File
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND transfer_status=?", id, Transfering).First(&file).Error; err != nil {
			return err
		}
		query := tx.Model(&File{}).Where("id=? AND transfer_status=?", id, Transfering)
		if len(attempts) > 0 {
			query = query.Where("transfer_attempts=?", attempts[0])
		}
		out := query.Updates(values)
		if out.Error != nil {
			return out.Error
		}
		if out.RowsAffected != 1 {
			return ErrConflict
		}
		if err := refreshRunTransferTx(ctx, tx, file.RunID, now); err != nil {
			return err
		}
		if status == TransferRetry {
			file.TransferStatus = status
			file.NextTransferAt = next
			return r.scheduleImportTx(ctx, tx, file, now)
		}
		return nil
	})
}

func (r *Repository) scheduleImportTx(ctx context.Context, tx *gorm.DB, f File, now time.Time) error {
	if r.scheduler == nil || (f.TransferStatus != TransferQueued && f.TransferStatus != TransferRetry && f.TransferStatus != Transfering) {
		return nil
	}
	at := now
	if f.NextTransferAt != nil {
		at = *f.NextTransferAt
	}
	payload, _ := json.Marshal(map[string]any{"mediaFileId": f.ID})
	var metadata map[string]json.RawMessage
	_ = json.Unmarshal(f.Metadata, &metadata)
	key := fmt.Sprintf("ai.media.import:%d:%d:%s", f.ID, f.TransferAttempts, string(metadata["retrySequence"]))
	var exists int64
	if err := tx.Model(&scheduler.Job{}).Where("source_key=?", key).Count(&exists).Error; err != nil {
		return err
	}
	if exists > 0 {
		return reopenMediaJob(ctx, tx, key, at, now)
	}
	job := scheduler.Job{TaskType: "ai.media.import", Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: key, ScheduledAt: at, AvailableAt: at, Status: scheduler.JobQueued, MaxAttempts: 3, CreatedAt: now, UpdatedAt: now}
	return r.scheduler.CreateJobTx(ctx, tx, &job)
}

// RecoverDue restores durable transport for bounded, unfinished imports. It never submits generation.
func (r *Repository) RecoverDue(ctx context.Context, now time.Time, limit int) (int, error) {
	if r == nil || r.db == nil || limit < 1 || limit > 100 {
		return 0, ErrUnavailable
	}
	count := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var files []File
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("transfer_status IN (?,?,?) AND (next_transfer_at IS NULL OR next_transfer_at<=?)", TransferQueued, TransferRetry, Transfering, now).Order("id").Limit(limit).Find(&files).Error; err != nil {
			return err
		}
		for _, f := range files {
			if f.TransferStatus == Transfering {
				f.TransferStatus = TransferRetry
				if f.SourceRecoverable != 1 || f.TransferAttempts >= 3 || f.TransferDeadline == nil || !f.TransferDeadline.After(now) {
					f.TransferStatus = TransferFailed
				}
				if err := tx.Model(&File{}).Where("id=?", f.ID).Updates(map[string]any{"transfer_status": f.TransferStatus, "next_transfer_at": now, "updated_at": now, "error_code": "TRANSFER_INTERRUPTED", "error_message": "媒体转存被中断"}).Error; err != nil {
					return err
				}
				f.NextTransferAt = &now
				if err := refreshRunTransferTx(ctx, tx, f.RunID, now); err != nil {
					return err
				}
			}
			if err := r.scheduleImportTx(ctx, tx, f, now); err != nil {
				return err
			}
			count++
		}
		var tasks []Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status=? AND (next_poll_at IS NULL OR next_poll_at<=?)", TaskPolling, now).Order("id").Limit(limit).Find(&tasks).Error; err != nil {
			return err
		}
		for _, task := range tasks {
			if err := r.schedulePollTx(ctx, tx, task, now, now); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

func importClaimDeadline(now time.Time, deadline *time.Time) *time.Time {
	next := now.Add(30 * time.Second)
	if deadline != nil && deadline.Before(next) {
		next = *deadline
	}
	return &next
}
func (r *Repository) RetryRunImports(ctx context.Context, runID, userID, platformID int64, now time.Time) error {
	if r == nil || r.db == nil || r.scheduler == nil {
		return ErrUnavailable
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner struct{ ID int64 }
		if err := tx.Table("ai_run").Select("id").Where("id=? AND user_id=? AND platform_id=?", runID, userID, platformID).Take(&owner).Error; err != nil {
			return err
		}
		var files []File
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("run_id=? AND role=? AND source_kind=? AND source_recoverable=1 AND source_ciphertext IS NOT NULL AND encryption_key_id IS NOT NULL AND transfer_status IN (?,?) AND transfer_deadline_at>? AND (source_expires_at IS NULL OR source_expires_at>?)", runID, FileRoleGenerated, SourceURL, TransferRetry, TransferFailed, now, now).Order("id").Find(&files).Error; err != nil {
			return err
		}
		for _, f := range files {
			var meta map[string]any
			if json.Unmarshal(f.Metadata, &meta) != nil {
				return ErrConflict
			}
			sequence := float64(0)
			if v, ok := meta["retrySequence"].(float64); ok {
				sequence = v
			}
			meta["retrySequence"] = sequence + 1
			raw, _ := json.Marshal(meta)
			f.Metadata = raw
			f.TransferAttempts = 0
			f.TransferStatus = TransferQueued
			f.NextTransferAt = &now
			if err := tx.Model(&File{}).Where("id=?", f.ID).Updates(map[string]any{"metadata": raw, "transfer_attempts": 0, "transfer_status": TransferQueued, "next_transfer_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			if err := r.scheduleImportTx(ctx, tx, f, now); err != nil {
				return err
			}
		}
		return refreshRunTransferTx(ctx, tx, runID, now)
	})
}

func refreshRunTransferTx(ctx context.Context, tx *gorm.DB, runID int64, now time.Time) error {
	return tx.WithContext(ctx).Exec(`UPDATE ai_run SET transfer_status=CASE
 WHEN stats.total=0 THEN 0 WHEN stats.pending>0 THEN 1 WHEN stats.active>0 THEN 2
 WHEN stats.ready=stats.total THEN 3 WHEN stats.ready>0 THEN 4 ELSE 5 END,updated_at=?
 FROM (SELECT count(*) total,count(*) FILTER (WHERE transfer_status IN(0,3)) pending,count(*) FILTER (WHERE transfer_status=1) active,count(*) FILTER(WHERE transfer_status=2) ready FROM ai_media_file WHERE run_id=? AND role=2)stats WHERE ai_run.id=?`, now, runID, runID).Error
}

func (r *Repository) schedulePollTx(ctx context.Context, tx *gorm.DB, task Task, at, now time.Time) error {
	if r.scheduler == nil {
		return nil
	}
	key := fmt.Sprintf("ai.media.poll:%d:%d", task.ID, task.PollAttempts)
	var count int64
	if err := tx.Model(&scheduler.Job{}).Where("source_key=?", key).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return reopenMediaJob(ctx, tx, key, at, now)
	}
	payload, _ := json.Marshal(map[string]any{"mediaTaskId": task.ID})
	job := scheduler.Job{TaskType: "ai.media.poll", Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: key, ScheduledAt: at, AvailableAt: at, Status: scheduler.JobQueued, MaxAttempts: 3, CreatedAt: now, UpdatedAt: now}
	return r.scheduler.CreateJobTx(ctx, tx, &job)
}

func reopenMediaJob(ctx context.Context, tx *gorm.DB, key string, at, now time.Time) error {
	return tx.WithContext(ctx).Model(&scheduler.Job{}).Where("source_key=? AND status IN (?,?,?)", key, scheduler.JobCompleted, scheduler.JobFailed, scheduler.JobCanceled).Updates(map[string]any{"status": scheduler.JobQueued, "attempt_count": 0, "available_at": at, "scheduled_at": at, "error_class": nil, "last_error": nil, "updated_at": now}).Error
}
func (r *Repository) RenewImport(ctx context.Context, fileID int64, attempt int, now time.Time) error {
	out := r.db.WithContext(ctx).Model(&File{}).Where("id=? AND transfer_status=? AND transfer_attempts=? AND transfer_deadline_at>? AND next_transfer_at>?", fileID, Transfering, attempt, now, now).Updates(map[string]any{"next_transfer_at": gorm.Expr("LEAST(transfer_deadline_at,?)", now.Add(30*time.Second)), "updated_at": now})
	if out.Error != nil {
		return out.Error
	}
	if out.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
