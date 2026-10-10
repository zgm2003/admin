package scheduler

import (
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

func (s *Service) Options(ctx context.Context) OptionsResult {
	result := OptionsResult{
		Tasks: make([]taskOptionResponse, 0),
		JobStatuses: []StatusOption[JobStatus]{
			{option.New(ctx, JobScheduled, "已调度", "Scheduled"), "info"},
			{option.New(ctx, JobQueued, "已排队", "Queued"), "info"},
			{option.New(ctx, JobRunning, "执行中", "Running"), "warning"},
			{option.New(ctx, JobCompleted, "已完成", "Completed"), "success"},
			{option.New(ctx, JobFailed, "失败", "Failed"), "danger"},
			{option.New(ctx, JobCanceled, "已取消", "Canceled"), "info"},
		},
		RunStatuses: []StatusOption[RunStatus]{
			{option.New(ctx, RunRunning, "执行中", "Running"), "warning"},
			{option.New(ctx, RunSucceeded, "成功", "Succeeded"), "success"},
			{option.New(ctx, RunFailed, "失败", "Failed"), "danger"},
		},
		ScheduleStatuses: []StatusOption[ScheduleStatus]{
			{option.New(ctx, ScheduleDisabled, "禁用", "Disabled"), "info"},
			{option.New(ctx, ScheduleEnabled, "启用", "Enabled"), "success"},
		},
		TriggerSources: []option.Option[TriggerSource]{
			option.New(ctx, TriggerCron, "定时触发", "Cron"),
			option.New(ctx, TriggerManual, "手动触发", "Manual"),
			option.New(ctx, TriggerRetry, "重试", "Retry"),
			option.New(ctx, TriggerBusiness, "业务触发", "Business"),
		},
		CronPresets: []option.Option[string]{
			option.New(ctx, "* * * * *", "每分钟", "Every minute"),
			option.New(ctx, "*/5 * * * *", "每 5 分钟", "Every 5 minutes"),
			option.New(ctx, "*/10 * * * *", "每 10 分钟", "Every 10 minutes"),
			option.New(ctx, "*/15 * * * *", "每 15 分钟", "Every 15 minutes"),
			option.New(ctx, "*/30 * * * *", "每 30 分钟", "Every 30 minutes"),
			option.New(ctx, "0 * * * *", "每小时", "Hourly"),
			option.New(ctx, "0 0 * * *", "每天零点", "Daily at midnight"),
			option.New(ctx, "30 3 * * *", "每天 03:30", "Daily at 03:30"),
			option.New(ctx, "0 0 * * 1", "每周一零点", "Monday at midnight"),
			option.New(ctx, "0 0 1 * *", "每月 1 日零点", "First day of month at midnight"),
		},
	}
	result.Defaults.CronExpression, result.Defaults.Timezone = result.CronPresets[1].Value, "Asia/Shanghai"
	for _, task := range s.catalog.Options() {
		if task.DisplayNameEnglish != "" {
			task.DisplayName = option.New(ctx, task.Type, task.DisplayName, task.DisplayNameEnglish).Label
		}
		result.Tasks = append(result.Tasks, taskOptionDTO(task))
	}
	return result
}
