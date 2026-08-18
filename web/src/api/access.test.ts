import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '../utils/request'
import { ProtocolError } from '../types/http'
import { getAccess } from './access'

vi.mock('../utils/request', () => ({ request: vi.fn() }))

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

  it('rejects an invalid access response', async () => {
    requestMock.mockResolvedValue({ roleCodes: null, menuTree: [], permissionCodes: [] })
    await expect(getAccess()).rejects.toBeInstanceOf(ProtocolError)
  })
})
