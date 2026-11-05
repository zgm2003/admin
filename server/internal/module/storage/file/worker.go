package file

import (
	"admin/server/internal/module/system/scheduler"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const CleanupTaskType = "storage.file.cleanup"
const ScanTaskType = "storage.file.scan"

func CleanupDefinition(s *Service) scheduler.TaskDefinition {
	return scheduler.TaskDefinition{Type: CleanupTaskType, DisplayName: "文件对象清理", DisplayNameEnglish: "File object cleanup", DefaultParams: json.RawMessage(`{}`), Queue: "maintenance", Timeout: 30 * time.Second, MaxAttempts: 10, Backoff: func(attempt int) time.Duration { return time.Duration(attempt) * time.Minute }, ValidateParams: func(raw json.RawMessage) error {
		var v struct {
			FileID int64 `json:"fileId"`
		}
		if e := json.Unmarshal(raw, &v); e != nil {
			return e
		}
		if v.FileID < 1 {
			return errors.New("file cleanup id invalid")
		}
		return nil
	}, Execute: func(ctx context.Context, execution scheduler.ExecutionContext) error {
		var v struct {
			FileID int64 `json:"fileId"`
		}
		if e := json.Unmarshal(execution.Payload, &v); e != nil {
			return e
		}
		return s.Cleanup(ctx, v.FileID)
	}}
}
func ScanDefinition(s *Service) scheduler.TaskDefinition {
	return scheduler.TaskDefinition{Type: ScanTaskType, DisplayName: "文件清理巡检", DisplayNameEnglish: "File cleanup scan", BuiltinKey: ScanTaskType, DefaultParams: json.RawMessage(`{}`), Queue: "maintenance", Timeout: 60 * time.Second, MaxAttempts: 3, Backoff: func(attempt int) time.Duration { return time.Duration(attempt) * time.Minute }, ValidateParams: func(raw json.RawMessage) error {
		var v map[string]json.RawMessage
		if e := json.Unmarshal(raw, &v); e != nil {
			return e
		}
		if len(v) != 0 {
			return errors.New("file scan parameters invalid")
		}
		return nil
	}, Execute: func(ctx context.Context, _ scheduler.ExecutionContext) error { return s.Scan(ctx) }}
}
func (r *Repository) CleanupCandidates(ctx context.Context, now time.Time, after int64) ([]Model, error) {
	rows := []Model{}
	e := r.db.WithContext(ctx).Where("id>? AND status IN (1,3) AND (status=3 OR credential_expires_at<=?) AND (operation_lease_until IS NULL OR operation_lease_until<=?)", after, now.Add(-time.Minute), now.Add(-time.Minute)).Order("id").Limit(100).Find(&rows).Error
	return rows, e
}
func (r *Repository) TemporaryCandidates(ctx context.Context, now time.Time, after int64) ([]Model, error) {
	rows := []Model{}
	e := r.db.WithContext(ctx).Where("id>? AND status=2 AND upload_object_key IS NOT NULL AND credential_expires_at<=? AND (operation_lease_until IS NULL OR operation_lease_until<=?)", after, now.Add(-time.Minute), now.Add(-time.Minute)).Order("id").Limit(100).Find(&rows).Error
	return rows, e
}
func (r *Repository) ClaimCleanup(ctx context.Context, id int64, now time.Time) (Model, error) {
	var m Model
	e := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Match profile adoption's user -> file lock order.
		var before Model
		if e := tx.Where("id=?", id).Take(&before).Error; e != nil {
			return e
		}
		if before.Kind == KindAvatar {
			var user int64
			if e := tx.Raw("SELECT id FROM user_account WHERE id=? FOR UPDATE", before.UserID).Scan(&user).Error; e != nil {
				return e
			}
			if user == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id).Take(&m).Error; e != nil {
			return e
		}
		if m.Kind == KindAvatar {
			var avatar string
			if e := tx.Table("user_profile").Select("avatar").Where("user_id=?", m.UserID).Scan(&avatar).Error; e != nil {
				return e
			}
			if avatar == m.ObjectKey {
				return ErrConflict
			}
		}
		if m.Status == StatusReady {
			return ErrConflict
		}
		if m.OperationLeaseUntil != nil && now.Before(m.OperationLeaseUntil.Add(time.Minute)) {
			return ErrConflict
		}
		if m.UploadObjectKey != nil && m.CredentialExpiresAt != nil && now.Before(m.CredentialExpiresAt.Add(time.Minute)) {
			return ErrConflict
		}
		if m.Status == StatusPending {
			if m.CredentialExpiresAt == nil || now.Before(m.CredentialExpiresAt.Add(time.Minute)) {
				return ErrConflict
			}
			if e := tx.Model(&m).Updates(map[string]any{"status": StatusCleanup, "updated_at": now}).Error; e != nil {
				return e
			}
			m.Status = StatusCleanup
		}
		return nil
	})
	return m, e
}
func (r *Repository) MarkCleaned(ctx context.Context, id int64, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id=? AND status=3", id).Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error
}
func (r *Repository) ClearTemporary(ctx context.Context, id int64, now time.Time) error {
	return r.db.WithContext(ctx).Model(&Model{}).Where("id=? AND status=2", id).Updates(map[string]any{"upload_object_key": nil, "updated_at": now}).Error
}
func (s *Service) Cleanup(ctx context.Context, fileID int64) error {
	m, e := s.repository.ClaimCleanup(ctx, fileID, s.now().UTC())
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil
	}
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	credentials, _, e := s.physical(ctx, m)
	if e != nil {
		return e
	}
	if e = s.cos.DeleteObject(ctx, credentials, m.ObjectKey); e != nil {
		return e
	}
	if m.UploadObjectKey != nil {
		if e = s.cos.DeleteObject(ctx, credentials, *m.UploadObjectKey); e != nil {
			return e
		}
	}
	return s.repository.MarkCleaned(ctx, m.ID, s.now().UTC())
}
func (s *Service) cleanupTemporary(ctx context.Context, m Model) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	credentials, _, e := s.physical(ctx, m)
	if e != nil {
		return e
	}
	if m.UploadObjectKey != nil {
		if e = s.cos.DeleteObject(ctx, credentials, *m.UploadObjectKey); e != nil {
			return e
		}
	}
	return s.repository.ClearTemporary(ctx, m.ID, s.now().UTC())
}
func (s *Service) Scan(ctx context.Context) error {
	var failures []error
	for _, temporary := range []bool{false, true} {
		after := int64(0)
		for batch := 0; batch < 10; batch++ {
			var rows []Model
			var e error
			if temporary {
				rows, e = s.repository.TemporaryCandidates(ctx, s.now().UTC(), after)
			} else {
				rows, e = s.repository.CleanupCandidates(ctx, s.now().UTC(), after)
			}
			if e != nil {
				return errors.Join(append(failures, e)...)
			}
			for _, m := range rows {
				after = m.ID
				if temporary {
					e = s.cleanupTemporary(ctx, m)
				} else {
					e = s.Cleanup(ctx, m.ID)
				}
				if e != nil && !errors.Is(e, ErrConflict) {
					failures = append(failures, fmt.Errorf("file %d cleanup failed: %w", m.ID, e))
				}
			}
			if len(rows) < 100 {
				break
			}
		}
	}
	return errors.Join(failures...)
}
