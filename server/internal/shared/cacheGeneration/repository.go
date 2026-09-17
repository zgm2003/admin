package cachegeneration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrGenerationRowMissing = errors.New("cache generation row is missing")
	ErrOutboxClaimLost      = errors.New("cache generation outbox claim was lost")
)

type Event struct {
	ID         int64
	Scope      Scope
	Generation int64
	Attempts   int
	LockToken  string
}

// Repository 只访问 PostgreSQL：generation 行与 outbox 事件。
type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) configured() error {
	if r == nil || r.db == nil {
		return fmt.Errorf("cache generation repository is not configured")
	}
	return nil
}

func (r *Repository) Current(ctx context.Context, scope Scope) (int64, error) {
	if err := r.configured(); err != nil {
		return 0, err
	}
	if err := scope.Validate(); err != nil {
		return 0, err
	}
	var generation int64
	if err := r.db.WithContext(ctx).Raw(
		`SELECT generation FROM system_config_cache_generation WHERE namespace = ? AND scope_key = ?`,
		scope.Namespace, scope.ScopeKey).Scan(&generation).Error; err != nil {
		return 0, fmt.Errorf("read cache generation: %w", err)
	}
	if generation < 1 {
		return 0, fmt.Errorf("%w: %s/%s", ErrGenerationRowMissing, scope.Namespace, scope.ScopeKey)
	}
	return generation, nil
}

