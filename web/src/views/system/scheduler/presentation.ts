import type { TaskOption } from '@/api/system/scheduler'
import { JobStatus, isJobStatus, isRunStatus } from '@/enums/scheduler'

export const jobStatusMetadata = [
  { value: JobStatus.completed, i18nKey: 'scheduler.completed' },
  { value: JobStatus.failed, i18nKey: 'scheduler.failed' },
  { value: JobStatus.running, i18nKey: 'scheduler.running' },
] as const

export const CUSTOM_CRON_VALUE = '__custom__'

export const CRON_PRESETS = [
  { value: '* * * * *', labelKey: 'scheduler.cronPreset.everyMinute' },
  { value: '*/5 * * * *', labelKey: 'scheduler.cronPreset.everyFiveMinutes' },
  { value: '*/10 * * * *', labelKey: 'scheduler.cronPreset.everyTenMinutes' },
  { value: '*/15 * * * *', labelKey: 'scheduler.cronPreset.everyFifteenMinutes' },
  { value: '*/30 * * * *', labelKey: 'scheduler.cronPreset.everyThirtyMinutes' },
  { value: '0 * * * *', labelKey: 'scheduler.cronPreset.hourly' },
  { value: '0 0 * * *', labelKey: 'scheduler.cronPreset.dailyMidnight' },
  { value: '30 3 * * *', labelKey: 'scheduler.cronPreset.dailyAt0330' },
  { value: '0 0 * * 1', labelKey: 'scheduler.cronPreset.weeklyMonday' },
  { value: '0 0 1 * *', labelKey: 'scheduler.cronPreset.monthlyFirstDay' },
  { value: CUSTOM_CRON_VALUE, labelKey: 'scheduler.cronPreset.custom' },
] as const

function normalizeCronExpression(expression: string): string {
  const normalized = expression.trim()
  return normalized === '*/1 * * * *' ? '* * * * *' : normalized
}

export function getCronPresetValue(expression: string): string {
  const normalized = normalizeCronExpression(expression)
  return CRON_PRESETS.some(
    (preset) => preset.value === normalized && preset.value !== CUSTOM_CRON_VALUE,
  )
    ? normalized
    : CUSTOM_CRON_VALUE
}

export function getCronLabelKey(expression: string): string {
  const normalized = normalizeCronExpression(expression)
  return (
    CRON_PRESETS.find((preset) => preset.value === normalized && preset.value !== CUSTOM_CRON_VALUE)
      ?.labelKey ?? 'scheduler.cronPreset.custom'
  )
}

export function resolveTaskDisplayName(type: string, options: readonly TaskOption[]): string {
  return options.find((option) => option.type === type)?.displayName ?? ''
}

export function getTriggerSourceKey(source: string): string {
  switch (source) {
    case 'cron':
      return 'scheduler.triggerSource.cron'
    case 'manual':
      return 'scheduler.triggerSource.manual'
    case 'retry':
      return 'scheduler.triggerSource.retry'
    case 'business':
      return 'scheduler.triggerSource.business'
    default:
      return 'scheduler.triggerSource.unknown'
  }
}

export function getJobStatusKey(status: number): string {
  if (!isJobStatus(status)) return 'scheduler.status.unknown'
  const keys = ['scheduled', 'queued', 'running', 'completed', 'failed', 'canceled'] as const
  return `scheduler.status.${keys[status - 1]}`
}

export function getRunStatusKey(status: number): string {
  if (!isRunStatus(status)) return 'scheduler.runStatus.unknown'
  const keys = ['running', 'succeeded', 'failed'] as const
  return `scheduler.runStatus.${keys[status - 1]}`
}
