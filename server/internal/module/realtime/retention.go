package realtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

const (
	retentionBatchLimit = 1000
	retentionMaxBatches = 10
	retentionBudget     = 5 * time.Second
)

type retentionRepository interface {
	CleanupExpired(context.Context, time.Time, int, time.Time) (int, error)
}

type CleanupResult struct {
	Deleted int
	Batches int
}

type RetentionService struct {
	repository retentionRepository
	settings   sharedsetting.Reader
	now        func() time.Time
}

func NewRetentionService(repository retentionRepository, settings sharedsetting.Reader) *RetentionService {
	return &RetentionService{repository: repository, settings: settings, now: time.Now}
}

func (s *RetentionService) Cleanup(ctx context.Context) (CleanupResult, error) {
	if s == nil || s.repository == nil || s.settings == nil {
		return CleanupResult{}, errors.New("realtime retention service is not configured")
	}
	record, err := s.settings.FindByKey(ctx, sharedsetting.RealtimeEventRetentionDaysKey)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("read realtime retention setting: %w", err)
	}
	if record.Key != sharedsetting.RealtimeEventRetentionDaysKey || record.IsEnabled != yesno.Yes || record.IsBuiltin != yesno.Yes {
		return CleanupResult{}, errors.New("realtime retention setting must be enabled and builtin")
	}
	days, err := record.Number()
	if err != nil || days < 1 || days > 30 {
		return CleanupResult{}, errors.New("realtime retention setting must be an integer from 1 to 30")
	}
	now := s.now().UTC()
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	cleanupCtx, cancel := context.WithTimeout(ctx, retentionBudget)
	defer cancel()
	result := CleanupResult{}
	for result.Batches < retentionMaxBatches {
		deleted, err := s.repository.CleanupExpired(cleanupCtx, cutoff, retentionBatchLimit, now)
		if err != nil {
			return result, err
		}
		result.Batches++
		result.Deleted += deleted
		if deleted < retentionBatchLimit {
			break
		}
	}
	return result, nil
}
