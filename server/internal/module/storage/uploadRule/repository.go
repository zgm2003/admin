package uploadrule

import (
	"admin/server/internal/module/permission/authPlatform"
	"admin/server/internal/module/storage/cosConfig"
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
	RuleID, PlatformID, CosConfigID     int64
	Code                                string
	MaxFileSizeBytes                    int64
	AllowedExtensions, AllowedMimeTypes StringArray
	AccessMode                          string
}

func (r *Repository) PlatformEnabled(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("permission_auth_platform").Where("id = ? AND is_enabled = 1 AND deleted_at IS NULL", id).Count(&count).Error
	return count == 1, err
}

// Config 读取启用的全局 COS 配置及其当前物理版本（Bucket 等物理字段已迁入 version 行）。
func (r *Repository) Config(ctx context.Context, id int64) (cosconfig.Current, error) {
	var row cosconfig.Current
	err := r.db.WithContext(ctx).Table("storage_cos_config").
		Select("storage_cos_config.*, current_version.bucket AS bucket, current_version.region AS region, current_version.endpoint AS endpoint, current_version.bucket_domain AS bucket_domain").
		Joins("JOIN storage_cos_config_version AS current_version ON current_version.cos_config_id = storage_cos_config.id AND current_version.version = storage_cos_config.current_version").
		Where("storage_cos_config.id = ? AND storage_cos_config.is_enabled = 1 AND storage_cos_config.deleted_at IS NULL", id).Take(&row).Error
	return row, err
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
		db = db.Where("storage_upload_rule.platform_id = ?", *q.PlatformID)
	}
	if q.CosConfigID != nil {
		db = db.Where("storage_upload_rule.cos_config_id = ?", *q.CosConfigID)
	}
	if q.Keyword != "" {
		p := "%" + q.Keyword + "%"
		db = db.Where("storage_upload_rule.name ILIKE ? OR EXISTS (SELECT 1 FROM storage_upload_rule_code k WHERE k.rule_id = storage_upload_rule.id AND k.deleted_at IS NULL AND k.code ILIKE ?)", p, p)
	}
	if q.IsEnabled != nil {
		db = db.Where("storage_upload_rule.is_enabled = ?", *q.IsEnabled)
	}
	return db
}
func (r *Repository) List(ctx context.Context, q ListQuery) ([]RuleValue, error) {
	type row struct {
		ID                int64
		PlatformID        int64
		PlatformCode      string
		PlatformName      string
		Codes             StringArray `gorm:"column:codes"`
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
	err := filter(r.db.WithContext(ctx).Table("storage_upload_rule").Select("storage_upload_rule.*, p.code as platform_code,p.name as platform_name,c.name as cos_config_name, COALESCE(array_agg(k.code ORDER BY k.id) FILTER (WHERE k.deleted_at IS NULL), '{}') as codes").Joins("JOIN permission_auth_platform p ON p.id=storage_upload_rule.platform_id").Joins("JOIN storage_cos_config c ON c.id=storage_upload_rule.cos_config_id").Joins("LEFT JOIN storage_upload_rule_code k ON k.rule_id=storage_upload_rule.id"), q).Where("storage_upload_rule.deleted_at IS NULL").Group("storage_upload_rule.id,p.code,p.name,c.name").Order("storage_upload_rule.created_at DESC,storage_upload_rule.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]RuleValue, 0, len(rows))
	for _, v := range rows {
		out = append(out, RuleValue{ID: v.ID, PlatformID: v.PlatformID, PlatformCode: v.PlatformCode, PlatformName: v.PlatformName, Codes: []string(v.Codes), Name: v.Name, CosConfigID: v.CosConfigID, CosConfigName: v.CosConfigName, MaxFileSizeBytes: v.MaxFileSizeBytes, AllowedExtensions: []string(v.AllowedExtensions), AllowedMimeTypes: []string(v.AllowedMimeTypes), AccessMode: v.AccessMode, IsEnabled: yesno.Value(v.IsEnabled), Remark: v.Remark, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt})
	}
	return out, nil
}
func (r *Repository) FindByID(ctx context.Context, id int64) (Model, error) {
	var m Model
	err := r.db.WithContext(ctx).Where("id=? AND deleted_at IS NULL", id).Take(&m).Error
	if err == nil {
		err = r.loadCodes(ctx, &m)
	}
	return m, err
}
func (r *Repository) LockByID(ctx context.Context, id int64) (Model, error) {
	var m Model
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&m).Error
	if err == nil {
		err = r.loadCodes(ctx, &m)
	}
	return m, err
}
func (r *Repository) LockActiveByPlatform(ctx context.Context, pid int64) ([]Model, error) {
	var rows []Model
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("platform_id=? AND deleted_at IS NULL", pid).Order("id ASC").Find(&rows).Error
	return rows, err
}
func (r *Repository) Create(ctx context.Context, m *Model, codes []string) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		var p *pgconn.PgError
		if errors.As(err, &p) && p.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	rows := make([]RuleCode, 0, len(codes))
	for _, code := range codes {
		rows = append(rows, RuleCode{RuleID: m.ID, Code: code, CreatedAt: m.CreatedAt, UpdatedAt: m.CreatedAt})
	}
	if err := r.db.WithContext(ctx).Create(&rows).Error; err != nil {
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
func (r *Repository) ReplaceCodes(ctx context.Context, ruleID int64, current, next []string, now time.Time) error {
	currentSet := make(map[string]struct{}, len(current))
	nextSet := make(map[string]struct{}, len(next))
	for _, code := range current {
		currentSet[code] = struct{}{}
	}
	for _, code := range next {
		nextSet[code] = struct{}{}
	}
	removed := make([]string, 0)
	for _, code := range current {
		if _, keep := nextSet[code]; !keep {
			removed = append(removed, code)
		}
	}
	if len(removed) > 0 {
		result := r.db.WithContext(ctx).Model(&RuleCode{}).Where("rule_id=? AND code IN ? AND deleted_at IS NULL", ruleID, removed).
			Updates(map[string]any{"deleted_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(removed)) {
			return gorm.ErrRecordNotFound
		}
	}
	added := make([]RuleCode, 0)
	for _, code := range next {
		if _, exists := currentSet[code]; !exists {
			added = append(added, RuleCode{RuleID: ruleID, Code: code, CreatedAt: now, UpdatedAt: now})
		}
	}
	if len(added) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&added).Error; err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return ErrConflict
		}
		return err
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
func (r *Repository) MarkCodesDeleted(ctx context.Context, ruleID int64, now time.Time) error {
	return r.db.WithContext(ctx).Model(&RuleCode{}).Where("rule_id=? AND deleted_at IS NULL", ruleID).Update("deleted_at", now).Error
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
	err := r.db.WithContext(ctx).Table("storage_upload_rule r").
		Select("r.id as rule_id,r.platform_id,r.cos_config_id,k.code,r.max_file_size_bytes,r.allowed_extensions,r.allowed_mime_types,r.access_mode").
		Joins("JOIN storage_upload_rule_code k ON k.rule_id=r.id AND k.deleted_at IS NULL").
		Where("r.platform_id=? AND k.code=? AND r.is_enabled=1 AND r.deleted_at IS NULL", pid, code).Take(&t).Error
	return t, err
}

// RuleRoute 只读取创建后不可变的对象物理路由；软删规则仍需服务其历史对象。
func (r *Repository) RuleRoute(ctx context.Context, ruleID int64) (ObjectRoute, error) {
	var route ObjectRoute
	err := r.db.WithContext(ctx).Unscoped().Model(&Model{}).
		Select("id AS rule_id, platform_id, cos_config_id, access_mode").
		Where("id = ?", ruleID).Take(&route).Error
	return route, err
}

// LockPlatform 按平台行锁协议（与菜单服务一致）确认认证平台仍然活动。
func (r *Repository) LockPlatform(ctx context.Context, id int64) error {
	var ids []int64
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM permission_auth_platform WHERE id = ? AND is_enabled = 1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&ids).Error; err != nil {
		return err
	}
	if len(ids) != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// LockConfig 锁定并确认目标 COS 配置活动（物理字段来自当前版本）。
func (r *Repository) LockConfig(ctx context.Context, id int64) (cosconfig.Current, error) {
	var row cosconfig.Current
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE", Options: "OF storage_cos_config"}).
		Table("storage_cos_config").
		Select("storage_cos_config.*, current_version.bucket AS bucket, current_version.region AS region, current_version.endpoint AS endpoint, current_version.bucket_domain AS bucket_domain").
		Joins("JOIN storage_cos_config_version AS current_version ON current_version.cos_config_id = storage_cos_config.id AND current_version.version = storage_cos_config.current_version").
		Where("storage_cos_config.id = ? AND storage_cos_config.is_enabled = 1 AND storage_cos_config.deleted_at IS NULL", id).Take(&row).Error
	return row, err
}

// LockEnabledOtherRules 按 id 顺序锁定同平台其它活动规则；部分唯一索引仍是最终兜底。
func (r *Repository) LockEnabledOtherRules(ctx context.Context, platformID, keepID int64) ([]int64, error) {
	var ids []int64
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id FROM storage_upload_rule WHERE platform_id = ? AND is_enabled = 1 AND deleted_at IS NULL AND id <> ? ORDER BY id FOR UPDATE`,
		platformID, keepID).Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// DisableRules 停用指定规则集合（同平台互斥启用的原子结果）。
func (r *Repository) DisableRules(ctx context.Context, ids []int64, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&Model{}).Where("id IN ? AND deleted_at IS NULL", ids).
		Updates(map[string]any{"is_enabled": yesno.No, "updated_at": now}).Error
}

func (r *Repository) loadCodes(ctx context.Context, m *Model) error {
	var rows []RuleCode
	if err := r.db.WithContext(ctx).Where("rule_id=? AND deleted_at IS NULL", m.ID).Order("id").Find(&rows).Error; err != nil {
		return err
	}
	m.Codes = make([]string, 0, len(rows))
	for _, row := range rows {
		m.Codes = append(m.Codes, row.Code)
	}
	return nil
}
func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(NewRepository(tx)) })
}
