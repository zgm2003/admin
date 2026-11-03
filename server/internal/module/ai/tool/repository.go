package tool

import (
	configcache "admin/server/internal/shared/configCache"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Create(ctx context.Context, in ToolInput) (int64, error) {
	var id int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		m := Model{Code: in.Code, Name: in.Name, Type: in.Version.Type, CurrentVersion: 1, Status: in.Status, CreatedBy: in.ActorID, UpdatedBy: in.ActorID, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		v := versionOf(in.Version, in.ActorID)
		v.ToolID = m.ID
		v.Version = 1
		v.CreatedAt = now
		v.UpdatedAt = now
		if err := tx.Create(&v).Error; err != nil {
			return err
		}
		id = m.ID
		return nil
	})
	return id, err
}
func (r *Repository) Update(ctx context.Context, id int64, in UpdateInput) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, id).Error; err != nil {
			return err
		}
		v := versionOf(in.Version, in.ActorID)
		if in.Version.Type == TypeHTTP && in.Version.SecretHeaders == nil {
			var old Version
			if e := tx.Where("tool_id=? AND version=?", id, m.CurrentVersion).First(&old).Error; e != nil {
				return e
			}
			v.SecretCiphertext = old.SecretCiphertext
			v.EncryptionKeyID = old.EncryptionKeyID
		}
		v.ToolID = id
		v.Version = m.CurrentVersion + 1
		v.CreatedAt = time.Now().UTC()
		v.UpdatedAt = v.CreatedAt
		if err := tx.Create(&v).Error; err != nil {
			return err
		}
		return tx.Model(&m).Updates(map[string]any{"name": in.Name, "type": v.Type, "current_version": v.Version, "updated_by": in.ActorID, "updated_at": v.CreatedAt}).Error
	})
}
func (r *Repository) CurrentVersion(ctx context.Context, id int64) (Version, error) {
	var v Version
	err := r.db.WithContext(ctx).Raw(`SELECT v.* FROM ai_tool t JOIN ai_tool_version v ON v.tool_id=t.id AND v.version=t.current_version WHERE t.id=? AND t.deleted_at IS NULL`, id).Scan(&v).Error
	if err == nil && v.ID == 0 {
		err = gorm.ErrRecordNotFound
	}
	return v, err
}
func (r *Repository) Version(ctx context.Context, id int64) (Version, error) {
	var v Version
	err := r.db.WithContext(ctx).First(&v, id).Error
	return v, err
}
func (r *Repository) Find(ctx context.Context, id int64) (Model, error) {
	var m Model
	err := r.db.WithContext(ctx).First(&m, id).Error
	return m, err
}
func (r *Repository) List(ctx context.Context, page, size int) ([]Model, int64, error) {
	var rows []Model
	var total int64
	q := r.db.WithContext(ctx).Model(&Model{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("updated_at DESC,id DESC").Limit(size).Offset((page - 1) * size).Find(&rows).Error
	return rows, total, err
}
func (r *Repository) UpdateStatus(ctx context.Context, id int64, status Status, actor int64) error {
	res := r.db.WithContext(ctx).Model(&Model{}).Where("id=?", id).Updates(map[string]any{"status": status, "updated_by": actor, "updated_at": time.Now().UTC()})
	if res.Error == nil && res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return res.Error
}
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, id).Error; err != nil {
			return err
		}
		var n int64
		if err := tx.Raw(`SELECT (SELECT count(*) FROM ai_agent_tool WHERE tool_id=?)+(SELECT count(*) FROM ai_tool_call c JOIN ai_tool_version v ON v.id=c.tool_version_id WHERE v.tool_id=?)`, id, id).Scan(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return ErrReferenced
		}
		return tx.Delete(&m).Error
	})
}

func (r *Repository) Mutate(ctx context.Context, expected int64, fn func(*Repository) error) (configcache.MutationResult, error) {
	if r == nil || r.db == nil {
		return configcache.MutationResult{}, errors.New("AI database unavailable")
	}
	var result configcache.MutationResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := fn(NewRepository(tx)); e != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, e)
		}
		event, e := configcache.NewRepository(tx).AdvanceTx(ctx, tx, configcache.Scope{Namespace: "ai.runtime", ScopeKey: "global"}, expected, time.Now().UTC())
		if e != nil {
			return fmt.Errorf("%w: %w", configcache.ErrMutationRolledBack, e)
		}
		result = configcache.MutationResult{Changed: true, Generation: event.Generation, OutboxID: event.ID}
		return nil
	})
	return result, err
}
