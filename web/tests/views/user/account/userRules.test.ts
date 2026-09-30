import { describe, expect, it } from 'vitest'

import { userTableColumns } from '@/views/user/account/userRules'

describe('user management table columns', () => {
  it('allocates enough width for the complete action set', () => {
    const columns = userTableColumns((key) => key)
    const actions = columns.find((column) => column.key === 'actions')

    expect(actions?.width).toBe(500)
  })
})
