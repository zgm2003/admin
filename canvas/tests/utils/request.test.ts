import { describe, expect, it, vi } from 'vitest'

import { ApiError, createRequestClient, ProtocolError } from '@/utils/request'

function response(body: unknown, status = 200, headers?: HeadersInit): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json', ...headers },
  })
}

describe('canvas request client', () => {
  it('uses same-origin API paths when no absolute API base URL is configured', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(response({ code: 0, data: {}, message: 'ok' }))
    const client = createRequestClient({
      baseUrl: '',
      platform: 'canvas',
      deviceId: '550e8400-e29b-41d4-a716-446655440000',
      fetchImpl: fetchMock,
    })

    await client.get('/api/canvas/v1/auth/policy')
    expect(fetchMock.mock.calls[0][0]).toBe('http://localhost:3000/api/canvas/v1/auth/policy')
  })

  it('sends canvas identity headers and unwraps the exact API envelope', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      response({ code: 0, data: { code: 'canvas' }, message: 'ok' }),
    )
    const client = createRequestClient({
      baseUrl: 'http://localhost:16301',
      platform: 'canvas',
      deviceId: '550e8400-e29b-41d4-a716-446655440000',
      fetchImpl: fetchMock,
    })

    await expect(client.get<{ code: string }>('/api/canvas/v1/auth/policy')).resolves.toEqual({ code: 'canvas' })
    const [, init] = fetchMock.mock.calls[0]
    expect(new Headers(init?.headers).get('X-Auth-Platform')).toBe('canvas')
    expect(new Headers(init?.headers).get('X-Device-ID')).toBe('550e8400-e29b-41d4-a716-446655440000')
    expect(init?.credentials).toBe('include')
  })

  it('rejects malformed envelopes instead of treating them as successful data', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      response({ code: 0, data: {}, message: 'ok', extra: true }),
    )
    const client = createRequestClient({
      baseUrl: 'http://localhost:16301',
      platform: 'canvas',
      deviceId: '550e8400-e29b-41d4-a716-446655440000',
      fetchImpl: fetchMock,
    })

    await expect(client.get('/api/canvas/v1/auth/me')).rejects.toBeInstanceOf(ProtocolError)
  })

  it('keeps login business errors available to the login page', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      response({ code: 10002, data: null, message: '凭据错误' }, 401),
    )
    const client = createRequestClient({
      baseUrl: 'http://localhost:16301',
      platform: 'canvas',
      deviceId: '550e8400-e29b-41d4-a716-446655440000',
      fetchImpl: fetchMock,
    })

    await expect(client.post('/api/canvas/v1/auth/login', { loginType: 'password' })).rejects.toMatchObject({
      code: 10002,
      httpStatus: 401,
      message: '凭据错误',
    })
  })

  it('sends the access token to logout without attempting a refresh on 401', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      response({ code: 10002, data: null, message: '会话已失效' }, 401),
    )
    const client = createRequestClient({
      baseUrl: 'http://localhost:16301',
      platform: 'canvas',
      deviceId: '550e8400-e29b-41d4-a716-446655440000',
      fetchImpl: fetchMock,
      initialAccessToken: 'active-token',
    })

    await expect(client.post('/api/canvas/v1/auth/logout')).rejects.toMatchObject({ code: 10002 })
    const [, init] = fetchMock.mock.calls[0]
    expect(new Headers(init?.headers).get('Authorization')).toBe('Bearer active-token')
    expect(fetchMock.mock.calls.some(([input]) => String(input).endsWith('/api/canvas/v1/auth/refresh'))).toBe(false)
  })

  it('shares one refresh request across concurrent 401 responses and retries both calls', async () => {
    let protectedCalls = 0
    const fetchMock = vi.fn<typeof fetch>().mockImplementation(async (input) => {
      const url = String(input)
      if (url.endsWith('/api/canvas/v1/auth/refresh')) {
        return response({
          code: 0,
          data: { accessToken: 'fresh-token', expiresIn: 300, isNewUser: false, passwordSetRequired: false },
          message: 'ok',
        })
      }
      protectedCalls += 1
      if (protectedCalls <= 2) return response({ code: 10002, data: null, message: 'expired' }, 401)
      return response({ code: 0, data: { ok: true }, message: 'ok' })
    })
    const client = createRequestClient({
      baseUrl: 'http://localhost:16301',
      platform: 'canvas',
      deviceId: '550e8400-e29b-41d4-a716-446655440000',
      fetchImpl: fetchMock,
      initialAccessToken: 'stale-token',
    })

    await expect(Promise.all([client.get('/api/canvas/v1/protected/a'), client.get('/api/canvas/v1/protected/b')])).resolves.toEqual([
      { ok: true },
      { ok: true },
    ])
    expect(fetchMock.mock.calls.filter(([input]) => String(input).endsWith('/api/canvas/v1/auth/refresh'))).toHaveLength(1)
  })

  it('clears access and calls onUnauthorized when refresh fails', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation(async (input) => {
      if (String(input).endsWith('/api/canvas/v1/auth/refresh')) {
        return response({ code: 10002, data: null, message: '会话已失效' }, 401)
      }
      return response({ code: 10002, data: null, message: 'expired' }, 401)
    })
    const onUnauthorized = vi.fn()
    const client = createRequestClient({
      baseUrl: 'http://localhost:16301',
      platform: 'canvas',
      deviceId: '550e8400-e29b-41d4-a716-446655440000',
      fetchImpl: fetchMock,
      initialAccessToken: 'stale-token',
      onUnauthorized,
    })

    await expect(client.get('/api/canvas/v1/protected')).rejects.toMatchObject({ code: 10002 })
    expect(client.getAccessToken()).toBe('')
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })
})
