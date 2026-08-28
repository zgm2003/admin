import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'
import Search from '@src/components/Search/src/index.vue'
import type { SearchField, SearchFormModel } from '@src/components/Search/src/types'
import { appI18n, setLocale } from '@src/i18n'

describe('Search', () => {
  const fields: SearchField[] = [
    { key: 'keyword', type: 'input', label: 'Keyword' },
    { key: 'status', type: 'select-v2', label: 'Status', options: [{ label: 'Enabled', value: 1 }] },
    { key: 'role', type: 'input', label: 'Role' },
  ]

  it('renders select-v2 fields and collapses extra fields', async () => {
    const wrapper = mount(Search, {
      props: { modelValue: { keyword: '', status: '', role: '' }, fields, collapseCount: 2 },
      global: { plugins: [ElementPlus, appI18n] },
    })
    expect(wrapper.findComponent({ name: 'ElSelectV2' }).exists()).toBe(true)
    expect(wrapper.findAll('.el-form-item')).toHaveLength(4)
    await wrapper.findAll('button').find((button) => button.text().includes('收起'))?.trigger('click')
    expect(wrapper.findAll('.el-form-item')).toHaveLength(3)
  })

  it('does not override Element Plus focus styles with bare native-control selectors', () => {
    const stylesheet = readFileSync(resolve(process.cwd(), 'src/styles/index.scss'), 'utf8')

    expect(stylesheet).not.toMatch(/^\s*(button|input|textarea):focus-visible\s*[,\{]/m)
  })

  it('emits query and reset with a copied form model', async () => {
    const model: SearchFormModel = { keyword: 'alice', status: 1 }
    const wrapper = mount(Search, {
      props: { modelValue: model, fields: fields.slice(0, 2) },
      global: { plugins: [ElementPlus, appI18n] },
    })
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('query')?.[0]?.[0]).toEqual(model)
    await wrapper.findAll('button').find((button) => button.text().includes('重置'))?.trigger('click')
    expect(wrapper.emitted('reset')?.[0]?.[0]).toEqual({ keyword: undefined, status: undefined })
  })

  it('emits query when the query button is clicked', async () => {
    const wrapper = mount(Search, {
      props: { modelValue: { keyword: 'portal' }, fields: fields.slice(0, 1) },
      global: { plugins: [ElementPlus, appI18n] },
    })

    await wrapper.find('button').trigger('click')

    expect(wrapper.emitted('query')?.[0]?.[0]).toEqual({ keyword: 'portal' })
  })

  it('uses Element Plus spacing for actions and updates default labels by locale', async () => {
    setLocale('zh-CN')
    const wrapper = mount(Search, {
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
})
