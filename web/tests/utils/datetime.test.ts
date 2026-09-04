import { afterEach, describe, expect, it, vi } from 'vitest'

import { setLocale } from '@/i18n'
import { formatTime } from '@/utils/datetime'

describe('formatTime', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    setLocale('zh-CN')
  })

  it('uses the active application locale instead of the browser default', () => {
    const NativeDateTimeFormat = Intl.DateTimeFormat
    const formatter = vi.spyOn(Intl, 'DateTimeFormat').mockImplementation(function DateTimeFormat(
      locales,
      options,
    ) {
      return new NativeDateTimeFormat(locales, options)
    } as typeof Intl.DateTimeFormat)
    setLocale('en-US')

    formatTime('2026-09-03T14:33:44.2485Z')

    expect(formatter).toHaveBeenCalledWith('en-US', {
      dateStyle: 'medium',
      timeStyle: 'medium',
    })
  })

  it('renders malformed timestamps as a placeholder', () => {
    expect(formatTime('not-a-timestamp')).toBe('-')
  })
})
