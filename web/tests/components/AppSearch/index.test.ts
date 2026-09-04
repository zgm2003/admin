import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'

import AppSearch from '@/components/AppSearch/index.vue'
import type { SearchField, SearchFieldType, SearchFormModel } from '@/components/AppSearch/types'
import { appI18n, setLocale } from '@/i18n'

describe('Search', () => {
  interface OperationLogSearchModel {
    keyword: string
    method: string
    dateRange: [] | [string, string]
  }

  const dateRangeType: SearchFieldType = 'date-range'
  const fields: SearchField[] = [
    { key: 'keyword', type: 'input', label: 'Keyword' },
    {
      key: 'status',
      type: 'select-v2',
      label: 'Status',
      options: [{ label: 'Enabled', value: 1 }],
    },
    { key: 'role', type: 'input', label: 'Role' },
  ]

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
    const model: SearchFormModel = { keyword: 'alice', status: 1 }
    const wrapper = mount(AppSearch, {
      props: { modelValue: model, fields: fields.slice(0, 2) },
      global: { plugins: [ElementPlus, appI18n] },
    })
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('query')?.[0]?.[0]).toEqual(model)
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('重置'))
      ?.trigger('click')
    expect(wrapper.emitted('reset')?.[0]?.[0]).toEqual({ keyword: undefined, status: undefined })
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
    setLocale('zh-CN')
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

  it('rejects a model value that does not match the field type without throwing during render', async () => {
    const wrapper = mount(AppSearch, {
      props: {
        modelValue: { keyword: ['bad'] } as never,
        fields: [{ key: 'keyword', type: 'input', label: 'Keyword' }],
      },
      global: { plugins: [ElementPlus, appI18n] },
    })

    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('query')).toBeUndefined()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it('models all supported field kinds with keys from the search model', () => {
    const typedFields: SearchField<OperationLogSearchModel>[] = [
      { key: 'keyword', type: 'input', label: 'Keyword' },
      { key: 'method', type: 'select-v2', label: 'Method', options: [] },
      { key: 'dateRange', type: 'date-range', label: 'Date range' },
    ]

    expect(typedFields.map((field) => field.key)).toEqual(['keyword', 'method', 'dateRange'])
  })
})
