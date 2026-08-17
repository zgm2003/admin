import type { AxiosAdapter } from 'axios'
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
    const networkError = new Error('connection refused')
    const adapter: AxiosAdapter = async () => Promise.reject(networkError)
    const client = createRequestClient('http://localhost:16301')

    await expect(client.get('/health', { adapter })).rejects.toBe(networkError)
  })
})
