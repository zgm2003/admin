import { AxiosError, AxiosHeaders, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios'
import { ElNotification } from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { pinia } from '@src/store'
import { setLocale } from '@src/i18n'
import { useAuthStore } from '@src/store/auth'
import {
  ApiError,
  ProtocolError,
  createRequestClient,
  unwrapEnvelope,
} from '@src/utils/request'

vi.mock('element-plus', () => ({
  ElNotification: {
    error: vi.fn(),
  },
}))

const notifyErrorMock = vi.mocked(ElNotification.error)

describe('unwrapEnvelope', () => {
  it('returns data from the only accepted success shape', () => {
    expect(unwrapEnvelope({ code: 0, data: { value: 'ok' }, message: 'ok' })).toEqual({
      value: 'ok',
    })
  })

  it.each([
    null,
    [],
    { code: 0, message: 'ok' },
    { code: 0, data: {}, msg: 'ok' },
    { code: 0, data: {}, message: 'ok', extra: true },
    { code: '0', data: {}, message: 'ok' },
    { code: 0, data: {}, message: 1 },
  ])('rejects protocol violations: %j', (value) => {
    expect(() => unwrapEnvelope(value)).toThrow(ProtocolError)
  })

  it('throws the stable business code and message', () => {
    expect.assertions(3)
    try {
      unwrapEnvelope({ code: 10001, data: null, message: '请求参数错误' })
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError)
      expect((error as ApiError).code).toBe(10001)
      expect((error as ApiError).message).toBe('请求参数错误')
    }
  })
})

