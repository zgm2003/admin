import { describe, expect, it, vi } from 'vitest'
import {
  buildRealtimeWebSocketURL,
  parseRealtimeTicket,
  requestRealtimeTicket,
} from '@/api/realtime'
import * as requestModule from '@/utils/request'

describe('realtime api', () => {
  it('strictly requests a ticket', async () => {
    vi.spyOn(requestModule, 'request').mockResolvedValue({
      ticket: 'a b',
      expiresAt: '2026-09-18T12:00:00Z',
    })
    expect((await requestRealtimeTicket()).ticket).toBe('a b')
  })
  it.each([
    {},
    { ticket: 'abc' },
    { ticket: '', expiresAt: '2026-09-18T12:00:00Z' },
    { ticket: 'abc', expiresAt: 'invalid' },
    { ticket: 'abc', expiresAt: '2026-09-18T12:00:00Z', extra: true },
  ])('rejects invalid ticket payload %#', (value) => {
    expect(() => parseRealtimeTicket(value)).toThrow()
  })
  it('builds same-origin ws URL', () =>
    expect(buildRealtimeWebSocketURL('a b', new URL('https://admin.test/x')).toString()).toBe(
      'wss://admin.test/api/v1/realtime/ws?ticket=a+b',
    ))
})
