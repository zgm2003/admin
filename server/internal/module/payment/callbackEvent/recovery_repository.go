package callbackevent

import (
	"context"
	"errors"
	"time"
)

func (r *Repository) ListPending(ctx context.Context, cutoff, after time.Time, afterID int64, limit int) ([]Model, error) {
	if ctx == nil || r == nil || r.db == nil || cutoff.IsZero() || afterID < 0 || (afterID == 0) != after.IsZero() || limit < 1 || limit > 100 {
		return nil, errors.New("payment callback recovery query invalid")
	}
	query := r.db.WithContext(ctx).Where("process_status = ? AND created_at <= ?", ProcessPending, cutoff.UTC())
	if afterID > 0 {
		query = query.Where("(created_at,id) > (?,?)", after.UTC(), afterID)
	}
	rows := make([]Model, 0, limit)
	err := query.Order("created_at ASC, id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}
