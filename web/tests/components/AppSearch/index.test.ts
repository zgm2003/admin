import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'

import { AppSearch } from '@/components/AppSearch'
import { appI18n } from '@/i18n'

describe('AppSearch', () => {
  const fields = [
    { key: 'keyword', type: 'input', label: 'Keyword' },
    {
      key: 'timeRange',
      type: 'date-range',
      label: 'Time',
      valueFormat: 'YYYY-MM-DDTHH:mm:ssZ',
    },
  ] as const

  it('maps a cleared date range to an empty range instead of dropping the event', () => {
    const wrapper = mount(AppSearch, {
      props: {
        modelValue: { keyword: '', timeRange: ['2026-09-01T00:00:00+08:00', '2026-09-02T00:00:00+08:00'] },
        fields: [...fields],
      },
      global: { plugins: [appI18n, ElementPlus] },
    })

    const picker = wrapper.findComponent({ name: 'ElDatePicker' })
    expect(picker.exists()).toBe(true)
    // el-date-picker emits null when the user clicks the clearable icon.
    picker.vm.$emit('update:modelValue', null)

    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    const last = emitted![emitted!.length - 1][0] as { keyword: string; timeRange: unknown }
    expect(last.timeRange).toEqual([])
  })
})
