import { describe, expect, it } from 'vitest'

import {
  formatTableColumnValue,
  tableColumnKey,
  tableColumnValue,
  type TableColumn,
  type TablePaginationState,
} from '@/components/AppTable/types'

interface UserRow {
  id: number
  username: string
  enabled: boolean
}

describe('AppTable column helpers', () => {
  const row: UserRow = { id: 4, username: 'alice', enabled: true }

  it('resolves property and derived columns with typed values', () => {
    const property: TableColumn<UserRow> = { prop: 'username', label: 'Username' }
    const derived: TableColumn<UserRow> = {
      key: 'status',
      label: 'Status',
      value: (item) => (item.enabled ? 'yes' : 'no'),
    }
    expect(tableColumnKey(property)).toBe('username')
    expect(tableColumnKey(derived)).toBe('status')
    expect(tableColumnValue(row, property)).toBe('alice')
    expect(tableColumnValue(row, derived)).toBe('yes')
    expect(formatTableColumnValue(row, property, 0)).toBe('alice')
  })

  it('formats values and rejects an unidentifiable column', () => {
    const column: TableColumn<UserRow> = {
      prop: 'id',
      label: 'ID',
      formatter: (item, value) => `${item.username}:${value}`,
    }
    expect(formatTableColumnValue(row, column, 2)).toBe('alice:4')
    expect(() => tableColumnKey({ label: 'Invalid' })).toThrow('column key or prop is required')
  })

  it('uses the explicit camel-case pagination contract', () => {
    const pagination: TablePaginationState = { currentPage: 2, pageSize: 50, total: 99 }
    expect(pagination).toEqual({ currentPage: 2, pageSize: 50, total: 99 })
  })
})
