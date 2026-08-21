import { describe, expect, it } from 'vitest'
import { parseSessionPage, parseSessionStats } from './session.contract'

const item = { id: 1, userId: 2, username: 'admin', platform: 'admin', deviceId: 'device', clientIp: '127.0.0.1', userAgent: 'test', createdAt: '2026-08-21T00:00:00Z', updatedAt: '2026-08-21T00:00:00Z', refreshExpiresAt: '2026-08-22T00:00:00Z', revokedAt: null, status: 'active', isCurrent: true }
describe('session contracts', () => {
  it('parses strict session page and stats', () => {
    expect(parseSessionPage({ list: [item], total: 1, page: 1, pageSize: 20 }).list[0].status).toBe('active')
		expect(parseSessionPage({ list: [item], total: 1, page: 1, pageSize: 20 }).list[0].isCurrent).toBe(true)
    expect(parseSessionStats({ activeTotal: 1, platforms: { admin: 1 } })).toEqual({ activeTotal: 1, platforms: { admin: 1 } })
  })
  it('rejects unknown fields and invalid status', () => {
    expect(() => parseSessionPage({ list: [{ ...item, status: 'bad' }], total: 1, page: 1, pageSize: 20 })).toThrow()
    expect(() => parseSessionStats({ activeTotal: 1, platforms: {}, msg: 'bad' })).toThrow()
  })
})
