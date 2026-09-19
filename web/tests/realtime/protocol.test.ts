import { describe, expect, it } from 'vitest'

import { parseRealtimeEnvelope } from '@/realtime/protocol'

const base = {
  eventId: '2ec9ca86-e265-4551-a15a-05c333326db0',
  sequence: 2,
  occurredAt: '2026-09-18T12:00:00Z',
  durability: 'durable',
}

describe('realtime protocol', () => {
  it.each([
    ['realtime.connected.v1', { sessionId: 3 }],
    ['realtime.pong.v1', {}],
    ['realtime.resumed.v1', { throughSequence: 2 }],
    ['realtime.resyncRequired.v1', { throughSequence: 2 }],
    ['realtime.error.v1', {}],
    [
      'notification.created.v1',
      {
        notificationId: 4,
        title: 't',
        summary: 's',
        variant: 'info',
        priority: 'normal',
        linkType: 'none',
        link: '',
        publishedAt: '2026-09-18T12:00:00Z',
      },
    ],
    ['notification.stateChanged.v1', { notificationId: 4, operation: 'read' }],
  ])('parses %s', (type, data) =>
    expect(parseRealtimeEnvelope({ ...base, type, data }).type).toBe(type),
  )

  it('rejects rich content and unknown fields', () => {
    expect(() =>
      parseRealtimeEnvelope({
        ...base,
        type: 'notification.created.v1',
        data: {
          notificationId: 4,
          title: 't',
          summary: 's',
          variant: 'info',
          priority: 'normal',
          linkType: 'none',
          link: '',
          publishedAt: '2026-09-18T12:00:00Z',
          contentHtml: '<b>x</b>',
        },
      }),
    ).toThrow()
    expect(() =>
      parseRealtimeEnvelope({ ...base, type: 'realtime.pong.v1', data: {}, extra: true }),
    ).toThrow()
  })
})
