package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"admin/server/internal/module/message/notification"
	"admin/server/internal/module/realtime"
	sharedsetting "admin/server/internal/shared/setting"
	"admin/server/internal/shared/yesno"
)

type realtimeRetentionCleaner interface {
	Cleanup(context.Context) (realtime.CleanupResult, error)
}
type notificationRetentionCleaner interface {
	Cleanup(context.Context) (notification.CleanupResult, error)
}

type HistoryCleaner struct {
	repository interface {
		CleanupTerminalHistory(context.Context, time.Time, int) (int, int, error)
	}
	settings sharedsetting.Reader
	now      func() time.Time
}

func NewHistoryCleaner(repository interface {
	CleanupTerminalHistory(context.Context, time.Time, int) (int, int, error)
}, settings sharedsetting.Reader) *HistoryCleaner {
	return &HistoryCleaner{repository: repository, settings: settings, now: time.Now}
}
func (h *HistoryCleaner) Cleanup(ctx context.Context) error {
	if h == nil || h.repository == nil || h.settings == nil {
		return errors.New("scheduler history cleaner is not configured")
	}
	record, err := h.settings.FindByKey(ctx, sharedsetting.SchedulerHistoryRetentionDaysKey)
	if err != nil {
		return fmt.Errorf("read scheduler history retention setting: %w", err)
	}
	if record.Key != sharedsetting.SchedulerHistoryRetentionDaysKey || record.IsEnabled != yesno.Yes || record.IsBuiltin != yesno.Yes {
		return errors.New("scheduler history retention setting must be enabled and builtin")
	}
	days, err := record.Number()
	if err != nil || days < 7 || days > 3650 {
		return errors.New("scheduler history retention setting must be an integer from 7 to 3650")
	}
	cutoff := h.now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	for batch := 0; batch < 10; batch++ {
		jobs, _, err := h.repository.CleanupTerminalHistory(ctx, cutoff, 500)
		if err != nil {
			return err
		}
		if jobs < 500 {
			return nil
		}
	}
	return nil
}

func BuiltinDefinitions(realtimeCleaner realtimeRetentionCleaner, notificationCleaner notificationRetentionCleaner, historyCleaner *HistoryCleaner) []TaskDefinition {
	fixed := func(taskType, name, builtinKey string, timeout time.Duration, execute TaskExecutor) TaskDefinition {
		return TaskDefinition{Type: taskType, DisplayName: name, BuiltinKey: builtinKey, DefaultParams: json.RawMessage(`{}`), ValidateParams: validateEmptyObject, Queue: "maintenance", Timeout: timeout, MaxAttempts: 3, Backoff: publishBackoff, Execute: execute}
	}
	return []TaskDefinition{
		fixed("realtime.retention.cleanup", "实时事件历史清理", "realtime.retention.cleanup", 30*time.Second, func(ctx context.Context, _ ExecutionContext) error {
			if realtimeCleaner == nil {
				return errors.New("realtime retention cleaner is unavailable")
			}
			_, err := realtimeCleaner.Cleanup(ctx)
			return err
		}),
		fixed("message.notification.retention.cleanup", "站内通知历史清理", "message.notification.retention.cleanup", 30*time.Second, func(ctx context.Context, _ ExecutionContext) error {
			if notificationCleaner == nil {
				return errors.New("notification retention cleaner is unavailable")
			}
			_, err := notificationCleaner.Cleanup(ctx)
			return err
		}),
		fixed("system.scheduler.history.cleanup", "调度器历史清理", "system.scheduler.history.cleanup", 30*time.Second, func(ctx context.Context, _ ExecutionContext) error {
			if historyCleaner == nil {
				return errors.New("scheduler history cleaner is unavailable")
			}
			return historyCleaner.Cleanup(ctx)
		}),
	}
}

func validateEmptyObject(raw json.RawMessage) error {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if len(value) != 0 {
		return errors.New("built-in task does not accept parameters")
	}
	return nil
}
