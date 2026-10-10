import { describe, expect, it } from 'vitest'

import type { TaskOption } from '@/api/system/scheduler'
import {
  CUSTOM_CRON_VALUE,
  getCronPresetValue,
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

describe('scheduler backend-owned presentation', () => {
  it('keeps a new backend cron preset selectable without a frontend value list', () => {
    const presets = [{ value: '*/11 * * * *', label: 'Every eleven minutes' }]
    expect(getCronPresetValue('*/11 * * * *', presets)).toBe('*/11 * * * *')
    expect(getCronPresetValue('15 2 * * 2', presets)).toBe(CUSTOM_CRON_VALUE)
    expect(getCronPresetValue('*/5 * * * *', [])).toBe(CUSTOM_CRON_VALUE)
  })

  it('resolves task labels while leaving internal type available as a fallback signal', () => {
    expect(resolveTaskDisplayName('realtime.retention.cleanup', options)).toBe('实时数据清理')
    expect(resolveTaskDisplayName('unknown.task', options)).toBe('unknown.task')
  })
})
