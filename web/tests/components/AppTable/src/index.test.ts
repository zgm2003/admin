import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { describe, expect, it } from 'vitest'

import AppTable from '@src/components/AppTable/src/index.vue'
import type { TableColumn, TablePaginationState } from '@src/components/AppTable/src/types'

interface UserRow { id: number; username: string; enabled: boolean }

describe('AppTable', () => {
  const columns: TableColumn<UserRow>[] = [
    { prop: 'username', label: 'User' },
    { key: 'status', label: 'Status', value: (row) => row.enabled ? 'Enabled' : 'Disabled' },
    { prop: 'enabled', label: 'Hidden', hidden: true },
  ]

  it('renders property, derived, hidden columns and slots', async () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: { columns, data: [{ id: 1, username: 'alice', enabled: true }] },
      slots: { 'cell-username': '<strong>{{ row.username }}</strong>' },
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ElTable' }).text()).toContain('alice')
    expect(wrapper.findComponent({ name: 'ElTable' }).text()).toContain('Enabled')
    expect(wrapper.findAll('.hidden').length).toBe(0)
  })

  it('exposes loading, empty/error states and typed pagination events', async () => {
    const pagination: TablePaginationState = { currentPage: 1, pageSize: 20, total: 40 }
    const wrapper = mount(AppTable<UserRow>, {
      props: { columns, data: [], loading: true, resultState: 'loading', pagination, statusMessage: 'No users' },
      global: { plugins: [ElementPlus] },
    })
    expect(wrapper.find('[aria-busy="true"]').exists()).toBe(true)
    await wrapper.setProps({ loading: false, resultState: 'empty' })
    await flushPromises()
    expect(wrapper.text()).toContain('No users')
    wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
    expect(wrapper.emitted('update:pagination')?.[0]?.[0]).toEqual({ currentPage: 2, pageSize: 20, total: 40 })
    await wrapper.setProps({ resultState: 'error' })
    expect(wrapper.text()).toContain('No users')
  })

  it('forwards selection and row events without making requests', async () => {
    const wrapper = mount(AppTable<UserRow>, {
      props: { columns, data: [{ id: 1, username: 'alice', enabled: true }], selectable: true },
      global: { plugins: [ElementPlus] },
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
      global: { plugins: [ElementPlus] },
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
      global: { plugins: [ElementPlus] },
    })

    const refreshButton = wrapper.find('button[data-testid="app-table-refresh"]')
    expect(refreshButton.exists()).toBe(true)
    await refreshButton.trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)

    await wrapper.setProps({ loading: true })
    expect(wrapper.findComponent({ name: 'ElButton' }).props('loading')).toBe(true)
  })
})
