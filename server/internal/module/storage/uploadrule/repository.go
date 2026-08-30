package uploadrule

import (
	"admin/server/internal/module/auth/platform"
	"admin/server/internal/module/storage/cosconfig"
	"admin/server/internal/shared/yesno"
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db} }

type UploadTarget struct {
	RuleID, PlatformID, CosConfigID                                 int64
	Code                                                            string
	MaxFileSizeBytes                                                int64
	AllowedExtensions, AllowedMimeTypes                             StringArray
	AccessMode                                                      string
	Bucket, Region, AppID                                           string
	Endpoint, BucketDomain, SecretIDCiphertext, SecretKeyCiphertext *string
}

func (r *Repository) PlatformEnabled(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("auth_platform").Where("id = ? AND is_enabled = 1 AND deleted_at IS NULL", id).Count(&count).Error
	return count == 1, err
}

func (r *Repository) Config(ctx context.Context, id int64) (cosconfig.Model, error) {
	var model cosconfig.Model
	err := r.db.WithContext(ctx).Where("id = ? AND is_enabled = 1 AND deleted_at IS NULL", id).Take(&model).Error
	return model, err
}

func (r *Repository) Count(ctx context.Context, q ListQuery) (int64, error) {
	var n int64
	db := r.db.WithContext(ctx).Model(&Model{}).Where("storage_upload_rule.deleted_at IS NULL")
	db = filter(db, q)
	if err := db.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}
func filter(db *gorm.DB, q ListQuery) *gorm.DB {
	if q.PlatformID != nil {
		db = db.Where("platform_id = ?", *q.PlatformID)
	}
	if q.CosConfigID != nil {
		db = db.Where("cos_config_id = ?", *q.CosConfigID)
	}
	if q.Keyword != "" {
		p := "%" + q.Keyword + "%"
		db = db.Where("code ILIKE ? OR name ILIKE ?", p, p)
	}
	if q.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *q.IsEnabled)
	}
	return db
}
func (r *Repository) List(ctx context.Context, q ListQuery) ([]RuleValue, error) {
	type row struct {
		ID                int64
		PlatformID        int64
		PlatformCode      string
		PlatformName      string
		Code              string
		Name              string
		CosConfigID       int64
		CosConfigName     string
		MaxFileSizeBytes  int64
		AllowedExtensions StringArray
		AllowedMimeTypes  StringArray
		AccessMode        string
		IsEnabled         int16
		Remark            string
		CreatedAt         time.Time
		UpdatedAt         time.Time
	}
	var rows []row
	err := filter(r.db.WithContext(ctx).Table("storage_upload_rule").Select("storage_upload_rule.*, p.code as platform_code,p.name as platform_name,c.name as cos_config_name").Joins("JOIN auth_platform p ON p.id=storage_upload_rule.platform_id").Joins("JOIN storage_cos_config c ON c.id=storage_upload_rule.cos_config_id"), q).Where("storage_upload_rule.deleted_at IS NULL").Order("storage_upload_rule.created_at DESC,storage_upload_rule.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]RuleValue, 0, len(rows))
	for _, v := range rows {
		out = append(out, RuleValue{ID: v.ID, PlatformID: v.PlatformID, PlatformCode: v.PlatformCode, PlatformName: v.PlatformName, Code: v.Code, Name: v.Name, CosConfigID: v.CosConfigID, CosConfigName: v.CosConfigName, MaxFileSizeBytes: v.MaxFileSizeBytes, AllowedExtensions: []string(v.AllowedExtensions), AllowedMimeTypes: []string(v.AllowedMimeTypes), AccessMode: v.AccessMode, IsEnabled: yesno.Value(v.IsEnabled), Remark: v.Remark, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt})
	}
	return out, nil
}
func (r *Repository) FindByID(ctx context.Context, id int64) (Model, error) {
	var m Model
	err := r.db.WithContext(ctx).Where("id=? AND deleted_at IS NULL", id).Take(&m).Error
	return m, err
}
func (r *Repository) LockByID(ctx context.Context, id int64) (Model, error) {
	var m Model
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&m).Error
	return m, err
}
func (r *Repository) LockActiveByPlatform(ctx context.Context, pid int64) ([]Model, error) {
	var rows []Model
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("platform_id=? AND deleted_at IS NULL", pid).Order("id ASC").Find(&rows).Error
	return rows, err
}
func (r *Repository) Create(ctx context.Context, m *Model) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		var p *pgconn.PgError
		if errors.As(err, &p) && p.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	return nil
}
func (r *Repository) Update(ctx context.Context, id int64, v map[string]any) error {
	res := r.db.WithContext(ctx).Model(&Model{}).Where("id=? AND deleted_at IS NULL", id).Updates(v)
	if res.Error != nil {
		var postgresError *pgconn.PgError
		if errors.As(res.Error, &postgresError) && postgresError.Code == "23505" {
			return ErrConflict
		}
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) MarkDeleted(ctx context.Context, id int64, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&Model{}).Where("id=? AND deleted_at IS NULL AND is_enabled=0", id).Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *Repository) FindPlatformOptions(ctx context.Context) ([]PlatformOption, error) {
	var out []PlatformOption
	err := r.db.WithContext(ctx).Model(&authplatform.Platform{}).Select("id,code,name,is_enabled").Where("deleted_at IS NULL AND is_enabled=1").Order("id").Scan(&out).Error
	return out, err
}
func (r *Repository) FindConfigSummaries(ctx context.Context) ([]ConfigSummary, error) {
	var out []ConfigSummary
	err := r.db.WithContext(ctx).Model(&cosconfig.Model{}).Select("id,name,bucket,region,is_enabled").Where("deleted_at IS NULL AND is_enabled=1").Order("id").Scan(&out).Error
	return out, err
}
func (r *Repository) FindUploadTarget(ctx context.Context, pid int64, code string) (UploadTarget, error) {
	var t UploadTarget
	err := r.db.WithContext(ctx).Table("storage_upload_rule r").Select("r.id as rule_id,r.platform_id,r.cos_config_id,r.code,r.max_file_size_bytes,r.allowed_extensions,r.allowed_mime_types,r.access_mode,c.bucket,c.region,c.app_id,c.endpoint,c.bucket_domain,c.secret_id_ciphertext,c.secret_key_ciphertext").Joins("JOIN storage_cos_config c ON c.id=r.cos_config_id").Where("r.platform_id=? AND r.code=? AND r.is_enabled=1 AND r.deleted_at IS NULL AND c.is_enabled=1 AND c.deleted_at IS NULL", pid, code).Take(&t).Error
	return t, err
}
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(NewRepository(tx)) })
}
