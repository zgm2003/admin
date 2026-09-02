import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getSessions, getSessionStats, revokeSession, revokeSessions } from '@/api/user/session'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('session API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses only the Admin session namespace', async () => {
    requestMock
      .mockResolvedValueOnce({ list: [sessionItem()], total: 1, page: 1, pageSize: 20 })
      .mockResolvedValueOnce({ activeTotal: 1, platforms: { web: 1 } })
      .mockResolvedValueOnce({ revoked: 1, skippedCurrent: 0, skippedRevoked: 0 })
      .mockResolvedValueOnce({ revoked: 1, skippedCurrent: 0, skippedRevoked: 0 })
    const query = { page: 1, pageSize: 20 }
    await getSessions(query)
    await getSessionStats()
    await revokeSession(7)
    await revokeSessions([7, 8])

    expect(requestMock).toHaveBeenNthCalledWith(1, {
      method: 'GET',
      url: '/api/admin/v1/sessions',
      params: query,
    })
    expect(requestMock).toHaveBeenNthCalledWith(2, {
      method: 'GET',
      url: '/api/admin/v1/sessions/stats',
    })
    expect(requestMock).toHaveBeenNthCalledWith(3, {
      method: 'DELETE',
      url: '/api/admin/v1/sessions/7',
    })
    expect(requestMock).toHaveBeenNthCalledWith(4, {
      method: 'DELETE',
      url: '/api/admin/v1/sessions',
      data: { ids: [7, 8] },
    })
  })
})

function sessionItem() {
  return {
    id: 7,
    userId: 1,
    username: 'admin',
    platform: 'web',
    deviceId: 'device-1',
    clientIp: '127.0.0.1',
    userAgent: 'Vitest',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    refreshExpiresAt: '2026-01-02T00:00:00Z',
    revokedAt: null,
    status: 'active',
    isCurrent: true,
  } as const
}
