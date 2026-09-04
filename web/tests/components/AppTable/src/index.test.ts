import { flushPromises, mount } from '@vue/test-utils'
import { h } from 'vue'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'

import AppTable from '@/components/AppTable/index.vue'
import type { TableColumn, TablePaginationState } from '@/components/AppTable/types'
import { appI18n, setLocale } from '@/i18n'

interface UserRow {
  id: number
  username: string
  enabled: boolean
}

describe('AppTable', () => {
  const columns: TableColumn<UserRow>[] = [
    { prop: 'username', label: 'User' },
    { key: 'status', label: 'Status', value: (row) => (row.enabled ? 'Enabled' : 'Disabled') },
    { prop: 'enabled', label: 'Hidden', hidden: true },
  ]

  it('renders property, derived, hidden columns and slots', async () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: { columns, data: [{ id: 1, username: 'alice', enabled: true }] },
      slots: { 'cell-username': '<strong>{{ row.username }}</strong>' },
      global: { plugins: [ElementPlus, appI18n] },
    })
    await flushPromises()
    const table = wrapper.findComponent({ name: 'ElTable' })
    expect(table.props('border')).toBe(true)
    expect(table.props('headerCellStyle')).toEqual({ background: 'var(--el-fill-color-light)' })
    expect(table.text()).toContain('alice')
    expect(table.text()).toContain('Enabled')
    expect(wrapper.findAll('.hidden').length).toBe(0)
  })

  it('passes the full row to a named cell slot when its key differs from its property', async () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: {
        columns: [{ key: 'status', prop: 'enabled', label: 'Status' }],
        data: [{ id: 7, username: 'alice', enabled: true }],
      },
      slots: {
        'cell-status': ({ row }: { row: UserRow }) =>
          h('span', { 'data-testid': 'status-cell' }, `${row.id}:${row.enabled}`),
      },
      global: { plugins: [ElementPlus, appI18n] },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="status-cell"]').text()).toBe('7:true')
  })

  it('exposes loading, empty/error states and typed pagination events', async () => {
    const pagination: TablePaginationState = { currentPage: 1, pageSize: 20, total: 40 }
    const wrapper = mount(AppTable<UserRow>, {
      props: {
        columns,
        data: [],
        loading: true,
        resultState: 'loading',
        pagination,
        statusMessage: 'No users',
      },
      global: { plugins: [ElementPlus, appI18n] },
    })
    expect(wrapper.find('[aria-busy="true"]').exists()).toBe(true)
    await wrapper.setProps({ loading: false, resultState: 'empty' })
    await flushPromises()
    expect(wrapper.text()).toContain('No users')
    wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
    expect(wrapper.emitted('update:pagination')?.[0]?.[0]).toEqual({
      currentPage: 2,
      pageSize: 20,
      total: 40,
    })
    await wrapper.setProps({ resultState: 'error' })
    expect(wrapper.text()).toContain('No users')
  })

  it('marks pagination for mobile end distribution', () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: {
        columns,
        data: [],
        pagination: { currentPage: 1, pageSize: 20, total: 2 },
      },
      global: { plugins: [ElementPlus, appI18n] },
    })

    expect(wrapper.get('.app-table__pagination').classes()).toContain(
      'app-table__pagination--distributed',
    )
  })

  it('forwards selection and row events without making requests', async () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: { columns, data: [{ id: 1, username: 'alice', enabled: true }], selectable: true },
      global: { plugins: [ElementPlus, appI18n] },
    })
    const table = wrapper.findComponent({ name: 'ElTable' })
    const row = { id: 1, username: 'alice', enabled: true }
    table.vm.$emit('selection-change', [row])
    table.vm.$emit('row-click', row)
    expect(wrapper.emitted('selection-change')?.[0]?.[0]).toEqual([row])
    expect(wrapper.emitted('row-click')?.[0]?.[0]).toEqual(row)
  })

  it('centers columns by default and allows explicit alignment overrides', () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: {
        columns: [
          { prop: 'username', label: 'User' },
          {
            prop: 'enabled',
            label: 'Enabled',
            elementProps: { align: 'left', headerAlign: 'right' },
          },
        ],
        data: [{ id: 1, username: 'alice', enabled: true }],
      },
      global: { plugins: [ElementPlus, appI18n] },
    })

    const columns = wrapper.findAllComponents({ name: 'ElTableColumn' })
    expect(columns[0]?.props('align')).toBe('center')
    expect(columns[0]?.props('headerAlign')).toBe('center')
    expect(columns[1]?.props('align')).toBe('left')
    expect(columns[1]?.props('headerAlign')).toBe('right')
  })

  it('renders an internal refresh button and emits refresh without making requests', async () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: { columns, data: [] },
      global: { plugins: [ElementPlus, appI18n] },
    })

    const refreshButton = wrapper.find('button[data-testid="app-table-refresh"]')
    expect(refreshButton.exists()).toBe(true)
    await refreshButton.trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)

    await wrapper.setProps({ loading: true })
    expect(wrapper.findComponent({ name: 'ElButton' }).props('loading')).toBe(true)
  })

  it('keeps toolbar actions in an Element Plus spacing container', () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: { columns, data: [] },
      global: { plugins: [ElementPlus, appI18n] },
    })

    expect(wrapper.findComponent({ name: 'ElSpace' }).exists()).toBe(true)
  })

  it('updates default state labels when the locale changes', async () => {
    setLocale('zh-CN')
    const wrapper = mount(AppTable<UserRow>, {
      props: { columns, data: [], resultState: 'empty' },
      global: { plugins: [ElementPlus, appI18n] },
    })
    expect(wrapper.text()).toContain('暂无数据')

    setLocale('en-US')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('No data')
    expect(wrapper.get('[data-testid="app-table-refresh"]').text()).toContain('Refresh')
  })
})
