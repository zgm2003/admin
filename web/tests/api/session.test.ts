import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getSessions, getSessionStats, revokeSession, revokeSessions } from '@src/api/session'
import { request } from '@src/utils/request'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('session API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses only the Admin session namespace', async () => {
    requestMock.mockResolvedValue({})
    const query = { page: 1, pageSize: 20 }
    await getSessions(query)
    await getSessionStats()
    await revokeSession(7)
    await revokeSessions([7, 8])

    expect(requestMock).toHaveBeenNthCalledWith(1, {
      method: 'GET', url: '/api/admin/v1/sessions', params: query,
    })
    expect(requestMock).toHaveBeenNthCalledWith(2, {
      method: 'GET', url: '/api/admin/v1/sessions/stats',
    })
    expect(requestMock).toHaveBeenNthCalledWith(3, {
      method: 'DELETE', url: '/api/admin/v1/sessions/7',
    })
    expect(requestMock).toHaveBeenNthCalledWith(4, {
      method: 'DELETE', url: '/api/admin/v1/sessions', data: { ids: [7, 8] },
    })
  })
})
