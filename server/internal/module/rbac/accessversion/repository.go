package accessversion

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"sort"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Find(ctx context.Context, userID int64) (Version, error) {
	var value Version
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Take(&value).Error; err != nil {
		return Version{}, fmt.Errorf("find access version: %w", err)
	}
	return value, nil
}
func (r *Repository) Increment(ctx context.Context, userIDs []int64, now time.Time) (map[int64]int64, error) {
	ids := append([]int64(nil), userIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	dedup := ids[:0]
	for _, id := range ids {
		if id < 1 {
			return nil, fmt.Errorf("access version user id is invalid")
		}
		if len(dedup) == 0 || dedup[len(dedup)-1] != id {
			dedup = append(dedup, id)
		}
	}
	if len(dedup) == 0 {
		return map[int64]int64{}, nil
	}
	rows := make([]Version, 0, len(dedup))
	if err := r.db.WithContext(ctx).Raw("UPDATE rbac_access_version SET version=version+1, updated_at=? WHERE user_id IN ? RETURNING user_id, version", now.UTC(), dedup).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("increment access versions: %w", err)
	}
	result := make(map[int64]int64, len(rows))
	for _, row := range rows {
		result[row.UserID] = row.Version
	}
	if len(result) != len(dedup) {
		return nil, fmt.Errorf("increment access versions returned incomplete users")
	}
	return result, nil
}
