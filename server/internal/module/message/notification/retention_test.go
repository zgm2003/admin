package notification

import (
	"context"
	"testing"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

func TestRetentionUsesSettingAndTenBatchLimit(t *testing.T) {
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	repo := &notificationRetentionRepositoryStub{values: []int{1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000}}
	settings := mailboxSettingStub{record: sharedsetting.Record{Key: sharedsetting.MessageNotificationRetentionDaysKey, Value: "180", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes}}
	service := NewRetentionService(repo, settings)
	service.now = func() time.Time { return now }
	result, err := service.Cleanup(context.Background())
	if err != nil || result.Deleted != 10000 || result.Batches != 10 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !repo.cutoff.Equal(now.Add(-180 * 24 * time.Hour)) {
		t.Fatalf("cutoff=%s", repo.cutoff)
	}
}
func TestRetentionRejectsInvalidSettingWithoutDelete(t *testing.T) {
	repo := &notificationRetentionRepositoryStub{}
	settings := mailboxSettingStub{record: sharedsetting.Record{Key: sharedsetting.MessageNotificationRetentionDaysKey, Value: "29", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes}}
	if _, err := NewRetentionService(repo, settings).Cleanup(context.Background()); err == nil {
		t.Fatal("Cleanup error=nil")
	}
	if repo.calls != 0 {
		t.Fatalf("calls=%d", repo.calls)
	}
}

type notificationRetentionRepositoryStub struct {
	values []int
	calls  int
	cutoff time.Time
}

func (s *notificationRetentionRepositoryStub) CleanupNotifications(_ context.Context, cutoff time.Time, _ int) (int, error) {
	s.cutoff = cutoff
	i := s.calls
	s.calls++
	if i >= len(s.values) {
		return 0, nil
	}
	return s.values[i], nil
}
