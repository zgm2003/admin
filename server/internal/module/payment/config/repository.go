package config

import (
	cache "admin/server/internal/shared/configCache"
	"admin/server/internal/shared/yesno"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"reflect"
	"time"
)

var ErrIdentityConflict = errors.New("payment application already exists")
var configScope = cache.Scope{Namespace: "payment.config", ScopeKey: "global"}

type Repository struct {
	db          *gorm.DB
	generations *cache.Repository
}

func NewRepository(db *gorm.DB) *Repository              { return &Repository{db: db} }
func (r *Repository) SetGenerations(v *cache.Repository) { r.generations = v }
func (r *Repository) ValidateDependencies() error {
	if r == nil || r.db == nil || r.generations == nil {
		return fmt.Errorf("payment repository dependencies missing")
	}
	return nil
}
func quiet(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Silent)})
}
func mapDatabaseError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "ux_payment_config_app_active" {
		return ErrIdentityConflict
	}
	return err
}
func filter(db *gorm.DB, q ListQuery) *gorm.DB {
	if q.Keyword != "" {
		db = db.Where("name ILIKE ? OR app_id ILIKE ? OR seller_id ILIKE ?", "%"+q.Keyword+"%", "%"+q.Keyword+"%", "%"+q.Keyword+"%")
	}
	if q.IsEnabled != nil {
		db = db.Where("is_enabled=?", *q.IsEnabled)
	}
	return db
}
func (r *Repository) Count(ctx context.Context, q ListQuery) (int64, error) {
	var n int64
	err := filter(r.db.WithContext(ctx).Model(&Model{}), q).Count(&n).Error
	return n, err
}
func (r *Repository) List(ctx context.Context, q ListQuery) ([]Current, error) {
	rows := []struct {
		Model
		VersionFields Version `gorm:"embedded;embeddedPrefix:credential_"`
	}{}
	db := r.db.WithContext(ctx).Table("payment_config").Select("payment_config.*,v.config_id AS credential_config_id,v.version AS credential_version,v.private_key_ciphertext AS credential_private_key_ciphertext,v.app_cert_pem AS credential_app_cert_pem,v.alipay_cert_pem AS credential_alipay_cert_pem,v.root_cert_pem AS credential_root_cert_pem,v.notify_url AS credential_notify_url,v.pay_methods AS credential_pay_methods,v.created_at AS credential_created_at,v.updated_at AS credential_updated_at").Joins("JOIN payment_config_version v ON v.config_id=payment_config.id AND v.version=payment_config.current_version").Where("payment_config.deleted_at IS NULL")
	if err := filter(db, q).Order("payment_config.sort_order,payment_config.id").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]Current, 0, len(rows))
	for _, row := range rows {
		result = append(result, Current{row.Model, row.VersionFields})
	}
	return result, nil
}
func findCurrent(ctx context.Context, db *gorm.DB, id int64, lock bool) (Current, error) {
	var m Model
	query := db.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&m, id).Error; err != nil {
		return Current{}, err
	}
	var v Version
	if err := db.WithContext(ctx).Where("config_id=? AND version=?", id, m.CurrentVersion).Take(&v).Error; err != nil {
		return Current{}, err
	}
	return Current{m, v}, nil
}
func (r *Repository) FindByID(ctx context.Context, id int64) (Current, error) {
	return findCurrent(ctx, r.db, id, false)
}

