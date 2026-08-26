import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { getHealth, getReadiness } from '@src/api/health'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

const mockedRequest = vi.mocked(request)

describe('health API', () => {
  beforeEach(() => mockedRequest.mockReset())

  it('returns backend health data directly', async () => {
    const data = { status: 'up' }
    mockedRequest.mockResolvedValue(data)
    await expect(getHealth()).resolves.toBe(data)
    expect(mockedRequest).toHaveBeenCalledWith({ method: 'GET', url: '/health' })
  })

  it('returns backend readiness data directly', async () => {
    const data = { postgresql: 'up', redis: 'up' }
    mockedRequest.mockResolvedValue(data)
    await expect(getReadiness()).resolves.toBe(data)
    expect(mockedRequest).toHaveBeenCalledWith({ method: 'GET', url: '/ready' })
  })
})
