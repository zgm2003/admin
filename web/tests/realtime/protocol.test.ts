import { describe, expect, it } from 'vitest'

import { buildRealtimeWebSocketURL, parseRealtimeEnvelope } from '@/realtime/protocol'

const base = {
  eventId: '2ec9ca86-e265-4551-a15a-05c333326db0',
  sequence: 2,
  occurredAt: '2026-09-18T12:00:00Z',
  durability: 'durable',
}

describe('realtime protocol', () => {
  it('builds a same-origin WebSocket URL without losing the ticket', () => {
    expect(buildRealtimeWebSocketURL('a b', new URL('https://admin.test/x')).toString()).toBe(
      'wss://admin.test/api/v1/realtime/ws?ticket=a+b',
    )
  })

  it('preserves backend notification display values without a business allow-list', () => {
    const data = {
      notificationId: 4,
      title: 't',
      summary: 's',
      variant: 'new-variant',
      priority: 'new-priority',
      linkType: 'new-link',
      link: '',
      publishedAt: '2026-09-18T12:00:00Z',
    }
    expect(parseRealtimeEnvelope({ ...base, type: 'notification.created.v1', data }).data).toEqual(
      data,
    )
  })
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
    [
      'notification.stateChanged.v1',
      { kind: 'read', notificationId: 4, readThroughNotificationId: null },
    ],
    [
      'notification.stateChanged.v1',
      { kind: 'readAll', notificationId: null, readThroughNotificationId: 4 },
    ],
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

    expect(() =>
      parseRealtimeEnvelope({
        ...base,
        type: 'notification.stateChanged.v1',
        data: { notificationId: 4, operation: 'read' },
      }),
    ).toThrow()
  })
})
