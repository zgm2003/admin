import { expectExactKeys, expectInteger, expectISODate, expectString } from '@/api/protocol'
import {
  type NotificationLinkType,
  type NotificationPriority,
  type NotificationVariant,
} from '@/api/message/notification'
import { ProtocolError } from '@/types/http'

export type RealtimeEvent =
  | { type: 'realtime.connected.v1'; data: { sessionId: number } }
  | { type: 'realtime.pong.v1'; data: Record<string, never> }
  | {
      type: 'realtime.resumed.v1' | 'realtime.resyncRequired.v1'
      data: { throughSequence: number }
    }
  | { type: 'realtime.error.v1'; data: Record<string, never> }
  | {
      type: 'notification.created.v1'
      data: {
        notificationId: number
        title: string
        summary: string
        variant: NotificationVariant
        priority: NotificationPriority
        linkType: NotificationLinkType
        link: string
        publishedAt: string
      }
    }
  | {
      type: 'notification.stateChanged.v1'
      data: { notificationId: number; operation: 'read' | 'readAll' | 'delete' }
    }
export type RealtimeEnvelope = RealtimeEvent & {
  eventId: string
  requestId?: string
  sequence: number
  occurredAt: string
  durability: 'durable'
}

function empty(value: unknown): Record<string, never> {
  expectExactKeys(value, [], 'event data')
  return {}
}
function positive(value: unknown, context: string): number {
  const n = expectInteger(value, context)
  if (n <= 0) throw new ProtocolError(`${context} must be positive`)
  return n
}
function nonnegative(value: unknown, context: string): number {
  const n = expectInteger(value, context)
  if (n < 0) throw new ProtocolError(`${context} must be nonnegative`)
  return n
}
function metadata(value: unknown) {
  const raw =
    value !== null && typeof value === 'object' && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : {}
  const keys = Object.prototype.hasOwnProperty.call(raw, 'requestId')
    ? ['eventId', 'type', 'requestId', 'sequence', 'occurredAt', 'durability', 'data']
    : ['eventId', 'type', 'sequence', 'occurredAt', 'durability', 'data']
  const r = expectExactKeys(value, keys, 'realtime envelope')
  const eventId = expectString(r.eventId, 'eventId')
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(eventId))
    throw new ProtocolError('eventId must be UUID')
  const requestId = Object.prototype.hasOwnProperty.call(r, 'requestId')
    ? expectString(r.requestId, 'requestId')
    : undefined
  if (r.durability !== 'durable') throw new ProtocolError('durability is invalid')
  return {
    r,
    eventId,
    requestId,
    sequence: positive(r.sequence, 'sequence'),
    occurredAt: expectISODate(r.occurredAt, 'occurredAt'),
    durability: 'durable' as const,
  }
}
export function parseRealtimeEnvelope(value: unknown): RealtimeEnvelope {
  const m = metadata(value)
  const type = expectString(m.r.type, 'type')
  let event: RealtimeEvent
  if (type === 'realtime.connected.v1') {
    const d = expectExactKeys(m.r.data, ['sessionId'], 'connected data')
    event = { type, data: { sessionId: positive(d.sessionId, 'sessionId') } }
  } else if (type === 'realtime.pong.v1' || type === 'realtime.error.v1')
    event = { type, data: empty(m.r.data) }
  else if (type === 'realtime.resumed.v1' || type === 'realtime.resyncRequired.v1') {
    const d = expectExactKeys(m.r.data, ['throughSequence'], 'resume data')
    event = { type, data: { throughSequence: nonnegative(d.throughSequence, 'throughSequence') } }
  } else if (type === 'notification.created.v1') {
    const d = expectExactKeys(
      m.r.data,
      [
        'notificationId',
        'title',
        'summary',
        'variant',
        'priority',
        'linkType',
        'link',
        'publishedAt',
      ],
      'notification created data',
    )
    const variant = expectString(d.variant, 'variant')
    const priority = expectString(d.priority, 'priority')
    const linkType = expectString(d.linkType, 'linkType')
    if (
      !['info', 'success', 'warning', 'error'].includes(variant) ||
      !['normal', 'urgent'].includes(priority) ||
      !['none', 'internal', 'external'].includes(linkType)
    )
      throw new ProtocolError('notification event enum is invalid')
    event = {
      type,
      data: {
        notificationId: positive(d.notificationId, 'notificationId'),
        title: expectString(d.title, 'title'),
        summary: expectString(d.summary, 'summary'),
        variant: variant as NotificationVariant,
        priority: priority as NotificationPriority,
        linkType: linkType as NotificationLinkType,
        link: expectString(d.link, 'link'),
        publishedAt: expectISODate(d.publishedAt, 'publishedAt'),
      },
    }
  } else if (type === 'notification.stateChanged.v1') {
    const d = expectExactKeys(m.r.data, ['notificationId', 'operation'], 'notification state data')
    const operation = expectString(d.operation, 'operation')
    if (!['read', 'readAll', 'delete'].includes(operation))
      throw new ProtocolError('operation is invalid')
    event = {
      type,
      data: {
        notificationId: positive(d.notificationId, 'notificationId'),
        operation: operation as 'read' | 'readAll' | 'delete',
      },
    }
  } else throw new ProtocolError('unsupported realtime event type')
  const common = {
    eventId: m.eventId,
    sequence: m.sequence,
    occurredAt: m.occurredAt,
    durability: m.durability,
  }
  return m.requestId === undefined
    ? { ...common, ...event }
    : { ...common, requestId: m.requestId, ...event }
}