// Runtime facts are bounded by the write-side catalog cap; historical credentials are loaded one version at a time.
func (r *Repository) CatalogFacts(ctx context.Context) ([]CatalogItem, error) {
	return catalogFacts(ctx, r.db)
}
func catalogFacts(ctx context.Context, db *gorm.DB) ([]CatalogItem, error) {
	rows := []CatalogItem{}
	err := db.WithContext(ctx).Table("payment_config").Select("id,name,provider,environment,app_id,seller_id,current_version,sort_order,is_enabled").Where("deleted_at IS NULL").Order("sort_order,id").Limit(MaxSnapshotBytes/128 + 1).Scan(&rows).Error
	if len(rows) > MaxSnapshotBytes/128 {
		return nil, fmt.Errorf("payment catalog size limit exceeded")
	}
	return rows, err
}
func (r *Repository) CredentialFacts(ctx context.Context, id, version int64) (BoundCredential, error) {
	var m Model
	if err := r.db.WithContext(ctx).Unscoped().First(&m, id).Error; err != nil {
		return BoundCredential{}, err
	}
	var v Version
	if err := r.db.WithContext(ctx).Where("config_id=? AND version=?", id, version).Take(&v).Error; err != nil {
		return BoundCredential{}, err
	}
	cv := CredentialVersion{v.ConfigID, v.Version, v.PrivateKeyCiphertext, v.AppCertPEM, v.AlipayCertPEM, v.RootCertPEM, v.NotifyURL, v.PayMethods, v.CreatedAt, v.UpdatedAt}
	return BoundCredential{ConfigID: id, Provider: m.Provider, Environment: m.Environment, AppID: m.AppID, SellerID: m.SellerID, Version: cv}, nil
}
func (r *Repository) Create(ctx context.Context, m *Model, v Version, base int64) (cache.MutationResult, error) {
	return r.mutation(ctx, base, m.CreatedAt, func(tx *gorm.DB) (bool, error) {
		m.CurrentVersion = 1
		if err := tx.Create(m).Error; err != nil {
			return false, mapDatabaseError(err)
		}
		v.ConfigID = m.ID
		v.Version = 1
		url, err := resolveNotifyURL(v.NotifyURL, m.ID, 1)
		if err != nil {
			return false, err
		}
		v.NotifyURL = url
		if err = tx.Create(&v).Error; err != nil {
			return false, err
		}
		if err = checkCatalogSize(ctx, tx); err != nil {
			return false, err
		}
		return true, nil
	})
}
func (r *Repository) mutation(ctx context.Context, base int64, now time.Time, apply func(*gorm.DB) (bool, error)) (cache.MutationResult, error) {
	if err := r.ValidateDependencies(); err != nil {
		return cache.MutationResult{}, err
	}
	var result cache.MutationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		changed, err := apply(quiet(tx))
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		event, err := r.generations.AdvanceTx(ctx, tx, configScope, base, now)
		if err != nil {
			return err
		}
		result = cache.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	return result, err
}
func (r *Repository) UpdateMetadata(ctx context.Context, id int64, v MetadataInput, base int64, now time.Time) (cache.MutationResult, error) {
	return r.mutation(ctx, base, now, func(tx *gorm.DB) (bool, error) {
		current, err := findCurrent(ctx, tx, id, true)
		if err != nil {
			return false, err
		}
		if current.Config.Name == v.Name && current.Config.SortOrder == v.SortOrder {
			return false, nil
		}
		if err = tx.Model(&Model{}).Where("id=?", id).Updates(map[string]any{"name": v.Name, "sort_order": v.SortOrder, "updated_at": now}).Error; err != nil {
			return false, err
		}
		return true, checkCatalogSize(ctx, tx)
	})
}
func (r *Repository) PublishVersion(ctx context.Context, id int64, v Version, base int64, now time.Time) (cache.MutationResult, error) {
	return r.mutation(ctx, base, now, func(tx *gorm.DB) (bool, error) {
		current, err := findCurrent(ctx, tx, id, true)
		if err != nil {
			return false, err
		}
		old := current.Credential
		template := v.NotifyURL
		if template == old.NotifyURL {
			template, err = notifyTemplate(old.NotifyURL, id, old.Version)
			if err != nil {
				return false, err
			}
		}
		oldURL, err := resolveNotifyURL(template, id, old.Version)
		if err != nil {
			return false, err
		}
		if old.PrivateKeyCiphertext == v.PrivateKeyCiphertext && old.AppCertPEM == v.AppCertPEM && old.AlipayCertPEM == v.AlipayCertPEM && old.RootCertPEM == v.RootCertPEM && oldURL == old.NotifyURL && reflect.DeepEqual(old.PayMethods, v.PayMethods) {
			return false, nil
		}
		if old.Version == int64(^uint64(0)>>1) {
			return false, fmt.Errorf("credential version overflow")
		}
		v.ConfigID = id
		v.Version = old.Version + 1
		v.NotifyURL, err = resolveNotifyURL(template, id, v.Version)
		if err != nil {
			return false, err
		}
		v.CreatedAt = now
		v.UpdatedAt = now
		if err = tx.Create(&v).Error; err != nil {
			return false, err
		}
		if err = tx.Model(&Model{}).Where("id=?", id).Updates(map[string]any{"current_version": v.Version, "updated_at": now}).Error; err != nil {
			return false, err
		}
		return true, checkCatalogSize(ctx, tx)
	})
}
func (r *Repository) UpdateStatus(ctx context.Context, id int64, value yesno.Value, base int64, now time.Time) (cache.MutationResult, error) {
	return r.mutation(ctx, base, now, func(tx *gorm.DB) (bool, error) {
		current, err := findCurrent(ctx, tx, id, true)
		if err != nil {
			return false, err
		}
		if current.Config.IsEnabled == value {
			return false, nil
		}
		return true, tx.Model(&Model{}).Where("id=?", id).Updates(map[string]any{"is_enabled": value, "updated_at": now}).Error
	})
}
func (r *Repository) MarkDeleted(ctx context.Context, id, base int64, now time.Time) (cache.MutationResult, error) {
	return r.mutation(ctx, base, now, func(tx *gorm.DB) (bool, error) {
		if _, err := findCurrent(ctx, tx, id, true); err != nil {
			return false, err
		}
		return true, tx.Model(&Model{}).Where("id=?", id).Updates(map[string]any{"is_enabled": yesno.No, "deleted_at": now, "updated_at": now}).Error
	})
}
func checkCatalogSize(ctx context.Context, db *gorm.DB) error {
	rows, err := catalogFacts(ctx, db)
	if err != nil {
		return err
	}
	// Same encoder and envelope as Redis, with worst-case generation digit count.
	raw, err := json.Marshal(snapshot{SchemaVersion: 1, Generation: int64(^uint64(0) >> 1), Variant: "catalog", Catalog: rows})
	if err != nil {
		return err
	}
	if len(raw) > MaxSnapshotBytes {
		return fmt.Errorf("payment catalog size limit exceeded")
	}
	return nil
}
