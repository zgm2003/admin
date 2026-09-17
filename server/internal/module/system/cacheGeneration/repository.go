package cachegeneration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// listRowsSQL 以 PostgreSQL 为唯一事实：generation 行左连接 pending 聚合与
// 已发布聚合；Redis state 由 Service 合成。
const listRowsSQL = `
SELECT
  g.namespace AS namespace,
  g.scope_key AS scope_key,
  g.generation AS generation,
  g.updated_at AS updated_at,
  COALESCE(pending.pending_count, 0) AS pending_count,
  pending.oldest_pending_at AS oldest_pending_at,
  COALESCE(pending.latest_attempts, 0) AS latest_attempts,
  COALESCE(pending.last_error, '') AS last_error,
  published.latest_published_generation AS latest_published_generation,
  published.latest_published_at AS latest_published_at
FROM system_config_cache_generation AS g
LEFT JOIN LATERAL (
  SELECT
    count(*) AS pending_count,
    min(created_at) AS oldest_pending_at,
    max(attempts) AS latest_attempts,
    (array_agg(last_error ORDER BY id DESC))[1] AS last_error
  FROM system_config_cache_outbox
  WHERE namespace = g.namespace AND scope_key = g.scope_key AND published_at IS NULL
) AS pending ON TRUE
LEFT JOIN LATERAL (
  SELECT
    max(generation) AS latest_published_generation,
    max(published_at) AS latest_published_at
  FROM system_config_cache_outbox
  WHERE namespace = g.namespace AND scope_key = g.scope_key AND published_at IS NOT NULL
) AS published ON TRUE
WHERE %s
`

func (r *Repository) List(ctx context.Context, query ListQuery) ([]Item, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("cache generation repository is not configured")
	}
	where, args := listConditions(query)
	rowsSQL := fmt.Sprintf(listRowsSQL, where)

	var total int64
	if err := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM (`+rowsSQL+`) AS rows`, args...).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count cache generations: %w", err)
	}

	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	var rows []struct {
		Namespace                 string
		ScopeKey                  string
		Generation                int64
		UpdatedAt                 time.Time
		PendingCount              int64
		OldestPendingAt           *time.Time
		LatestAttempts            int
		LastError                 string
		LatestPublishedGeneration *int64
		LatestPublishedAt         *time.Time
	}
	if err := r.db.WithContext(ctx).Raw(rowsSQL+` ORDER BY namespace ASC, scope_key ASC LIMIT ? OFFSET ?`, listArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list cache generations: %w", err)
	}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, Item{
			Namespace:                 row.Namespace,
			ScopeKey:                  row.ScopeKey,
			Generation:                row.Generation,
			UpdatedAt:                 row.UpdatedAt,
			PendingCount:              row.PendingCount,
			OldestPendingAt:           row.OldestPendingAt,
			LatestAttempts:            row.LatestAttempts,
			LastError:                 row.LastError,
			LatestPublishedGeneration: row.LatestPublishedGeneration,
			LatestPublishedAt:         row.LatestPublishedAt,
		})
	}
	return items, total, nil
}

func listConditions(query ListQuery) (string, []any) {
	conditions := []string{"TRUE"}
	args := make([]any, 0, 3)
	if query.Keyword != "" {
		pattern := "%" + escapeLike(query.Keyword) + "%"
		conditions = append(conditions, `(g.namespace LIKE ? ESCAPE '\' OR g.scope_key LIKE ? ESCAPE '\')`)
		args = append(args, pattern, pattern)
	}
	switch query.PublishState {
	case PublishStateReady:
		conditions = append(conditions, "COALESCE(pending.pending_count, 0) = 0")
	case PublishStatePending:
		conditions = append(conditions, "COALESCE(pending.pending_count, 0) > 0 AND COALESCE(pending.latest_attempts, 0) = 0")
	case PublishStateRetrying:
		conditions = append(conditions, "COALESCE(pending.pending_count, 0) > 0 AND COALESCE(pending.latest_attempts, 0) > 0")
	}
	return strings.Join(conditions, " AND "), args
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "%", `\%`)
	return strings.ReplaceAll(value, "_", `\_`)
}