describe('createRequestClient', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    useAuthStore(pinia).$reset()
    notifyErrorMock.mockReset()
  })

  it('requires an explicit API base URL', () => {
    expect(() => createRequestClient('  ')).toThrow(ProtocolError)
  })

  it('returns network errors unchanged', async () => {
    const networkError = new AxiosError('connection refused', AxiosError.ERR_NETWORK)
    const adapter: AxiosAdapter = async () => Promise.reject(networkError)
    const client = createRequestClient('http://localhost:16301')

    await expect(client.get('/health', { adapter })).rejects.toBe(networkError)
    expect(notifyErrorMock).not.toHaveBeenCalled()
  })

  it.each([
    {
      status: 400,
      data: { code: 10001, data: null, message: '请求参数错误' },
      code: 10001,
      message: '请求参数错误',
    },
    {
      status: 503,
      data: { code: 10006, data: null, message: '服务暂未就绪' },
      code: 10006,
      message: '服务暂未就绪',
    },
  ])('converts HTTP $status business failures to ApiError', async ({ status, data, code, message }) => {
    const client = createRequestClient('http://localhost:16301')
    const adapter = failureAdapter(status, data)

    try {
      await client.get('/ready', { adapter })
      throw new Error('request unexpectedly succeeded')
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError)
      expect((error as ApiError).code).toBe(code)
      expect((error as ApiError).message).toBe(message)
      expect((error as ApiError).httpStatus).toBe(status)
    }
  })

  it.each([
    { status: 400, data: { code: 0, data: null, message: 'ok' } },
    { status: 503, data: { code: 10006, data: null, msg: '服务暂未就绪' } },
    { status: 503, data: '<html>service unavailable</html>' },
  ])('rejects invalid HTTP $status error envelopes', async ({ status, data }) => {
    const client = createRequestClient('http://localhost:16301')

    await expect(client.get('/ready', { adapter: failureAdapter(status, data) })).rejects.toBeInstanceOf(ProtocolError)
  })

  it('notifies when a successful HTTP response has a non-zero business code', async () => {
    const client = createRequestClient('http://localhost:16301')
    const adapter: AxiosAdapter = async (config) => successResponse(config, {
      code: 10005,
      data: null,
      message: '用户名已存在',
    })

    await expect(client.post('/api/v1/protected', undefined, { adapter })).rejects.toMatchObject({ code: 10005 })
    expect(notifyErrorMock).toHaveBeenCalledWith(expect.objectContaining({
      title: '请求失败',
      message: '用户名已存在',
      type: 'error',
    }))
  })

  it.each([
    { status: 401, code: 10002, message: '未登录或登录已失效' },
    { status: 403, code: 10003, message: '没有访问权限' },
  ])('notifies HTTP $status failures', async ({ status, code, message }) => {
    const client = createRequestClient('http://localhost:16301')

    await expect(client.post('/api/v1/auth/login', undefined, {
      adapter: failureAdapter(status, { code, data: null, message }),
    })).rejects.toMatchObject({ code, httpStatus: status })
    expect(notifyErrorMock).toHaveBeenCalledWith(expect.objectContaining({
      title: '请求失败',
      message,
      type: 'error',
    }))
  })

  it('does not notify an intermediate 401 that is recovered by Refresh', async () => {
    let refreshed = false
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/api/v1/auth/refresh') {
        refreshed = true
        return successResponse(config, { code: 0, data: { accessToken: 'new-token', expiresIn: 900 }, message: 'ok' })
      }
      if (!refreshed) throw apiFailure(config, 401, 10002, '未登录或登录已失效')
      return successResponse(config, { code: 0, data: { ok: true }, message: 'ok' })
    }
    const client = createRequestClient('http://localhost:16301', adapter)

    await client.get('/api/v1/protected')
    expect(notifyErrorMock).not.toHaveBeenCalled()
  })

  it('adds the in-memory bearer token to protected requests', async () => {
    useAuthStore(pinia).setCredential({ accessToken: 'memory-token', expiresIn: 900 })
    let authorization = ''
    const adapter: AxiosAdapter = async (config) => {
      authorization = AxiosHeaders.from(config.headers).get('Authorization')?.toString() ?? ''
      return successResponse(config, { code: 0, data: { ok: true }, message: 'ok' })
    }
    const client = createRequestClient('http://localhost:16301', adapter)

    await client.get('/api/v1/protected')
    expect(authorization).toBe('Bearer memory-token')
  })

  it('sends the current locale on normal requests', async () => {
    setLocale('en-US')
    let acceptLanguage = ''
    const adapter: AxiosAdapter = async (config) => {
      acceptLanguage = AxiosHeaders.from(config.headers).get('Accept-Language')?.toString() ?? ''
      return successResponse(config, { code: 0, data: { ok: true }, message: 'ok' })
    }
    const client = createRequestClient('http://localhost:16301', adapter)

    await client.get('/api/v1/protected')

    expect(acceptLanguage).toBe('en-US')
  })

  it('attaches platform and device headers to protected requests with a bearer token', async () => {
    useAuthStore(pinia).setCredential({ accessToken: 'stale-token', expiresIn: 900 })
    localStorage.setItem('admin:device-id', '550e8400-e29b-41d4-a716-446655440000')
    let headers = new AxiosHeaders()
    const adapter: AxiosAdapter = async (config) => {
      headers = AxiosHeaders.from(config.headers)
      return successResponse(config, { code: 0, data: {}, message: 'ok' })
    }
    const client = createRequestClient('http://localhost:16301', adapter)

    await client.get('/api/v1/protected')

    expect(headers.get('Accept-Language')).toBe('zh-CN')
    expect(headers.get('X-Auth-Platform')).toBe('admin')
    expect(headers.get('X-Device-ID')).toBe('550e8400-e29b-41d4-a716-446655440000')
    expect(headers.get('Authorization')).toBe('Bearer stale-token')
  })

  it('sends the current locale on the raw refresh request', async () => {
    setLocale('en-US')
    let refreshLanguage = ''
    let refreshed = false
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/api/v1/auth/refresh') {
        refreshLanguage = AxiosHeaders.from(config.headers).get('Accept-Language')?.toString() ?? ''
        refreshed = true
        return successResponse(config, { code: 0, data: { accessToken: 'new-token', expiresIn: 900 }, message: 'ok' })
      }
      if (!refreshed) throw apiFailure(config, 401, 10002, '未登录或登录已失效')
      return successResponse(config, { code: 0, data: { ok: true }, message: 'ok' })
    }
    const client = createRequestClient('http://localhost:16301', adapter)

    await client.get('/api/v1/protected')

    expect(refreshLanguage).toBe('en-US')
  })

  it('coordinates concurrent 401 responses through one refresh', async () => {
    let refreshCalls = 0
    let protectedCalls = 0
    let refreshed = false
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/api/v1/auth/refresh') {
        refreshCalls += 1
        refreshed = true
        return successResponse(config, { code: 0, data: { accessToken: 'new-token', expiresIn: 900 }, message: 'ok' })
      }
      protectedCalls += 1
      if (!refreshed) {
        throw apiFailure(config, 401, 10002, '未登录或登录已失效')
      }
      expect(AxiosHeaders.from(config.headers).get('Authorization')).toBe('Bearer new-token')
      return successResponse(config, { code: 0, data: { ok: true }, message: 'ok' })
    }
    const client = createRequestClient('http://localhost:16301', adapter)

    await Promise.all([
      client.get('/api/v1/protected/1'),
      client.get('/api/v1/protected/2'),
      client.get('/api/v1/protected/3'),
    ])
    expect(refreshCalls).toBe(1)
    expect(protectedCalls).toBe(6)
  })

  it('coordinates concurrent final 401 responses through one notification and callback', async () => {
    let refreshCalls = 0
    const onUnauthorized = vi.fn()
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/api/v1/auth/refresh') refreshCalls += 1
      throw apiFailure(config, 401, 10002, '未登录或登录已失效')
    }
    const client = createRequestClient('http://localhost:16301', adapter, onUnauthorized)

    const results = await Promise.allSettled([
      client.get('/api/v1/protected/1'),
      client.get('/api/v1/protected/2'),
      client.get('/api/v1/protected/3'),
    ])

    expect(results.every((result) => result.status === 'rejected')).toBe(true)
    expect(refreshCalls).toBe(1)
    expect(notifyErrorMock).toHaveBeenCalledTimes(1)
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('notifies a 403 without refreshing or invoking the unauthorized callback', async () => {
    let refreshCalls = 0
    const onUnauthorized = vi.fn()
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/api/v1/auth/refresh') refreshCalls += 1
      throw apiFailure(config, 403, 10003, '没有访问权限')
    }
    const client = createRequestClient('http://localhost:16301', adapter, onUnauthorized)

    await expect(client.get('/api/v1/protected')).rejects.toMatchObject({ code: 10003, httpStatus: 403 })
    expect(refreshCalls).toBe(0)
    expect(notifyErrorMock).toHaveBeenCalledTimes(1)
    expect(onUnauthorized).not.toHaveBeenCalled()
  })

  it('retries each original request at most once', async () => {
    let protectedCalls = 0
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/api/v1/auth/refresh') {
        return successResponse(config, { code: 0, data: { accessToken: 'new-token', expiresIn: 900 }, message: 'ok' })
      }
      protectedCalls += 1
      throw apiFailure(config, 401, 10002, '未登录或登录已失效')
    }
    const client = createRequestClient('http://localhost:16301', adapter)
    await expect(client.get('/api/v1/protected')).rejects.toMatchObject({ code: 10002 })
    expect(protectedCalls).toBe(2)
  })

  it('does not refresh login refresh or logout requests', async () => {
    let refreshCalls = 0
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/api/v1/auth/refresh') {
        refreshCalls += 1
      }
      throw apiFailure(config, 401, 10002, '未登录或登录已失效')
    }
    const client = createRequestClient('http://localhost:16301', adapter)
    for (const url of ['/api/v1/auth/login', '/api/v1/auth/refresh', '/api/v1/auth/logout']) {
      await expect(client.post(url)).rejects.toMatchObject({ code: 10002 })
    }
    expect(refreshCalls).toBe(1)
  })

  it('sets anonymous after a refresh 401', async () => {
    const store = useAuthStore(pinia)
    store.setCredential({ accessToken: 'expired', expiresIn: 900 })
    const adapter: AxiosAdapter = async (config) => {
      throw apiFailure(config, 401, 10002, '未登录或登录已失效')
    }
    const client = createRequestClient('http://localhost:16301', adapter)
    await expect(client.get('/api/v1/protected')).rejects.toMatchObject({ code: 10002 })
    expect(store.status).toBe('anonymous')
    expect(store.accessToken).toBe('')
    expect(notifyErrorMock).toHaveBeenCalledTimes(1)
  })

  it.each([
    {
      name: '503',
      refreshResult: (config: InternalAxiosRequestConfig) => Promise.reject(apiFailure(config, 503, 10006, '服务暂未就绪')),
    },
    {
      name: 'protocol violation',
      refreshResult: async (config: InternalAxiosRequestConfig) => successResponse(config, { code: 0, data: { expiresIn: 900 }, message: 'ok' }),
    },
  ])('sets error after a refresh $name', async ({ refreshResult }) => {
    const store = useAuthStore(pinia)
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/api/v1/auth/refresh') {
        return refreshResult(config)
      }
      throw apiFailure(config, 401, 10002, '未登录或登录已失效')
    }
    const client = createRequestClient('http://localhost:16301', adapter)
    await expect(client.get('/api/v1/protected')).rejects.toBeDefined()
    expect(store.status).toBe('error')
    expect(store.errorMessage).not.toBe('')
    expect(notifyErrorMock).toHaveBeenCalledTimes(1)
  })
})

function failureAdapter(status: number, data: unknown): AxiosAdapter {
  return async (config) => {
    throw new AxiosError(`HTTP ${status}`, AxiosError.ERR_BAD_REQUEST, config, undefined, {
      data,
      status,
      statusText: 'failure',
      headers: {},
      config,
    })
  }
}

function successResponse(config: InternalAxiosRequestConfig, data: unknown) {
  return {
    data,
    status: 200,
    statusText: 'OK',
    headers: new AxiosHeaders(),
    config,
  }
}

function apiFailure(config: InternalAxiosRequestConfig, status: number, code: number, message: string): AxiosError {
  return new AxiosError(`HTTP ${status}`, AxiosError.ERR_BAD_REQUEST, config, undefined, {
    data: { code, data: null, message },
    status,
    statusText: 'failure',
    headers: new AxiosHeaders(),
    config,
  })
}
