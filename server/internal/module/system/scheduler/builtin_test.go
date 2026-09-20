package scheduler

import (
	"context"
	"testing"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type historyRepositoryStub struct {
	calls  int
	cutoff time.Time
}

func (s *historyRepositoryStub) CleanupTerminalHistory(_ context.Context, cutoff time.Time, _ int) (int, int, error) {
	s.calls++
	s.cutoff = cutoff
	return 0, 0, nil
}

type settingReaderStub struct{ record sharedsetting.Record }

func (s settingReaderStub) FindByKey(context.Context, string) (sharedsetting.Record, error) {
	return s.record, nil
}
func TestHistoryCleanerUsesConfiguredRetention(t *testing.T) {
	now := time.Date(2026, 9, 20, 4, 0, 0, 0, time.UTC)
	repository := &historyRepositoryStub{}
	cleaner := NewHistoryCleaner(repository, settingReaderStub{record: sharedsetting.Record{Key: sharedsetting.SchedulerHistoryRetentionDaysKey, Value: "30", ValueType: sharedsetting.ValueTypeNumber, IsEnabled: yesno.Yes, IsBuiltin: yesno.Yes}})
	cleaner.now = func() time.Time { return now }
	if err := cleaner.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 || !repository.cutoff.Equal(now.Add(-30*24*time.Hour)) {
		t.Fatalf("repository=%+v", repository)
	}
}
