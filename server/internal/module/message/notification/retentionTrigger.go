package notification

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"
)

type retentionCleaner interface {
	Cleanup(context.Context) (CleanupResult, error)
}
type RetentionTrigger struct {
	cleaner  retentionCleaner
	logger   *slog.Logger
	interval time.Duration
}

func NewRetentionTrigger(cleaner retentionCleaner, logger *slog.Logger) *RetentionTrigger {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &RetentionTrigger{cleaner: cleaner, logger: logger, interval: time.Minute}
}
func (t *RetentionTrigger) Run(ctx context.Context) error {
	if t == nil || t.cleaner == nil {
		return errors.New("notification retention trigger is not configured")
	}
	t.run(ctx)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			t.run(ctx)
		}
	}
}
func (t *RetentionTrigger) run(ctx context.Context) {
	result, err := t.cleaner.Cleanup(ctx)
	if err != nil {
		if ctx.Err() == nil {
			t.logger.Error("notification retention cleanup failed", "errorClass", "dependency-unavailable")
		}
		return
	}
	t.logger.Info("notification retention cleanup completed", "deleted", result.Deleted, "batches", result.Batches)
}
