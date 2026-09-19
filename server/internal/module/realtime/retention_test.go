package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

func TestRetentionRejectsInvalidSettingWithoutDeleting(t *testing.T) {
	tests := []struct {
		name   string
		record sharedsetting.Record
		err    error
	}{
		{name: "missing", err: errors.New("missing")},
		{name: "disabled", record: retentionRecord("7", sharedsetting.ValueTypeNumber, yesno.No, yesno.Yes)},
		{name: "wrong type", record: retentionRecord("7", sharedsetting.ValueTypeString, yesno.Yes, yesno.Yes)},
		{name: "not builtin", record: retentionRecord("7", sharedsetting.ValueTypeNumber, yesno.Yes, yesno.No)},
		{name: "below range", record: retentionRecord("0", sharedsetting.ValueTypeNumber, yesno.Yes, yesno.Yes)},
		{name: "above range", record: retentionRecord("31", sharedsetting.ValueTypeNumber, yesno.Yes, yesno.Yes)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &retentionRepositoryStub{}
			service := NewRetentionService(repository, retentionSettingStub{record: test.record, err: test.err})
			if _, err := service.Cleanup(context.Background()); err == nil {
				t.Fatal("Cleanup() error=nil")
			}
			if repository.calls != 0 {
				t.Fatalf("cleanup calls=%d", repository.calls)
			}
		})
	}
}

func TestRetentionUsesDynamicCutoffAndTenBatchLimit(t *testing.T) {
	now := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	repository := &retentionRepositoryStub{deleted: []int{1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000, 1000}}
	settings := &retentionSettingStub{record: retentionRecord("7", sharedsetting.ValueTypeNumber, yesno.Yes, yesno.Yes)}
	service := NewRetentionService(repository, settings)
	service.now = func() time.Time { return now }
	result, err := service.Cleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted != 10000 || result.Batches != 10 || repository.calls != 10 {
		t.Fatalf("result=%+v calls=%d", result, repository.calls)
	}
	if !repository.cutoffs[0].Equal(now.Add(-7*24*time.Hour)) || repository.limits[0] != 1000 {
		t.Fatalf("cutoff=%s limit=%d", repository.cutoffs[0], repository.limits[0])
	}
	settings.record.Value = "1"
	repository.deleted = []int{0}
	if _, err := service.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !repository.cutoffs[len(repository.cutoffs)-1].Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("dynamic cutoff=%s", repository.cutoffs[len(repository.cutoffs)-1])
	}
}

func retentionRecord(value string, valueType int, enabled, builtin yesno.Value) sharedsetting.Record {
	return sharedsetting.Record{Key: sharedsetting.RealtimeEventRetentionDaysKey, Value: value, ValueType: valueType, IsEnabled: enabled, IsBuiltin: builtin}
}

type retentionSettingStub struct {
	record sharedsetting.Record
	err    error
}

func (s retentionSettingStub) FindByKey(context.Context, string) (sharedsetting.Record, error) {
	return s.record, s.err
}

type retentionRepositoryStub struct {
	deleted []int
	calls   int
	cutoffs []time.Time
	limits  []int
}

func (s *retentionRepositoryStub) CleanupExpired(_ context.Context, cutoff time.Time, limit int, _ time.Time) (int, error) {
	s.cutoffs = append(s.cutoffs, cutoff)
	s.limits = append(s.limits, limit)
	index := s.calls
	s.calls++
	if index >= len(s.deleted) {
		return 0, nil
	}
	return s.deleted[index], nil
}
