import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it } from 'vitest'

import { AppSearch } from '@/components/AppSearch'
import type { SearchField, SearchFieldType, SearchFormModel } from '@/components/AppSearch/types'
import { appI18n, setLocale } from '@/i18n'

describe('AppSearch', () => {
  interface OperationLogSearchModel {
    keyword: string
    method: string
    dateRange: [] | [string, string]
  }

  const dateRangeType: SearchFieldType = 'date-range'
  const fields: SearchField[] = [
    { key: 'keyword', type: 'input', label: 'Keyword', resetValue: '' },
    {
      key: 'status',
      type: 'select-v2',
      label: 'Status',
      options: [{ label: 'Enabled', value: 1 }],
      resetValue: '',
    },
    { key: 'role', type: 'input', label: 'Role', resetValue: '' },
  ]

  beforeEach(() => setLocale('zh-CN'))

  it('renders select-v2 fields and collapses extra fields', async () => {
    const wrapper = mount(AppSearch, {
      props: { modelValue: { keyword: '', status: '', role: '' }, fields, collapseCount: 2 },
      global: { plugins: [ElementPlus, appI18n] },
    })
    expect(wrapper.findComponent({ name: 'ElSelectV2' }).exists()).toBe(true)
    expect(wrapper.findAll('.el-form-item')).toHaveLength(4)
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('收起'))
      ?.trigger('click')
    expect(wrapper.findAll('.el-form-item')).toHaveLength(3)
  })

  it('emits query and reset with a copied form model', async () => {
    interface ResetSearchModel {
      keyword: string
      status: '' | 1
    }
    const model: SearchFormModel<ResetSearchModel> = { keyword: 'alice', status: 1 }
    const resetFields: SearchField<ResetSearchModel>[] = [
      { key: 'keyword', type: 'input', label: 'Keyword', resetValue: '' },
      {
        key: 'status',
        type: 'select-v2',
        label: 'Status',
        options: [{ label: 'Enabled', value: 1 }],
        resetValue: '',
      },
    ]
    const wrapper = mount(AppSearch, {
      props: {
        modelValue: model as unknown as SearchFormModel,
        fields: resetFields as SearchField[],
      },
      global: { plugins: [ElementPlus, appI18n] },
    })
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('query')?.[0]?.[0]).toEqual(model)
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('重置'))
      ?.trigger('click')
    expect(wrapper.emitted('reset')?.[0]?.[0]).toEqual({
      keyword: '',
      status: '',
    })
  })

  it('emits query when the query button is clicked', async () => {
    const wrapper = mount(AppSearch, {
      props: { modelValue: { keyword: 'portal' }, fields: fields.slice(0, 1) },
      global: { plugins: [ElementPlus, appI18n] },
    })

    await wrapper.find('button').trigger('click')

    expect(wrapper.emitted('query')?.[0]?.[0]).toEqual({ keyword: 'portal' })
  })

  it('uses Element Plus spacing for actions and updates default labels by locale', async () => {
    const wrapper = mount(AppSearch, {
      props: { modelValue: { keyword: '' }, fields: fields.slice(0, 1) },
      global: { plugins: [ElementPlus, appI18n] },
    })
    expect(wrapper.findComponent({ name: 'ElSpace' }).exists()).toBe(true)
    expect(wrapper.text()).toContain('查询')

    setLocale('en-US')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Search')
    expect(wrapper.text()).toContain('Reset')
  })

  it('supports date-range as a public field type', () => {
    expect(dateRangeType).toBe('date-range')
  })

  it('passes distinct start and end placeholders to date ranges', () => {
    const wrapper = mount(AppSearch, {
      props: {
        modelValue: { dateRange: [] },
        fields: [
          {
            key: 'dateRange',
            type: 'date-range',
            label: 'Date range',
            resetValue: [],
            startPlaceholder: 'Start time',
            endPlaceholder: 'End time',
          },
        ],
      },
      global: { plugins: [ElementPlus, appI18n] },
    })

    const picker = wrapper.getComponent({ name: 'ElDatePicker' })
    expect(picker.props('startPlaceholder')).toBe('Start time')
    expect(picker.props('endPlaceholder')).toBe('End time')
  })

  it('rejects a model value that does not match the field type without throwing during render', async () => {
    const wrapper = mount(AppSearch, {
      props: {
        modelValue: { keyword: ['bad'] } as never,
        fields: [{ key: 'keyword', type: 'input', label: 'Keyword', resetValue: '' }],
      },
      global: { plugins: [ElementPlus, appI18n] },
    })

    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('query')).toBeUndefined()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it('models all supported field kinds with keys from the search model', () => {
    const typedFields: SearchField<OperationLogSearchModel>[] = [
      { key: 'keyword', type: 'input', label: 'Keyword', resetValue: '' },
      { key: 'method', type: 'select-v2', label: 'Method', options: [], resetValue: '' },
      { key: 'dateRange', type: 'date-range', label: 'Date range', resetValue: [] },
    ]

    expect(typedFields.map((field) => field.key)).toEqual(['keyword', 'method', 'dateRange'])
  })

  it('binds select option values to the selected model field type', () => {
    interface TypedStatusSearchModel {
      keyword: string
      status: number | undefined
    }

    const typedFields: SearchField<TypedStatusSearchModel>[] = [
      { key: 'keyword', type: 'input', label: 'Keyword', resetValue: '' },
      {
        key: 'status',
        type: 'select-v2',
        label: 'Status',
        resetValue: undefined,
        options: [
          { label: 'Enabled', value: 1 },
          // @ts-expect-error status options must preserve the numeric model value type
          { label: 'Invalid', value: 'enabled' },
        ],
      },
    ]

    const invalidResetFields: SearchField<TypedStatusSearchModel>[] = [
      // @ts-expect-error status reset values must preserve the model field type
      {
        key: 'status',
        type: 'select-v2',
        label: 'Status',
        options: [{ label: 'Enabled', value: 1 }],
        resetValue: '',
      },
    ]

    expect([...typedFields, ...invalidResetFields]).toHaveLength(3)
  })

  it.each([null, undefined])('maps a cleared date range value %s to an empty range', (value) => {
    const wrapper = mountDateRange()
    wrapper.findComponent({ name: 'ElDatePicker' }).vm.$emit('update:modelValue', value)

    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    expect((emitted!.at(-1)![0] as OperationLogSearchModel).dateRange).toEqual([])
  })

  it.each([['2026-09-01T00:00:00Z'], {}, 42])(
    'rejects malformed date range update %j instead of treating it as clear',
    async (value) => {
      const wrapper = mountDateRange()
      wrapper.findComponent({ name: 'ElDatePicker' }).vm.$emit('update:modelValue', value)
      await wrapper.vm.$nextTick()

      expect(wrapper.emitted('update:modelValue')).toBeUndefined()
      expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    },
  )

  function mountDateRange() {
    return mount(AppSearch, {
      props: {
        modelValue: {
          keyword: '',
          method: '',
          dateRange: ['2026-09-01T00:00:00Z', '2026-09-02T00:00:00Z'],
        },
        fields: [
          { key: 'keyword', type: 'input', label: 'Keyword', resetValue: '' },
          { key: 'method', type: 'select-v2', label: 'Method', options: [], resetValue: '' },
          { key: 'dateRange', type: 'date-range', label: 'Date range', resetValue: [] },
        ],
      },
      global: { plugins: [ElementPlus, appI18n] },
    })
  }
})
