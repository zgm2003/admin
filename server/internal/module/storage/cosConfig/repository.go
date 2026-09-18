package cosconfig

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	cachegeneration "admin/server/internal/shared/cacheGeneration"
	"admin/server/internal/shared/yesno"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// cosConfigGenerationNamespace 必须服从缓存协议的全小写点分正则（Go 与 PostgreSQL CHECK 同一约束）。
const cosConfigGenerationNamespace = "storage.cosconfig"

const currentVersionColumns = "storage_cos_config.*, current_version.bucket AS bucket, current_version.region AS region, current_version.endpoint AS endpoint, current_version.bucket_domain AS bucket_domain"

const currentVersionJoin = "JOIN storage_cos_config_version AS current_version ON current_version.cos_config_id = storage_cos_config.id AND current_version.version = storage_cos_config.current_version"

// Repository 只访问 PostgreSQL：逻辑配置、不可变物理版本与 generation/outbox 事实。
type Repository struct {
	db          *gorm.DB
	generations *cachegeneration.Repository
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) SetGenerations(repository *cachegeneration.Repository) {
	r.generations = repository
}

func (r *Repository) configured() error {
	if r == nil || r.db == nil {
		return fmt.Errorf("COS config repository is not configured")
	}
	return nil
}

func (r *Repository) ValidateDependencies() error {
	if err := r.configured(); err != nil {
		return err
	}
	if r.generations == nil {
		return fmt.Errorf("COS config repository generation dependency is not configured")
	}
	return nil
}

func (r *Repository) generationScope(id int64) (cachegeneration.Scope, error) {
	return cachegeneration.NewScope(cosConfigGenerationNamespace, strconv.FormatInt(id, 10))
}

func quietDB(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Silent)})
}

func (r *Repository) Count(ctx context.Context, q ListQuery) (int64, error) {
	if err := r.configured(); err != nil {
		return 0, err
	}
	var n int64
	db := r.db.WithContext(ctx).Table("storage_cos_config").Joins(currentVersionJoin).Where("storage_cos_config.deleted_at IS NULL")
	db = filterList(db, q)
	if err := db.Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count configs: %w", err)
	}
	return n, nil
}

func filterList(db *gorm.DB, q ListQuery) *gorm.DB {
	if q.Keyword != "" {
		pattern := "%" + q.Keyword + "%"
		db = db.Where("storage_cos_config.name ILIKE ? OR current_version.bucket ILIKE ?", pattern, pattern)
	}
	if q.IsEnabled != nil {
		db = db.Where("storage_cos_config.is_enabled = ?", *q.IsEnabled)
	}
	return db
}

