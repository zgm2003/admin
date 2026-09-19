package notification

import (
	"context"
	"errors"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type notificationRetentionRepository interface {
	CleanupNotifications(context.Context, time.Time, int) (int, error)
}
type CleanupResult struct {
	Deleted int
	Batches int
}
type RetentionService struct {
	repository notificationRetentionRepository
	settings   sharedsetting.Reader
	now        func() time.Time
}

func NewRetentionService(repository notificationRetentionRepository, settings sharedsetting.Reader) *RetentionService {
	return &RetentionService{repository: repository, settings: settings, now: time.Now}
}
func (s *RetentionService) Cleanup(ctx context.Context) (CleanupResult, error) {
	if s == nil || s.repository == nil || s.settings == nil {
		return CleanupResult{}, errors.New("notification retention service is not configured")
	}
	record, err := s.settings.FindByKey(ctx, sharedsetting.MessageNotificationRetentionDaysKey)
	if err != nil {
		return CleanupResult{}, err
	}
	if record.Key != sharedsetting.MessageNotificationRetentionDaysKey || record.IsEnabled != yesno.Yes || record.IsBuiltin != yesno.Yes {
		return CleanupResult{}, errors.New("notification retention setting must be enabled and builtin")
	}
	days, err := record.Number()
	if err != nil || days < 30 || days > 3650 {
		return CleanupResult{}, errors.New("notification retention setting must be an integer from 30 to 3650")
	}
	cutoff := s.now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result := CleanupResult{}
	for result.Batches < 10 {
		deleted, err := s.repository.CleanupNotifications(bounded, cutoff, 1000)
		if err != nil {
			return result, err
		}
		result.Batches++
		result.Deleted += deleted
		if deleted < 1000 {
			break
		}
	}
	return result, nil
}
