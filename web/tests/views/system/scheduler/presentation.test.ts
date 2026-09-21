import { describe, expect, it } from 'vitest'

import type { TaskOption } from '@/api/system/scheduler'
import { JobStatus, RunStatus } from '@/enums/scheduler'
import {
  CUSTOM_CRON_VALUE,
  CRON_PRESETS,
  getCronPresetValue,
  getCronLabelKey,
  getJobStatusKey,
  getRunStatusKey,
  getTriggerSourceKey,
  jobStatusMetadata,
  resolveTaskDisplayName,
} from '@/views/system/scheduler/presentation'

const options: TaskOption[] = [
  {
    type: 'realtime.retention.cleanup',
    displayName: '实时数据清理',
    adminCreatable: true,
    builtinKey: 'realtime-retention',
    defaultParams: {},
  },
]

describe('scheduler presentation', () => {
  it('owns selectable job status metadata in scheduler presentation', () => {
    expect(jobStatusMetadata.map((item) => item.value)).toEqual([4, 5, 3])
    expect(jobStatusMetadata.map((item) => item.i18nKey)).toEqual([
      'scheduler.completed',
      'scheduler.failed',
      'scheduler.running',
    ])
  })
  it('provides common cron presets and keeps custom expressions editable', () => {
    expect(CRON_PRESETS.map((item) => item.value)).toContain('*/5 * * * *')
    expect(getCronPresetValue('*/5 * * * *')).toBe('*/5 * * * *')
    expect(getCronPresetValue('*/1 * * * *')).toBe('* * * * *')
    expect(getCronLabelKey('*/1 * * * *')).toBe('scheduler.cronPreset.everyMinute')
    expect(getCronPresetValue('15 2 * * 2')).toBe(CUSTOM_CRON_VALUE)
    expect(getCronLabelKey('15 2 * * 2')).toBe('scheduler.cronPreset.custom')
  })

  it('resolves task labels while leaving internal type available as a fallback signal', () => {
    expect(resolveTaskDisplayName('realtime.retention.cleanup', options)).toBe('实时数据清理')
    expect(resolveTaskDisplayName('unknown.task', options)).toBe('')
  })

  it('maps internal trigger and status values to translation keys', () => {
    expect(getTriggerSourceKey('cron')).toBe('scheduler.triggerSource.cron')
    expect(getTriggerSourceKey('manual')).toBe('scheduler.triggerSource.manual')
    expect(getTriggerSourceKey('other')).toBe('scheduler.triggerSource.unknown')
    expect(getJobStatusKey(JobStatus.queued)).toBe('scheduler.status.queued')
    expect(getRunStatusKey(RunStatus.succeeded)).toBe('scheduler.runStatus.succeeded')
    expect(getRunStatusKey(99)).toBe('scheduler.runStatus.unknown')
  })
})
