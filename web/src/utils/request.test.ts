import { AxiosError, type AxiosAdapter } from 'axios'
import { describe, expect, it } from 'vitest'

import {
  ApiError,
  ProtocolError,
  createRequestClient,
  unwrapEnvelope,
} from './request'

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
  it('requires an explicit API base URL', () => {
    expect(() => createRequestClient('  ')).toThrow(ProtocolError)
  })

  it('returns network errors unchanged', async () => {
    const networkError = new AxiosError('connection refused', AxiosError.ERR_NETWORK)
    const adapter: AxiosAdapter = async () => Promise.reject(networkError)
    const client = createRequestClient('http://localhost:16301')

    await expect(client.get('/health', { adapter })).rejects.toBe(networkError)
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
