import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { getHealth, getReadiness } from '@src/api/health'

vi.mock('@src/utils/request', () => ({
  ProtocolError: class ProtocolError extends Error {},
  request: vi.fn(),
}))

const mockedRequest = vi.mocked(request)

describe('health API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('rejects malformed health data', async () => {
    mockedRequest.mockResolvedValue({})

    await expect(getHealth()).rejects.toThrow('GET /health returned invalid data')
  })

  it('rejects malformed readiness data', async () => {
    mockedRequest.mockResolvedValue({ postgresql: 'up' })

    await expect(getReadiness()).rejects.toThrow('GET /ready returned invalid data')
  })
})
