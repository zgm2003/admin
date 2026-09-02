import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getLoginLogPageInit, getLoginLogs } from '@/api/user/loginlog'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('login log API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses the exact Admin endpoints', async () => {
    requestMock.mockResolvedValueOnce({ eventTypes: ['login', 'logout'], loginTypes: ['password'] })
    requestMock.mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20 })

    await getLoginLogPageInit()
    await getLoginLogs({ page: 1, pageSize: 20, eventType: 'login', isSuccess: 1 })

    expect(requestMock).toHaveBeenNthCalledWith(1, {
      method: 'GET',
      url: '/api/admin/v1/users/login-logs/page-init',
    })
    expect(requestMock).toHaveBeenNthCalledWith(2, {
      method: 'GET',
      url: '/api/admin/v1/users/login-logs',
      params: { page: 1, pageSize: 20, eventType: 'login', isSuccess: 1 },
    })
  })
})
