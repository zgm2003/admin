package file

import (
	"admin/server/internal/module/auth/login"
	"admin/server/internal/module/system/scheduler"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repository struct {
	db        *gorm.DB
	scheduler *scheduler.Repository
}

func NewRepository(db *gorm.DB, jobs *scheduler.Repository) *Repository {
	return &Repository{db: db, scheduler: jobs}
}
func owner(db *gorm.DB, id auth.Identity) *gorm.DB {
	return db.Where("platform_id=? AND user_id=?", id.PlatformID, id.UserID)
}
func (r *Repository) Create(ctx context.Context, m *Model) error {
	return r.db.WithContext(ctx).Create(m).Error
}
func (r *Repository) FindObject(ctx context.Context, key string) (Model, error) {
	var m Model
	e := r.db.WithContext(ctx).Unscoped().Where("object_key=?", key).Take(&m).Error
	if e == nil && m.ObjectKey != key {
		return Model{}, ErrConflict
	}
	return m, e
}
func (r *Repository) IsCurrentAvatar(ctx context.Context, userID int64, key string) (bool, error) {
	var n int64
	e := r.db.WithContext(ctx).Table("user_profile").Where("user_id=? AND avatar=?", userID, key).Count(&n).Error
	return n == 1, e
}
func (r *Repository) Find(ctx context.Context, id auth.Identity, fileID int64) (Model, error) {
	var m Model
	e := owner(r.db.WithContext(ctx), id).Where("id=?", fileID).Take(&m).Error
	return m, e
}
func (r *Repository) ByRequest(ctx context.Context, id auth.Identity, key string) (Model, error) {
	var m Model
	e := owner(r.db.WithContext(ctx), id).Where("request_key=?", key).Take(&m).Error
	return m, e
}
func (r *Repository) ReserveETag(ctx context.Context, id auth.Identity, fileID int64, etag string, now time.Time) (Model, error) {
	e := owner(r.db.WithContext(ctx).Model(&Model{}), id).Where("id=? AND status=1 AND etag=''", fileID).Updates(map[string]any{"etag": etag, "updated_at": now}).Error
	if e != nil {
		return Model{}, e
	}
	return r.Find(ctx, id, fileID)
}
func (r *Repository) RuleAccess(ctx context.Context, ruleID int64) (string, error) {
	var result string
	out := r.db.WithContext(ctx).Table("storage_upload_rule").Select("access_mode").Where("id=?", ruleID).Scan(&result)
	if out.Error != nil {
		return "", out.Error
	}
	if out.RowsAffected != 1 {
		return "", gorm.ErrRecordNotFound
	}
	return result, nil
}
func fileFilter(db *gorm.DB, q ListQuery) *gorm.DB {
	db = db.Where("kind=1 AND status=2")
	if q.Keyword != "" {
		db = db.Where("original_name ILIKE ?", "%"+q.Keyword+"%")
	}
	switch q.Category {
	case "image", "video", "audio":
		db = db.Where("mime_type LIKE ?", q.Category+"/%")
	case "document":
		db = db.Where("mime_type LIKE 'text/%' OR mime_type IN ?", []string{"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"})
	case "other":
		db = db.Where("mime_type NOT LIKE 'image/%' AND mime_type NOT LIKE 'video/%' AND mime_type NOT LIKE 'audio/%' AND mime_type NOT LIKE 'text/%' AND mime_type NOT IN ?", []string{"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"})
	}
	if q.Purpose == "avatar" {
		db = db.Where("mime_type LIKE 'image/%' AND size_bytes<=?", q.MaxSize)
		if len(q.MIMEs) > 0 {
			db = db.Where("mime_type IN ?", q.MIMEs)
		}
		if len(q.Extensions) > 0 {
			db = db.Where("lower(regexp_replace(original_name, '^.*\\.', '')) IN ?", q.Extensions)
		}
	}
	return db
}
func (r *Repository) List(ctx context.Context, id auth.Identity, q ListQuery) ([]Model, int64, error) {
	db := fileFilter(owner(r.db.WithContext(ctx).Model(&Model{}), id), q)
	var n int64
	if e := db.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	rows := []Model{}
	e := db.Order("created_at DESC,id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return rows, n, e
}
func (r *Repository) Finish(ctx context.Context, id int64, etag string, now time.Time, ready bool) error {
	values := map[string]any{"etag": etag, "completed_at": now, "updated_at": now}
	if ready {
		values["status"] = StatusReady
	}
	out := r.db.WithContext(ctx).Model(&Model{}).Where("id=? AND status=1", id).Updates(values)
	if out.Error != nil {
		return out.Error
	}
	if out.RowsAffected == 0 {
		return ErrConflict
	}
	return nil
}

var ErrConflict = errors.New("file state conflict")

func cleanupJob(ctx context.Context, jobs *scheduler.Repository, tx *gorm.DB, id int64, available, now time.Time) error {
	if jobs == nil {
		return errors.New("file cleanup scheduler unavailable")
	}
	payload, _ := json.Marshal(struct {
		FileID int64 `json:"fileId"`
	}{id})
	job := scheduler.Job{TaskType: CleanupTaskType, Payload: payload, TriggerSource: scheduler.TriggerBusiness, SourceKey: fmt.Sprintf("storageFile:%d:%d", id, now.UnixNano()), ScheduledAt: available, AvailableAt: available, Status: scheduler.JobScheduled, MaxAttempts: 10, CreatedAt: now, UpdatedAt: now}
	return jobs.CreateJobTx(ctx, tx, &job)
}
func (r *Repository) Delete(ctx context.Context, id auth.Identity, fileID int64, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m Model
		if e := owner(tx.Clauses(clause.Locking{Strength: "UPDATE"}), id).Where("id=? AND kind=1", fileID).Take(&m).Error; e != nil {
			return e
		}
		if m.Status == StatusCleanup {
			return nil
		}
		if m.Status != StatusReady {
			return ErrConflict
		}
		if e := tx.Model(&m).Updates(map[string]any{"status": StatusCleanup, "updated_at": now}).Error; e != nil {
			return e
		}
		return cleanupJob(ctx, r.scheduler, tx, m.ID, now, now)
	})
}

