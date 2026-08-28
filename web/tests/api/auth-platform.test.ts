import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { YesNo } from '@src/enums/yes-no'
import {
  createAuthPlatform,
  deleteAuthPlatform,
  getAuthPlatforms,
  updateAuthPlatform,
  updateAuthPlatformStatus,
} from '@src/api/auth-platform'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('authentication platform API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses the exact list request', async () => {
    requestMock.mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20 })

    await getAuthPlatforms({ page: 1, pageSize: 20, keyword: 'admin', isEnabled: YesNo.Yes })

    expect(requestMock).toHaveBeenNthCalledWith(1, {
      method: 'GET', url: '/api/admin/v1/auth-platforms',
      params: { page: 1, pageSize: 20, keyword: 'admin', isEnabled: YesNo.Yes },
    })
  })

  it('uses exact mutation methods and allowlisted bodies', async () => {
    requestMock
      .mockResolvedValueOnce({ id: 3 })
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({ id: 3, isEnabled: YesNo.No })
      .mockResolvedValueOnce({})

    await createAuthPlatform({
      code: 'portal', name: 'Portal', accessTTLSeconds: 900, refreshTTLSeconds: 86_400,
      sessionCacheTTLSeconds: 7_200, accessCacheTTLSeconds: 600, bindDevice: YesNo.Yes,
      bindIP: YesNo.No, maxSessions: 1, allowRegister: YesNo.No, isEnabled: YesNo.Yes,
    })
    await updateAuthPlatform(3, {
      name: 'Portal 2', accessTTLSeconds: 1_800, refreshTTLSeconds: 86_400,
      sessionCacheTTLSeconds: 7_200, accessCacheTTLSeconds: 600, bindDevice: YesNo.Yes,
      bindIP: YesNo.Yes, maxSessions: 2, allowRegister: YesNo.Yes,
    })
    await updateAuthPlatformStatus(3, YesNo.No)
    await deleteAuthPlatform(3)

    expect(requestMock).toHaveBeenNthCalledWith(1, expect.objectContaining({ method: 'POST', url: '/api/admin/v1/auth-platforms' }))
    expect(requestMock.mock.calls[0]?.[0]).toMatchObject({ data: expect.objectContaining({ code: 'portal' }) })
    expect(requestMock.mock.calls[1]?.[0]).toMatchObject({ method: 'PUT', url: '/api/admin/v1/auth-platforms/3' })
    expect(requestMock.mock.calls[1]?.[0]).not.toHaveProperty('data.code')
    expect(requestMock).toHaveBeenNthCalledWith(3, { method: 'PATCH', url: '/api/admin/v1/auth-platforms/3/status', data: { isEnabled: YesNo.No } })
    expect(requestMock).toHaveBeenNthCalledWith(4, { method: 'DELETE', url: '/api/admin/v1/auth-platforms/3' })
  })
})
