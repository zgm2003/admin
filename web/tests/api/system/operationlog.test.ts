import { expect, it, vi } from 'vitest'

import { getOperationLogs } from '@/api/system/operationlog'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

it('uses the Admin operation log namespace', async () => {
  vi.mocked(request).mockResolvedValue({ list: [], total: 0, page: 1, pageSize: 20 })
  const query = { page: 1, pageSize: 20 }
  await getOperationLogs(query)
  expect(request).toHaveBeenCalledWith({
    method: 'GET',
    url: '/api/admin/v1/operation-logs',
    params: query,
  })
})