// AdoptAvatarTx participates in the caller's user-row-locked profile transaction.
func AdoptAvatarTx(ctx context.Context, tx *gorm.DB, id auth.Identity, fileID int64, key, requestKey string, now time.Time) (bool, error) {
	var m Model
	if e := owner(tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}), id).Where("id=? AND kind=2 AND object_key=? AND request_key=?", fileID, key, requestKey).Take(&m).Error; e != nil {
		return false, e
	}
	if m.Status == StatusReady {
		var avatar string
		if e := tx.Table("user_profile").Select("avatar").Where("user_id=?", id.UserID).Scan(&avatar).Error; e != nil {
			return false, e
		}
		if avatar != key {
			return false, ErrConflict
		}
		return true, nil
	}
	if m.Status != StatusPending || m.CompletedAt == nil {
		return false, ErrConflict
	}
	return false, tx.Model(&m).Updates(map[string]any{"status": StatusReady, "updated_at": now}).Error
}
func RetireAvatarTx(ctx context.Context, tx *gorm.DB, id auth.Identity, oldKey string, now time.Time) error {
	if oldKey == "" {
		return nil
	}
	var avatar string
	if e := tx.WithContext(ctx).Table("user_profile").Select("avatar").Where("user_id=?", id.UserID).Scan(&avatar).Error; e != nil {
		return e
	}
	if avatar == oldKey {
		return ErrConflict
	}
	out := tx.WithContext(ctx).Model(&Model{}).Where("user_id=? AND kind=2 AND object_key=? AND status=2", id.UserID, oldKey).Updates(map[string]any{"status": StatusCleanup, "updated_at": now})
	return out.Error
}

func (r *Repository) BeginRemote(ctx context.Context, id int64, now time.Time) error {
	out := r.db.WithContext(ctx).Model(&Model{}).Where("id=? AND status=1", id).Updates(map[string]any{"operation_lease_until": now.Add(time.Minute), "updated_at": now})
	if out.Error != nil {
		return out.Error
	}
	if out.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
func (r *Repository) ReopenCleanup(ctx context.Context, id int64, now time.Time) error {
	return r.db.WithContext(ctx).Unscoped().Model(&Model{}).Where("id=? AND status=3", id).Updates(map[string]any{"deleted_at": nil, "updated_at": now}).Error
}
