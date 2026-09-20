import type { TaskOption } from '@/api/system/scheduler'

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

const JOB_STATUS_KEYS = new Set([
  'scheduled',
  'queued',
  'running',
  'completed',
  'failed',
  'canceled',
])

const RUN_STATUS_KEYS = new Set(['running', 'succeeded', 'failed', 'canceled'])

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

export function getJobStatusKey(status: string): string {
  return JOB_STATUS_KEYS.has(status) ? `scheduler.status.${status}` : 'scheduler.status.unknown'
}

export function getRunStatusKey(status: string): string {
  return RUN_STATUS_KEYS.has(status)
    ? `scheduler.runStatus.${status}`
    : 'scheduler.runStatus.unknown'
}
