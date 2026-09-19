package realtime

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
	if t == nil || t.cleaner == nil || t.interval <= 0 {
		return errors.New("realtime retention trigger is not configured")
	}
	t.runOnce(ctx)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			t.runOnce(ctx)
		}
	}
}

func (t *RetentionTrigger) runOnce(ctx context.Context) {
	result, err := t.cleaner.Cleanup(ctx)
	if err != nil {
		if ctx.Err() == nil {
			t.logger.Error("realtime retention cleanup failed", "errorClass", relayErrorClass(err))
		}
		return
	}
	t.logger.Info("realtime retention cleanup completed", "deleted", result.Deleted, "batches", result.Batches)
}
