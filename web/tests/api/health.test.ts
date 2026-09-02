import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@/utils/request'
import { getHealth, getReadiness } from '@/api/health'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))

const mockedRequest = vi.mocked(request)

describe('health API', () => {
  beforeEach(() => mockedRequest.mockReset())

  it('parses backend health data', async () => {
    const data = { status: 'up' }
    mockedRequest.mockResolvedValue(data)
    await expect(getHealth()).resolves.toEqual(data)
    expect(mockedRequest).toHaveBeenCalledWith({ method: 'GET', url: '/health' })
  })

  it('parses backend readiness data', async () => {
    const data = { postgresql: 'up', redis: 'up' }
    mockedRequest.mockResolvedValue(data)
    await expect(getReadiness()).resolves.toEqual(data)
    expect(mockedRequest).toHaveBeenCalledWith({ method: 'GET', url: '/ready' })
  })
})
