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
export const notificationVariantMetadata = [
  { value: 'info', i18nKey: 'notification.variant.info' },
  { value: 'success', i18nKey: 'notification.variant.success' },
  { value: 'warning', i18nKey: 'notification.variant.warning' },
  { value: 'error', i18nKey: 'notification.variant.error' },
] as const satisfies ReadonlyArray<{ value: NotificationVariant; i18nKey: string }>
export const notificationPriorityMetadata = [
  { value: 'normal', i18nKey: 'notification.priority.normal' },
  { value: 'urgent', i18nKey: 'notification.priority.urgent' },
] as const satisfies ReadonlyArray<{ value: NotificationPriority; i18nKey: string }>

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

const variants = new Set<NotificationVariant>(notificationVariantMetadata.map(({ value }) => value))
const priorities = new Set<NotificationPriority>(
  notificationPriorityMetadata.map(({ value }) => value),
)
const links = new Set<NotificationLinkType>(['none', 'internal', 'external'])
const notificationHtmlTags = new Set([
  'a',
  'br',
  'em',
  'h2',
  'h3',
  'li',
  'ol',
  'p',
  'strong',
  'u',
  'ul',
])
const htmlNamespace = 'http://www.w3.org/1999/xhtml'

function enumValue<T extends string>(value: unknown, values: ReadonlySet<T>, context: string): T {
  const parsed = expectString(value, context)
  if (!values.has(parsed as T)) throw new ProtocolError(`${context} is invalid`)
  return parsed as T
}

export function parseNotificationHtml(value: unknown, context = 'contentHtml'): string {
  const content = expectString(value, context)
  if (content.trim() === '' || Array.from(content).length > 16_384) {
    throw new ProtocolError(`${context} is invalid`)
  }

  const parsed = new DOMParser().parseFromString('', 'text/html')
  const template = parsed.createElement('template')
  template.innerHTML = content
  if ((template.content.textContent ?? '').trim() === '') {
    throw new ProtocolError(`${context} is invalid`)
  }

  for (const element of template.content.querySelectorAll('*')) {
    const tag = element.tagName.toLowerCase()
    if (element.namespaceURI !== htmlNamespace || !notificationHtmlTags.has(tag)) {
      throw new ProtocolError(`${context} contains an unsafe element`)
    }
    if (tag !== 'a') {
      if (element.attributes.length !== 0) {
        throw new ProtocolError(`${context} contains an unsafe attribute`)
      }
      continue
    }
    validateNotificationAnchor(element, context)
  }
  return content
}

function validateNotificationAnchor(element: Element, context: string): void {
  if (element.attributes.length === 0) return
  const attributeNames = new Set(Array.from(element.attributes, ({ name }) => name))
  if (
    attributeNames.size !== 3 ||
    !attributeNames.has('href') ||
    !attributeNames.has('target') ||
    !attributeNames.has('rel') ||
    element.getAttribute('target') !== '_blank' ||
    element.getAttribute('rel') !== 'noopener noreferrer'
  ) {
    throw new ProtocolError(`${context} contains an unsafe anchor`)
  }

  const href = element.getAttribute('href') ?? ''
  if (
    href.length > 2_048 ||
    href.trim() !== href ||
    Array.from(href).some((character) => {
      const codePoint = character.codePointAt(0) ?? 0
      return character === '\\' || codePoint <= 0x1f || (codePoint >= 0x7f && codePoint <= 0x9f)
    })
  ) {
    throw new ProtocolError(`${context} contains an unsafe anchor`)
  }
  try {
    const parsed = new URL(href)
    if (
      parsed.protocol !== 'https:' ||
      parsed.hostname === '' ||
      parsed.username !== '' ||
      parsed.password !== ''
    ) {
      throw new ProtocolError(`${context} contains an unsafe anchor`)
    }
  } catch (error: unknown) {
    if (error instanceof ProtocolError) throw error
    throw new ProtocolError(`${context} contains an unsafe anchor`)
  }
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
      contentHtml: parseNotificationHtml(contentHtml),
    }
  })
  const nextBeforeId =
    row.nextBeforeId === null ? null : expectInteger(row.nextBeforeId, 'nextBeforeId')
  return { items, nextBeforeId }
}

export async function getNotificationSummary(): Promise<NotificationSummary> {
  return parseNotificationSummary(
    await request({ method: 'GET', url: '/api/v1/message/notification/summary' }),
  )
}
export async function listNotifications(params: NotificationQuery): Promise<NotificationList> {
  return parseNotificationList(
    await request({ method: 'GET', url: '/api/v1/message/notification', params }),
  )
}
export async function readNotification(id: number): Promise<void> {
  await request({ method: 'PATCH', url: `/api/v1/message/notification/${id}/read` })
}
export async function readAllNotifications(): Promise<void> {
  await request({ method: 'PATCH', url: '/api/v1/message/notification/read-all' })
}
export async function deleteNotification(id: number): Promise<void> {
  await request({ method: 'DELETE', url: `/api/v1/message/notification/${id}` })
}
