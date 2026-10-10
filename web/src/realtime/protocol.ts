import {
  type NotificationLinkType,
  type NotificationPriority,
  type NotificationVariant,
} from '@/api/message/notification'
import { ProtocolError } from '@/types/http'

function eventFields(
  value: unknown,
  keys: readonly string[],
  context: string,
): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new ProtocolError(`${context} must be an event object`)
  }
  const fields = value as Record<string, unknown>
  if (
    Object.keys(fields).length !== keys.length ||
    keys.some((key) => !Object.hasOwn(fields, key))
  ) {
    throw new ProtocolError(`${context} has invalid event fields`)
  }
  return fields
}

function eventText(value: unknown, context: string): string {
  if (typeof value !== 'string') throw new ProtocolError(`${context} must be text`)
  return value
}

function eventInteger(value: unknown, context: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) {
    throw new ProtocolError(`${context} must be a safe event integer`)
  }
  return value
}

function eventTime(value: unknown, context: string): string {
  const timestamp = eventText(value, context)
  if (
    !/^\d{4}-\d{2}-\d{2}T.+(?:Z|[+-]\d{2}:\d{2})$/.test(timestamp) ||
    Number.isNaN(Date.parse(timestamp))
  ) {
    throw new ProtocolError(`${context} must be an event timestamp`)
  }
  return timestamp
}

export function buildRealtimeWebSocketURL(
  ticket: string,
  location: Pick<Location, 'protocol' | 'host'> | URL = window.location,
): URL {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(`${protocol}//${location.host}/api/v1/realtime/ws`)
  url.searchParams.set('ticket', ticket)
  return url
}

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
      data:
        | { kind: 'read' | 'delete'; notificationId: number; readThroughNotificationId: null }
        | { kind: 'readAll'; notificationId: null; readThroughNotificationId: number }
    }
export type RealtimeEnvelope = RealtimeEvent & {
  eventId: string
  requestId?: string
  sequence: number
  occurredAt: string
  durability: 'durable'
}

function empty(value: unknown): Record<string, never> {
  eventFields(value, [], 'event data')
  return {}
}
function positive(value: unknown, context: string): number {
  const n = eventInteger(value, context)
  if (n <= 0) throw new ProtocolError(`${context} must be positive`)
  return n
}
function nonnegative(value: unknown, context: string): number {
  const n = eventInteger(value, context)
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
  const r = eventFields(value, keys, 'realtime envelope')
  const eventId = eventText(r.eventId, 'eventId')
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(eventId))
    throw new ProtocolError('eventId must be UUID')
  const requestId = Object.prototype.hasOwnProperty.call(r, 'requestId')
    ? eventText(r.requestId, 'requestId')
    : undefined
  if (r.durability !== 'durable') throw new ProtocolError('durability is invalid')
  return {
    r,
    eventId,
    requestId,
    sequence: positive(r.sequence, 'sequence'),
    occurredAt: eventTime(r.occurredAt, 'occurredAt'),
    durability: 'durable' as const,
  }
}
export function parseRealtimeEnvelope(value: unknown): RealtimeEnvelope {
  const m = metadata(value)
  const type = eventText(m.r.type, 'type')
  let event: RealtimeEvent
  if (type === 'realtime.connected.v1') {
    const d = eventFields(m.r.data, ['sessionId'], 'connected data')
    event = { type, data: { sessionId: positive(d.sessionId, 'sessionId') } }
  } else if (type === 'realtime.pong.v1' || type === 'realtime.error.v1')
    event = { type, data: empty(m.r.data) }
  else if (type === 'realtime.resumed.v1' || type === 'realtime.resyncRequired.v1') {
    const d = eventFields(m.r.data, ['throughSequence'], 'resume data')
    event = { type, data: { throughSequence: nonnegative(d.throughSequence, 'throughSequence') } }
  } else if (type === 'notification.created.v1') {
    const d = eventFields(
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
    const variant = eventText(d.variant, 'variant')
    const priority = eventText(d.priority, 'priority')
    const linkType = eventText(d.linkType, 'linkType')
    event = {
      type,
      data: {
        notificationId: positive(d.notificationId, 'notificationId'),
        title: eventText(d.title, 'title'),
        summary: eventText(d.summary, 'summary'),
        variant: variant as NotificationVariant,
        priority: priority as NotificationPriority,
        linkType: linkType as NotificationLinkType,
        link: eventText(d.link, 'link'),
        publishedAt: eventTime(d.publishedAt, 'publishedAt'),
      },
    }
  } else if (type === 'notification.stateChanged.v1') {
    const d = eventFields(
      m.r.data,
      ['kind', 'notificationId', 'readThroughNotificationId'],
      'notification state data',
    )
    const kind = eventText(d.kind, 'kind')
    if (kind === 'readAll') {
      if (d.notificationId !== null) throw new ProtocolError('readAll notificationId must be null')
      event = {
        type,
        data: {
          kind,
          notificationId: null,
          readThroughNotificationId: positive(
            d.readThroughNotificationId,
            'readThroughNotificationId',
          ),
        },
      }
    } else if (kind === 'read' || kind === 'delete') {
      if (d.readThroughNotificationId !== null)
        throw new ProtocolError(`${kind} readThroughNotificationId must be null`)
      event = {
        type,
        data: {
          kind,
          notificationId: positive(d.notificationId, 'notificationId'),
          readThroughNotificationId: null,
        },
      }
    } else throw new ProtocolError('kind is invalid')
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
