import { describe, expect, it } from 'vitest'
import { parseOperationLogPage } from '@src/api/operation-log.contract'

describe('operation log contracts', () => {
  it('parses a strict page with redacted summaries', () => {
    const result = parseOperationLogPage({ list: [{ id: 1, requestId: 'r', userId: null, sessionId: null, platform: 'admin', method: 'PUT', route: '/api/v1/users/:id', module: 'user', action: 'user.update', clientIp: '127.0.0.1', userAgent: 'test', statusCode: 200, isSuccess: 1, latencyMs: 3, requestData: { password: '***' }, responseData: { code: 0 }, createdAt: '2026-08-21T00:00:00Z', updatedAt: '2026-08-21T00:00:00Z' }], total: 1, page: 1, pageSize: 20 })
    expect(result.list[0].requestData).toEqual({ password: '***' })
  })
  it('rejects compatibility fields and invalid success values', () => {
    expect(() => parseOperationLogPage({ list: [], total: 0, page: 1, pageSize: 20, msg: 'bad' })).toThrow()
    expect(() => parseOperationLogPage({ list: [{ id: 1, requestId: 'r', userId: null, sessionId: null, platform: 'admin', method: 'PUT', route: '/x', module: 'x', action: 'x', clientIp: '', userAgent: '', statusCode: 200, isSuccess: 2, latencyMs: 0, requestData: null, responseData: null, createdAt: 'x', updatedAt: 'x' }], total: 1, page: 1, pageSize: 20 })).toThrow()
  })
})
