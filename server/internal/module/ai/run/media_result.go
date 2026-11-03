package run

import (
	pr "admin/server/internal/module/ai/providerRequest"
	"admin/server/internal/module/system/scheduler"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// MediaResultItem contains public-safe response metadata only. Source URLs and
// inline bytes deliberately have no field in this persistence contract.
type MediaResultItem struct {
	Ordinal       int    `json:"ordinal"`
	RevisedPrompt string `json:"revisedPrompt,omitempty"`
}

// MediaResultFile is the initial generated output fact owned by a confirmed
// provider result. It carries no media bytes or plaintext source locator.
type MediaResultFile struct {
	ID                int64           `gorm:"column:id;primaryKey"`
	RunID             int64           `gorm:"column:run_id"`
	Role              int16           `gorm:"column:role"`
	Ordinal           int             `gorm:"column:ordinal"`
	ImportRequestKey  string          `gorm:"column:import_request_key"`
	SourceKind        int16           `gorm:"column:source_kind"`
	SourceCiphertext  *string         `gorm:"column:source_ciphertext"`
	EncryptionKeyID   *string         `gorm:"column:encryption_key_id"`
	SourceRecoverable int16           `gorm:"column:source_recoverable"`
	TransferDeadline  time.Time       `gorm:"column:transfer_deadline_at"`
	MIMEType          *string         `gorm:"column:mime_type"`
	Metadata          json.RawMessage `gorm:"column:metadata;type:jsonb"`
	TransferStatus    int16           `gorm:"column:transfer_status"`
	TransferAttempts  int             `gorm:"column:transfer_attempts"`
	NextTransferAt    *time.Time      `gorm:"column:next_transfer_at"`
	ErrorCode         *string         `gorm:"column:error_code"`
	ErrorMessage      *string         `gorm:"column:error_message"`
	CreatedAt         time.Time       `gorm:"column:created_at"`
	UpdatedAt         time.Time       `gorm:"column:updated_at"`
}

func (MediaResultFile) TableName() string { return "ai_media_file" }

// FinishMediaResult commits the response/usage and every initial media fact in
// the same fenced transaction. A media insert or durable transport failure
// rolls back the response status, so known success can never lose its outputs.
func (r *Repository) FinishMediaResult(ctx context.Context, id int64, token string, requestID int64, header string, items []MediaResultItem, usage pr.MediaUsage, requestErr error, files []MediaResultFile, now time.Time) error {
	if len(files) == 0 || len(files) != len(items) || len(files) > 100 {
		return ErrInvalidSnapshot
	}
	for i := range files {
		if files[i].Ordinal != items[i].Ordinal {
			return ErrInvalidSnapshot
		}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return ErrInvalidSnapshot
	}
	return r.finishMediaRequest(ctx, id, token, requestID, header, raw, usage, requestErr, files, now)
}
func (r *Repository) createMediaResultFilesTx(ctx context.Context, tx *gorm.DB, runID int64, files []MediaResultFile, now time.Time) error {
	seen := map[int]bool{}
	for i := range files {
		f := &files[i]
		var metadata map[string]json.RawMessage
		_, keyErr := uuid.Parse(f.ImportRequestKey)
		if f.RunID != runID || f.Ordinal < 0 || seen[f.Ordinal] || keyErr != nil || f.SourceKind < 1 || f.SourceKind > 3 || f.TransferDeadline.IsZero() || f.TransferAttempts < 0 || len(f.Metadata) > 65536 || json.Unmarshal(f.Metadata, &metadata) != nil || metadata == nil || (f.TransferStatus != 0 && f.TransferStatus != 1 && f.TransferStatus != 4) {
			return ErrInvalidSnapshot
		}
		if (f.SourceCiphertext == nil) != (f.EncryptionKeyID == nil) {
			return ErrInvalidSnapshot
		}
		if f.SourceKind != 1 && (f.SourceRecoverable != 0 || f.SourceCiphertext != nil) {
			return ErrInvalidSnapshot
		}
		if f.SourceRecoverable != 0 && f.SourceRecoverable != 1 {
			return ErrInvalidSnapshot
		}
		if f.SourceRecoverable == 1 && (f.SourceKind != 1 || f.SourceCiphertext == nil || *f.SourceCiphertext == "" || *f.EncryptionKeyID == "") {
			return ErrInvalidSnapshot
		}
		if f.TransferStatus != 4 && ((f.SourceKind == 1 && f.SourceRecoverable != 1) || (f.SourceKind != 1 && f.TransferStatus != 1)) {
			return ErrInvalidSnapshot
		}
		if f.MIMEType != nil && len(*f.MIMEType) > 100 {
			return ErrInvalidSnapshot
		}
		seen[f.Ordinal] = true
		f.ID = 0
		f.Role = 2
		f.CreatedAt = now
		f.UpdatedAt = now
		result := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(f)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing MediaResultFile
			if err := tx.Where("run_id=? AND role=2 AND ordinal=?", runID, f.Ordinal).First(&existing).Error; err != nil {
				return err
			}
			if existing.ImportRequestKey != f.ImportRequestKey || existing.SourceKind != f.SourceKind {
				return ErrInvalidSnapshot
			}
			if f.SourceKind != 1 && f.TransferStatus == 4 && existing.TransferStatus == 1 {
				if err := tx.Model(&MediaResultFile{}).Where("id=? AND transfer_status=1", existing.ID).Updates(map[string]any{"transfer_status": 4, "next_transfer_at": nil, "error_code": f.ErrorCode, "error_message": f.ErrorMessage, "updated_at": now}).Error; err != nil {
					return err
				}
			}
			// A synchronous stream may already be ready or explicitly failed. Never
			// replace its observed status or extend its originally frozen deadline.
			continue
		}
		if f.TransferStatus == 4 {
			continue
		}
		if r.scheduler == nil {
			return ErrInvalidSnapshot
		}
		at := now
		if f.NextTransferAt != nil {
			at = *f.NextTransferAt
		}
		payload, _ := json.Marshal(map[string]any{"mediaFileId": f.ID})
		job := scheduler.Job{TaskType: "ai.media.import", Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: fmt.Sprintf("ai.media.import:%d:%d:", f.ID, f.TransferAttempts), ScheduledAt: at, AvailableAt: at, Status: scheduler.JobQueued, MaxAttempts: 3, CreatedAt: now, UpdatedAt: now}
		if err := r.scheduler.CreateJobTx(ctx, tx, &job); err != nil {
			return err
		}
	}
	return nil
}