// AdvanceTx 使用调用方事务：锁定 generation 行、校验 expected、
// 原子推进 generation 并插入 outbox。
func (r *Repository) AdvanceTx(ctx context.Context, tx *gorm.DB, scope Scope, expected int64, now time.Time) (Event, error) {
	if r == nil {
		return Event{}, fmt.Errorf("cache generation repository is not configured")
	}
	if tx == nil {
		return Event{}, fmt.Errorf("cache generation advance requires an open transaction")
	}
	if err := scope.Validate(); err != nil {
		return Event{}, err
	}
	if expected < 1 {
		return Event{}, fmt.Errorf("expected cache generation is invalid")
	}

	statement := tx.WithContext(ctx)
	var current int64
	if err := statement.Raw(
		`SELECT generation FROM system_config_cache_generation WHERE namespace = ? AND scope_key = ? FOR UPDATE`,
		scope.Namespace, scope.ScopeKey).Scan(&current).Error; err != nil {
		return Event{}, fmt.Errorf("lock cache generation: %w", err)
	}
	if current < 1 {
		return Event{}, fmt.Errorf("%w: %s/%s", ErrGenerationRowMissing, scope.Namespace, scope.ScopeKey)
	}
	if current != expected {
		return Event{}, fmt.Errorf("%w: expected %d, current %d", ErrGenerationChanged, expected, current)
	}

	next := current + 1
	result := statement.Exec(
		`UPDATE system_config_cache_generation SET generation = ?, updated_at = ? WHERE namespace = ? AND scope_key = ?`,
		next, now, scope.Namespace, scope.ScopeKey)
	if result.Error != nil {
		return Event{}, fmt.Errorf("advance cache generation: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return Event{}, fmt.Errorf("advance cache generation affected %d rows", result.RowsAffected)
	}

	event := Event{Scope: scope, Generation: next}
	if err := statement.Raw(
		`INSERT INTO system_config_cache_outbox (
			namespace, scope_key, generation, attempts, available_at, created_at, updated_at
		 ) VALUES (?, ?, ?, 0, ?, ?, ?) RETURNING id`,
		scope.Namespace, scope.ScopeKey, next, now, now, now).Scan(&event.ID).Error; err != nil {
		return Event{}, fmt.Errorf("insert cache generation outbox: %w", err)
	}
	return event, nil
}

// ClaimPending 在一个短事务内用 FOR UPDATE SKIP LOCKED 批量 claim，
// 写入统一 lock token、locked_until 并递增 attempts。
func (r *Repository) ClaimPending(ctx context.Context, limit int, token string, now time.Time, lease time.Duration) ([]Event, error) {
	if err := r.configured(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 50 {
		return nil, fmt.Errorf("cache generation claim limit is out of range")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("cache generation claim token is required")
	}
	if lease <= 0 {
		return nil, fmt.Errorf("cache generation claim lease is invalid")
	}
	lockedUntil := now.Add(lease)
	events := make([]Event, 0, limit)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			ID         int64
			Namespace  string
			ScopeKey   string
			Generation int64
			Attempts   int
		}
		if err := tx.Raw(
			`SELECT id, namespace, scope_key, generation, attempts
			 FROM system_config_cache_outbox
			 WHERE published_at IS NULL AND available_at <= ? AND (locked_until IS NULL OR locked_until <= ?)
			 ORDER BY id
			 LIMIT ?
			 FOR UPDATE SKIP LOCKED`,
			now, now, limit).Scan(&rows).Error; err != nil {
			return fmt.Errorf("claim cache generation outbox: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		result := tx.Exec(
			`UPDATE system_config_cache_outbox
			 SET lock_token = ?, locked_until = ?, attempts = attempts + 1, updated_at = ?
			 WHERE id IN ? AND published_at IS NULL`,
			token, lockedUntil, now, ids)
		if result.Error != nil {
			return fmt.Errorf("mark cache generation claim: %w", result.Error)
		}
		if int(result.RowsAffected) != len(rows) {
			return fmt.Errorf("mark cache generation claim affected %d of %d rows", result.RowsAffected, len(rows))
		}
		for _, row := range rows {
			scope, err := NewScope(row.Namespace, row.ScopeKey)
			if err != nil {
				return fmt.Errorf("decode cache generation outbox scope: %w", err)
			}
			events = append(events, Event{
				ID: row.ID, Scope: scope, Generation: row.Generation, Attempts: row.Attempts + 1, LockToken: token,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *Repository) MarkPublished(ctx context.Context, id int64, token string, now time.Time) error {
	if err := r.configured(); err != nil {
		return err
	}
	if id < 1 || strings.TrimSpace(token) == "" {
		return fmt.Errorf("cache generation publish claim is invalid")
	}
	result := r.db.WithContext(ctx).Exec(
		`UPDATE system_config_cache_outbox
		 SET published_at = ?, locked_until = NULL, lock_token = NULL, updated_at = ?
		 WHERE id = ? AND lock_token = ? AND published_at IS NULL`,
		now, now, id, token)
	if result.Error != nil {
		return fmt.Errorf("mark cache generation outbox published: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: outbox %d", ErrOutboxClaimLost, id)
	}
	return nil
}

// MarkPublishedIfUnclaimed 供同步发布路径使用：只处理尚未被 relay claim 的事件。
// 事件已被 claim 时返回 false，由 relay 用 claim token 完成发布与标记。
func (r *Repository) MarkPublishedIfUnclaimed(ctx context.Context, id int64, now time.Time) (bool, error) {
	if err := r.configured(); err != nil {
		return false, err
	}
	if id < 1 {
		return false, fmt.Errorf("cache generation publish id is invalid")
	}
	result := r.db.WithContext(ctx).Exec(
		`UPDATE system_config_cache_outbox
		 SET published_at = ?, updated_at = ?
		 WHERE id = ? AND published_at IS NULL AND lock_token IS NULL`,
		now, now, id)
	if result.Error != nil {
		return false, fmt.Errorf("mark cache generation outbox published: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	if result.RowsAffected != 1 {
		return false, fmt.Errorf("mark cache generation outbox published affected %d rows", result.RowsAffected)
	}
	return true, nil
}

func (r *Repository) Reschedule(ctx context.Context, id int64, token, safeError string, availableAt, now time.Time) error {
	if err := r.configured(); err != nil {
		return err
	}
	if id < 1 || strings.TrimSpace(token) == "" {
		return fmt.Errorf("cache generation reschedule claim is invalid")
	}
	if availableAt.IsZero() {
		return fmt.Errorf("cache generation reschedule time is invalid")
	}
	result := r.db.WithContext(ctx).Exec(
		`UPDATE system_config_cache_outbox
		 SET available_at = ?, last_error = ?, locked_until = NULL, lock_token = NULL, updated_at = ?
		 WHERE id = ? AND lock_token = ? AND published_at IS NULL`,
		availableAt, truncateOutboxError(safeError), now, id, token)
	if result.Error != nil {
		return fmt.Errorf("reschedule cache generation outbox: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: outbox %d", ErrOutboxClaimLost, id)
	}
	return nil
}

// truncateOutboxError 只保留可打印字符并把长度限制在 512 字符内。
func truncateOutboxError(message string) string {
	cleaned := make([]rune, 0, len(message))
	for _, character := range message {
		if character < 0x20 || character == 0x7f {
			character = ' '
		}
		cleaned = append(cleaned, character)
	}
	text := strings.TrimSpace(string(cleaned))
	runes := []rune(text)
	if len(runes) > 512 {
		text = string(runes[:512])
	}
	return text
}
