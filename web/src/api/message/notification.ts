import {
  expectArray,
  expectBoolean,
  expectExactKeys,
  expectInteger,
  expectISODate,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

export type NotificationVariant = 'info' | 'success' | 'warning' | 'error'
export type NotificationPriority = 'normal' | 'urgent'
export type NotificationLinkType = 'none' | 'internal' | 'external'

export interface NotificationRecent {
  id: number
  title: string
  summary: string
  variant: NotificationVariant
  priority: NotificationPriority
  linkType: NotificationLinkType
  link: string
  publishedAt: string
  isRead: boolean
}
export interface NotificationItem extends NotificationRecent {
  contentHtml: string
}
export interface NotificationSummary {
  unreadCount: number
  recent: NotificationRecent[]
}
export interface NotificationList {
  items: NotificationItem[]
  nextBeforeId: number | null
}
export interface NotificationQuery {
  beforeId?: number
  limit?: number
  filter?: 'all' | 'unread'
  variant?: NotificationVariant
  priority?: NotificationPriority
}

const variants = new Set<NotificationVariant>(['info', 'success', 'warning', 'error'])
const priorities = new Set<NotificationPriority>(['normal', 'urgent'])
const links = new Set<NotificationLinkType>(['none', 'internal', 'external'])

function enumValue<T extends string>(value: unknown, values: ReadonlySet<T>, context: string): T {
  const parsed = expectString(value, context)
  if (!values.has(parsed as T)) throw new ProtocolError(`${context} is invalid`)
  return parsed as T
}

export function parseNotificationRecent(value: unknown): NotificationRecent {
  const row = expectExactKeys(
    value,
    ['id', 'title', 'summary', 'variant', 'priority', 'linkType', 'link', 'publishedAt', 'isRead'],
    'notification recent',
  )
  return {
    id: expectInteger(row.id, 'id'),
    title: expectString(row.title, 'title'),
    summary: expectString(row.summary, 'summary'),
    variant: enumValue(row.variant, variants, 'variant'),
    priority: enumValue(row.priority, priorities, 'priority'),
    linkType: enumValue(row.linkType, links, 'linkType'),
    link: expectString(row.link, 'link'),
    publishedAt: expectISODate(row.publishedAt, 'publishedAt'),
    isRead: expectBoolean(row.isRead, 'isRead'),
  }
}

export function parseNotificationSummary(value: unknown): NotificationSummary {
  const row = expectExactKeys(value, ['unreadCount', 'recent'], 'notification summary')
  const unreadCount = expectInteger(row.unreadCount, 'unreadCount')
  if (unreadCount < 0) throw new ProtocolError('unreadCount must be nonnegative')
  return { unreadCount, recent: expectArray(row.recent, 'recent').map(parseNotificationRecent) }
}

export function parseNotificationList(value: unknown): NotificationList {
  const row = expectExactKeys(value, ['items', 'nextBeforeId'], 'notification list')
  const items = expectArray(row.items, 'items').map((item) => {
    const detail = expectExactKeys(
      item,
      [
        'id',
        'title',
        'contentHtml',
        'summary',
        'variant',
        'priority',
        'linkType',
        'link',
        'publishedAt',
        'isRead',
      ],
      'notification item',
    )
    const { contentHtml, ...recent } = detail
    return {
      ...parseNotificationRecent(recent),
      contentHtml: expectString(contentHtml, 'contentHtml'),
    }
  })
  const nextBeforeId =
    row.nextBeforeId === null ? null : expectInteger(row.nextBeforeId, 'nextBeforeId')
  return { items, nextBeforeId }
}

export async function getNotificationSummary(): Promise<NotificationSummary> {
  return parseNotificationSummary(
    await request<unknown>({ method: 'GET', url: '/api/v1/message/notification/summary' }),
  )
}
export async function listNotifications(params: NotificationQuery): Promise<NotificationList> {
  return parseNotificationList(
    await request<unknown>({ method: 'GET', url: '/api/v1/message/notification', params }),
  )
}
export async function readNotification(id: number): Promise<void> {
  await request<unknown>({ method: 'PATCH', url: `/api/v1/message/notification/${id}/read` })
}
export async function readAllNotifications(): Promise<void> {
  await request<unknown>({ method: 'PATCH', url: '/api/v1/message/notification/read-all' })
}
export async function deleteNotification(id: number): Promise<void> {
  await request<unknown>({ method: 'DELETE', url: `/api/v1/message/notification/${id}` })
}