func (r *Repository) List(ctx context.Context, q ListQuery) ([]Current, error) {
	if err := r.configured(); err != nil {
		return nil, err
	}
	rows := []Current{}
	db := r.db.WithContext(ctx).Table("storage_cos_config").Select(currentVersionColumns).Joins(currentVersionJoin).Where("storage_cos_config.deleted_at IS NULL")
	db = filterList(db, q)
	if err := db.Order("storage_cos_config.created_at DESC,storage_cos_config.id DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}
	return rows, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (Current, error) {
	if err := r.configured(); err != nil {
		return Current{}, err
	}
	var row Current
	err := r.db.WithContext(ctx).Table("storage_cos_config").Select(currentVersionColumns).Joins(currentVersionJoin).
		Where("storage_cos_config.id = ? AND storage_cos_config.deleted_at IS NULL", id).Take(&row).Error
	if err != nil {
		return Current{}, err
	}
	return row, nil
}

// lockByID 只锁逻辑配置行；物理版本行不可变，无需加锁。
func lockByID(db *gorm.DB, id int64) (Current, error) {
	var row Current
	err := db.Clauses(clause.Locking{Strength: "UPDATE", Options: "OF storage_cos_config"}).
		Table("storage_cos_config").Select(currentVersionColumns).Joins(currentVersionJoin).
		Where("storage_cos_config.id = ? AND storage_cos_config.deleted_at IS NULL", id).Take(&row).Error
	if err != nil {
		return Current{}, err
	}
	return row, nil
}

// Create 在同一事务内写入逻辑配置、version 1 与 generation 1/outbox 1，并返回机械提交结果。
func (r *Repository) Create(ctx context.Context, model *Model, version Version) (cachegeneration.MutationResult, error) {
	if err := r.configured(); err != nil {
		return cachegeneration.MutationResult{}, err
	}
	if r.generations == nil {
		return cachegeneration.MutationResult{}, fmt.Errorf("COS config generation repository is not configured")
	}
	result := cachegeneration.MutationResult{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		quiet := quietDB(tx)
		model.CurrentVersion = 1
		if err := quiet.Create(model).Error; err != nil {
			return mapNameConflict(err, "create COS config")
		}
		version.CosConfigID = model.ID
		version.Version = 1
		if err := quiet.Create(&version).Error; err != nil {
			return fmt.Errorf("insert COS config version: %w", err)
		}
		scope, err := r.generationScope(model.ID)
		if err != nil {
			return err
		}
		event, err := r.generations.InitializeTx(ctx, tx, scope, version.CreatedAt)
		if err != nil {
			return fmt.Errorf("initialize COS config generation: %w", err)
		}
		result = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	return result, nil
}

// RuntimeFacts 读取运行时事实：逻辑配置（含软删行）与全部不可变物理版本，供 generation 快照回源使用。
func (r *Repository) RuntimeFacts(ctx context.Context, id int64) (RuntimeConfig, error) {
	if err := r.configured(); err != nil {
		return RuntimeConfig{}, err
	}
	var logical struct {
		ID                  int64
		AppID               string
		SecretIDCiphertext  string
		SecretKeyCiphertext string
		CurrentVersion      int64
		IsEnabled           yesno.Value
		Deleted             bool
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id, app_id, secret_id_ciphertext, secret_key_ciphertext, current_version, is_enabled, deleted_at IS NOT NULL AS deleted
		 FROM storage_cos_config WHERE id = ?`, id).Scan(&logical).Error; err != nil {
		return RuntimeConfig{}, fmt.Errorf("read COS config runtime facts: %w", err)
	}
	if logical.ID == 0 {
		return RuntimeConfig{}, gorm.ErrRecordNotFound
	}
	var rows []struct {
		Version      int64
		Bucket       string
		Region       string
		Endpoint     *string
		BucketDomain *string
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT version, bucket, region, endpoint, bucket_domain FROM storage_cos_config_version WHERE cos_config_id = ? ORDER BY version`, id).Scan(&rows).Error; err != nil {
		return RuntimeConfig{}, fmt.Errorf("read COS config versions: %w", err)
	}
	versions := make([]RuntimeVersion, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, RuntimeVersion{Version: row.Version, Bucket: row.Bucket, Region: row.Region, Endpoint: row.Endpoint, BucketDomain: row.BucketDomain})
	}
	return RuntimeConfig{
		ID: logical.ID, AppID: logical.AppID, SecretIDCiphertext: logical.SecretIDCiphertext, SecretKeyCiphertext: logical.SecretKeyCiphertext,
		CurrentVersion: logical.CurrentVersion, IsEnabled: logical.IsEnabled, Deleted: logical.Deleted, Versions: versions,
	}, nil
}

// UpdateValues 是规范化后的目标值；Secret 为 nil 表示"不轮换"。
type UpdateValues struct {
	Name                string
	SecretIDCiphertext  *string
	SecretKeyCiphertext *string
	Bucket              string
	Region              string
	Endpoint            *string
	BucketDomain        *string
	Remark              string
}

// Update 在调用方可见的一个事务内锁定逻辑行、计算真实变化、必要时新增物理版本，最后推进 generation。
func (r *Repository) Update(ctx context.Context, id int64, values UpdateValues, baseGeneration int64, now time.Time) (cachegeneration.MutationResult, error) {
	if err := r.prepareMutation(id, baseGeneration); err != nil {
		return cachegeneration.MutationResult{}, err
	}
	scope, err := r.generationScope(id)
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	result := cachegeneration.MutationResult{}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockByID(tx.WithContext(ctx), id)
		if err != nil {
			return err
		}
		changes := map[string]any{}
		if values.Name != current.Name {
			changes["name"] = values.Name
		}
		if values.Remark != current.Remark {
			changes["remark"] = values.Remark
		}
		if values.SecretIDCiphertext != nil && *values.SecretIDCiphertext != current.SecretIDCiphertext {
			changes["secret_id_ciphertext"] = *values.SecretIDCiphertext
		}
		if values.SecretKeyCiphertext != nil && *values.SecretKeyCiphertext != current.SecretKeyCiphertext {
			changes["secret_key_ciphertext"] = *values.SecretKeyCiphertext
		}
		physical := values.Bucket != current.Bucket || values.Region != current.Region ||
			!sameOptionalString(values.Endpoint, current.Endpoint) || !sameOptionalString(values.BucketDomain, current.BucketDomain)
		if len(changes) == 0 && !physical {
			return nil
		}
		if physical {
			next := current.CurrentVersion + 1
			version := Version{CosConfigID: id, Version: next, Bucket: values.Bucket, Region: values.Region, Endpoint: values.Endpoint, BucketDomain: values.BucketDomain, CreatedAt: now, UpdatedAt: now}
			if err := quietDB(tx).Create(&version).Error; err != nil {
				return fmt.Errorf("insert COS config version: %w", err)
			}
			changes["current_version"] = next
		}
		changes["updated_at"] = now
		updateResult := quietDB(tx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).Updates(changes)
		if updateResult.Error != nil {
			return mapNameConflict(updateResult.Error, "update COS config")
		}
		if updateResult.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		event, err := r.generations.AdvanceTx(ctx, tx, scope, baseGeneration, now)
		if err != nil {
			return err
		}
		result = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	return result, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id int64, value yesno.Value, baseGeneration int64, now time.Time) (cachegeneration.MutationResult, error) {
	if err := r.prepareMutation(id, baseGeneration); err != nil {
		return cachegeneration.MutationResult{}, err
	}
	scope, err := r.generationScope(id)
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	result := cachegeneration.MutationResult{}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockByID(tx.WithContext(ctx), id)
		if err != nil {
			return err
		}
		if current.IsEnabled == value {
			return nil
		}
		updateResult := quietDB(tx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).
			Updates(map[string]any{"is_enabled": value, "updated_at": now})
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		event, err := r.generations.AdvanceTx(ctx, tx, scope, baseGeneration, now)
		if err != nil {
			return err
		}
		result = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	return result, nil
}

func (r *Repository) MarkDeleted(ctx context.Context, id int64, baseGeneration int64, now time.Time) (cachegeneration.MutationResult, error) {
	if err := r.prepareMutation(id, baseGeneration); err != nil {
		return cachegeneration.MutationResult{}, err
	}
	scope, err := r.generationScope(id)
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	result := cachegeneration.MutationResult{}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockByID(tx.WithContext(ctx), id); err != nil {
			return err
		}
		updateResult := tx.WithContext(ctx).Model(&Model{}).Where("id = ? AND deleted_at IS NULL", id).
			Updates(map[string]any{"is_enabled": yesno.No, "deleted_at": now, "updated_at": now})
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		event, err := r.generations.AdvanceTx(ctx, tx, scope, baseGeneration, now)
		if err != nil {
			return err
		}
		result = cachegeneration.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	if err != nil {
		return cachegeneration.MutationResult{}, err
	}
	return result, nil
}

// CountRuleReferences 统计引用该配置的上传规则，包含已禁用与已软删规则：只要被引用过就不允许删除。
func (r *Repository) CountRuleReferences(ctx context.Context, id int64) (int64, error) {
	if err := r.configured(); err != nil {
		return 0, err
	}
	var n int64
	if err := r.db.WithContext(ctx).Table("storage_upload_rule").Where("cos_config_id = ?", id).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count rule references: %w", err)
	}
	return n, nil
}

func (r *Repository) prepareMutation(id, baseGeneration int64) error {
	if err := r.configured(); err != nil {
		return err
	}
	if r.generations == nil {
		return fmt.Errorf("COS config generation repository is not configured")
	}
	if id < 1 {
		return fmt.Errorf("COS config id is invalid")
	}
	if baseGeneration < 1 {
		return fmt.Errorf("COS config base generation is invalid")
	}
	return nil
}

func mapNameConflict(err error, action string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "ux_storage_cos_config_name_active" {
		return fmt.Errorf("%s: %w", action, ErrNameConflict)
	}
	return err
}

func sameOptionalString(left, right *string) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}
