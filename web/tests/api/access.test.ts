import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { getAccess } from '@src/api/access'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('access API', () => {
  beforeEach(() => {
    requestMock.mockReset()
  })

  it('loads and validates the current access snapshot', async () => {
    const snapshot = { roleCodes: [], menuTree: [], permissionCodes: [] }
    requestMock.mockResolvedValue(snapshot)

    await expect(getAccess()).resolves.toEqual(snapshot)
    expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/v1/access' })
  })

  it('returns the backend snapshot without rebuilding it', async () => {
    const snapshot = { roleCodes: null, menuTree: [], permissionCodes: [] }
    requestMock.mockResolvedValue(snapshot)
    await expect(getAccess()).resolves.toBe(snapshot)
  })
})
