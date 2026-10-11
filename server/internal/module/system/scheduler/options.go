package scheduler

import (
	"admin/server/internal/shared/i18n"
	"admin/server/internal/shared/option"
	"context"
)

type StatusOption[T comparable] struct {
	option.Option[T]
	Tone string `json:"tone"`
}

type OptionsResult struct {
	Defaults struct {
		CronExpression string `json:"cronExpression"`
		Timezone       string `json:"timezone"`
	} `json:"defaults"`
	Tasks            []taskOptionResponse           `json:"tasks"`
	JobStatuses      []StatusOption[JobStatus]      `json:"jobStatuses"`
	RunStatuses      []StatusOption[RunStatus]      `json:"runStatuses"`
	ScheduleStatuses []StatusOption[ScheduleStatus] `json:"scheduleStatuses"`
	TriggerSources   []option.Option[TriggerSource] `json:"triggerSources"`
	CronPresets      []option.Option[string]        `json:"cronPresets"`
}

func (s *Service) Options(ctx context.Context) (OptionsResult, error) {
	job, err := option.ManyLocalized(ctx, []option.LocalizedEntry[JobStatus]{
		{Value: JobScheduled, Key: i18n.KeySchedulerJobScheduled}, {Value: JobQueued, Key: i18n.KeySchedulerJobQueued}, {Value: JobRunning, Key: i18n.KeySchedulerJobRunning}, {Value: JobCompleted, Key: i18n.KeySchedulerJobCompleted}, {Value: JobFailed, Key: i18n.KeySchedulerJobFailed}, {Value: JobCanceled, Key: i18n.KeySchedulerJobCanceled},
	})
	if err != nil {
		return OptionsResult{}, err
	}
	runs, err := option.ManyLocalized(ctx, []option.LocalizedEntry[RunStatus]{
		{Value: RunRunning, Key: i18n.KeySchedulerJobRunning}, {Value: RunSucceeded, Key: i18n.KeySchedulerRunSucceeded}, {Value: RunFailed, Key: i18n.KeySchedulerJobFailed},
	})
	if err != nil {
		return OptionsResult{}, err
	}
	schedules, err := option.ManyLocalized(ctx, []option.LocalizedEntry[ScheduleStatus]{
		{Value: ScheduleDisabled, Key: i18n.KeySchedulerScheduleDisabled}, {Value: ScheduleEnabled, Key: i18n.KeySchedulerScheduleEnabled},
	})
	if err != nil {
		return OptionsResult{}, err
	}
	triggers, err := option.ManyLocalized(ctx, []option.LocalizedEntry[TriggerSource]{
		{Value: TriggerCron, Key: i18n.KeySchedulerTriggerCron}, {Value: TriggerManual, Key: i18n.KeySchedulerTriggerManual}, {Value: TriggerRetry, Key: i18n.KeySchedulerTriggerRetry}, {Value: TriggerBusiness, Key: i18n.KeySchedulerTriggerBusiness},
	})
	if err != nil {
		return OptionsResult{}, err
	}
	cron, err := option.ManyLocalized(ctx, []option.LocalizedEntry[string]{
		{Value: "* * * * *", Key: i18n.KeySchedulerCronEveryMinute}, {Value: "*/5 * * * *", Key: i18n.KeySchedulerCronEvery5Minutes}, {Value: "*/10 * * * *", Key: i18n.KeySchedulerCronEvery10Minutes}, {Value: "*/15 * * * *", Key: i18n.KeySchedulerCronEvery15Minutes}, {Value: "*/30 * * * *", Key: i18n.KeySchedulerCronEvery30Minutes}, {Value: "0 * * * *", Key: i18n.KeySchedulerCronHourly}, {Value: "0 0 * * *", Key: i18n.KeySchedulerCronDaily}, {Value: "30 3 * * *", Key: i18n.KeySchedulerCronDaily0330}, {Value: "0 0 * * 1", Key: i18n.KeySchedulerCronWeekly}, {Value: "0 0 1 * *", Key: i18n.KeySchedulerCronMonthly},
	})
	if err != nil {
		return OptionsResult{}, err
	}
	result := OptionsResult{Tasks: make([]taskOptionResponse, 0), TriggerSources: triggers, CronPresets: cron}
	for _, item := range job {
		tone := "info"
		if item.Value == JobRunning {
			tone = "warning"
		}
		if item.Value == JobCompleted {
			tone = "success"
		}
		if item.Value == JobFailed {
			tone = "danger"
		}
		result.JobStatuses = append(result.JobStatuses, StatusOption[JobStatus]{Option: item, Tone: tone})
	}
	for _, item := range runs {
		tone := "warning"
		if item.Value == RunSucceeded {
			tone = "success"
		}
		if item.Value == RunFailed {
			tone = "danger"
		}
		result.RunStatuses = append(result.RunStatuses, StatusOption[RunStatus]{Option: item, Tone: tone})
	}
	for _, item := range schedules {
		tone := "info"
		if item.Value == ScheduleEnabled {
			tone = "success"
		}
		result.ScheduleStatuses = append(result.ScheduleStatuses, StatusOption[ScheduleStatus]{Option: item, Tone: tone})
	}
	result.Defaults.CronExpression, result.Defaults.Timezone = result.CronPresets[1].Value, "Asia/Shanghai"
	for _, task := range s.catalog.Options() {
		if task.DisplayNameEnglish != "" {
			label, translateErr := i18n.Translate(i18n.LocaleFromContext(ctx), i18n.MessageKey("scheduler.tasks."+task.Type), nil)
			if translateErr != nil {
				return OptionsResult{}, translateErr
			}
			task.DisplayName = label
		}
		result.Tasks = append(result.Tasks, taskOptionDTO(task))
	}
	return result, nil
}
