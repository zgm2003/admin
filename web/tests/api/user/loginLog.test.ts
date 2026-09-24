import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getLoginLogPageInit, getLoginLogs } from '@/api/user/loginLog'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('login log API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses the exact Admin endpoints', async () => {
    requestMock.mockResolvedValueOnce({ eventTypes: [1, 2, 3], loginTypes: [1, 2, 3] })
    requestMock.mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20 })

    await getLoginLogPageInit()
    await getLoginLogs({ page: 1, pageSize: 20, eventType: 2, loginType: 1, isSuccess: 1 })

    expect(requestMock).toHaveBeenNthCalledWith(1, {
      method: 'GET',
      url: '/api/admin/v1/user/loginlog/page-init',
    })
    expect(requestMock).toHaveBeenNthCalledWith(2, {
      method: 'GET',
      url: '/api/admin/v1/user/loginlog',
      params: { page: 1, pageSize: 20, eventType: 2, loginType: 1, isSuccess: 1 },
    })
  })

  it('parses the numeric audit contract and rejects legacy fields', async () => {
    requestMock.mockResolvedValueOnce({
      list: [
        {
          id: 1,
          userId: 7,
          platform: 'admin',
          account: 'user@example.com',
          eventType: 2,
          loginType: 2,
          isSuccess: 1,
          reasonCode: 'success',
          clientIp: '127.0.0.1',
          userAgent: 'Vitest',
          createdAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const result = await getLoginLogs({ page: 1, pageSize: 20 })
    expect(result.list[0].account).toBe('user@example.com')
    expect(result.list[0].eventType).toBe(2)
    expect(result.list[0].loginType).toBe(2)

    requestMock.mockResolvedValueOnce({
      list: [
        {
          id: 1,
          userId: null,
          sessionId: null,
          platform: 'admin',
          loginAccount: 'user@example.com',
          eventType: 'login',
          loginType: 'email',
          isSuccess: 1,
          reasonCode: 'success',
          clientIp: '127.0.0.1',
          userAgent: 'Vitest',
          createdAt: '2026-01-01T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    await expect(getLoginLogs({ page: 1, pageSize: 20 })).rejects.toThrow()
  })
})
