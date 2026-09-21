import {
  expectArray,
  expectExactKeys,
  expectInteger,
  expectISODate,
  expectString,
} from '@/api/protocol'
import {
  notificationPriorityMetadata,
  notificationVariantMetadata,
  parseNotificationHtml,
  type NotificationLinkType,
  type NotificationPriority,
  type NotificationVariant,
} from '@/api/message/notification'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

export type NotificationAudience = 'user' | 'role' | 'platform'
export const notificationTaskAudienceMetadata = [
  { value: 'platform', i18nKey: 'notificationTask.audience.platform' },
  { value: 'user', i18nKey: 'notificationTask.audience.user' },
  { value: 'role', i18nKey: 'notificationTask.audience.role' },
] as const satisfies ReadonlyArray<{ value: NotificationAudience; i18nKey: string }>
export const notificationTaskVariantMetadata = notificationVariantMetadata.map(({ value }) => ({
  value,
  i18nKey: `notificationTask.variant.${value}`,
})) satisfies ReadonlyArray<{ value: NotificationVariant; i18nKey: string }>
export const notificationTaskPriorityMetadata = notificationPriorityMetadata
export const notificationTaskLinkTypeMetadata = [
  { value: 'none', i18nKey: 'notificationTask.linkType.none' },
  { value: 'internal', i18nKey: 'notificationTask.linkType.internal' },
  { value: 'external', i18nKey: 'notificationTask.linkType.external' },
] as const satisfies ReadonlyArray<{ value: NotificationLinkType; i18nKey: string }>
export const NotificationTaskStatus = {
  Draft: 1,
  Scheduled: 2,
  Queued: 3,
  Processing: 4,
  Completed: 5,
  Failed: 6,
  Canceled: 7,
} as const
export type NotificationTaskStatus =
  (typeof NotificationTaskStatus)[keyof typeof NotificationTaskStatus]
export const notificationTaskStatusMetadata = [
  {
    value: NotificationTaskStatus.Draft,
    i18nKey: 'notificationTask.status.draft',
    tagType: 'info',
  },
  {
    value: NotificationTaskStatus.Scheduled,
    i18nKey: 'notificationTask.status.scheduled',
    tagType: 'warning',
  },
  {
    value: NotificationTaskStatus.Queued,
    i18nKey: 'notificationTask.status.queued',
    tagType: 'primary',
  },
  {
    value: NotificationTaskStatus.Processing,
    i18nKey: 'notificationTask.status.processing',
    tagType: 'primary',
  },
  {
    value: NotificationTaskStatus.Completed,
    i18nKey: 'notificationTask.status.completed',
    tagType: 'success',
  },
  {
    value: NotificationTaskStatus.Failed,
    i18nKey: 'notificationTask.status.failed',
    tagType: 'danger',
  },
  {
    value: NotificationTaskStatus.Canceled,
    i18nKey: 'notificationTask.status.canceled',
    tagType: 'info',
  },
] as const
const statusValues = new Set<NotificationTaskStatus>(
  notificationTaskStatusMetadata.map((item) => item.value),
)
export interface NotificationTask {
  id: number
  platformId: number
  platformName: string
  notificationId: number | null
  title: string
  contentHtml: string
  summary: string
  variant: NotificationVariant
  priority: NotificationPriority
  linkType: NotificationLinkType
  link: string
  audienceType: NotificationAudience
  targetIds: number[]
  scheduledAt: string | null
  audienceMaxUserId: number | null
  submittedAt: string | null
  publishedAt: string | null
  completedAt: string | null
  canceledAt: string | null
  failedAt: string | null
  failureMessage: string | null
  status: NotificationTaskStatus
  generatedCount: number
  createdBy: number
  createdAt: string
  updatedAt: string
}
export interface NotificationTaskListItem {
  id: number
  platformId: number
  platformName: string
  title: string
  variant: NotificationVariant
  priority: NotificationPriority
  audienceType: NotificationAudience
  scheduledAt: string | null
  submittedAt: string | null
  completedAt: string | null
  status: NotificationTaskStatus
  generatedCount: number
  updatedAt: string
}
export interface NotificationTaskInput {
  platformId: number
  title: string
  contentHtml: string
  variant: NotificationVariant
  priority: NotificationPriority
  linkType: NotificationLinkType
  link: string
  audienceType: NotificationAudience
  targetIds: number[]
  scheduledAt: string | null
}
export interface NotificationTaskOption {
  id: number
  label: string
}
export interface NotificationTaskOptions {
  items: NotificationTaskOption[]
  nextAfterId: number | null
}
export interface NotificationTaskPage {
  list: NotificationTaskListItem[]
  total: number
  page: number
  pageSize: number
}
export interface NotificationTaskListQuery {
  page: number
  pageSize: number
  platformId?: number
  status?: NotificationTaskStatus
  audienceType?: NotificationAudience
  keyword?: string
  from?: string
  to?: string
}

const variants = new Set(notificationTaskVariantMetadata.map(({ value }) => value))
const priorities = new Set(notificationTaskPriorityMetadata.map(({ value }) => value))
const links = new Set(notificationTaskLinkTypeMetadata.map(({ value }) => value))
const audiences = new Set(notificationTaskAudienceMetadata.map(({ value }) => value))
function oneOf<T extends string>(value: unknown, values: Set<string>, context: string): T {
  const parsed = expectString(value, context)
  if (!values.has(parsed)) throw new ProtocolError(`${context} is invalid`)
  return parsed as T
}
function status(value: unknown, context: string): NotificationTaskStatus {
  const parsed = expectInteger(value, context)
  if (!statusValues.has(parsed as NotificationTaskStatus))
    throw new ProtocolError(`${context} is invalid`)
  return parsed as NotificationTaskStatus
}
function nullableInteger(value: unknown, context: string): number | null {
  return value === null ? null : expectInteger(value, context)
}
function nullableDate(value: unknown, context: string): string | null {
  return value === null ? null : expectISODate(value, context)
}
function nullableString(value: unknown, context: string): string | null {
  return value === null ? null : expectString(value, context)
}

export function parseNotificationTask(value: unknown): NotificationTask {
  const r = expectExactKeys(
    value,
    [
      'id',
      'platformId',
      'platformName',
      'notificationId',
      'title',
      'contentHtml',
      'summary',
      'variant',
      'priority',
      'linkType',
      'link',
      'audienceType',
      'targetIds',
      'scheduledAt',
      'audienceMaxUserId',
      'submittedAt',
      'publishedAt',
      'completedAt',
      'canceledAt',
      'failedAt',
      'failureMessage',
      'status',
      'generatedCount',
      'createdBy',
      'createdAt',
      'updatedAt',
    ],
    'notification task',
  )
  return {
    id: expectInteger(r.id, 'id'),
    platformId: expectInteger(r.platformId, 'platformId'),
    platformName: expectString(r.platformName, 'platformName'),
    notificationId: nullableInteger(r.notificationId, 'notificationId'),
    title: expectString(r.title, 'title'),
    contentHtml: parseNotificationHtml(r.contentHtml),
    summary: expectString(r.summary, 'summary'),
    variant: oneOf(r.variant, variants, 'variant'),
    priority: oneOf(r.priority, priorities, 'priority'),
    linkType: oneOf(r.linkType, links, 'linkType'),
    link: expectString(r.link, 'link'),
    audienceType: oneOf(r.audienceType, audiences, 'audienceType'),
    targetIds: expectArray(r.targetIds, 'targetIds').map((v) => expectInteger(v, 'targetId')),
    scheduledAt: nullableDate(r.scheduledAt, 'scheduledAt'),
    audienceMaxUserId: nullableInteger(r.audienceMaxUserId, 'audienceMaxUserId'),
    submittedAt: nullableDate(r.submittedAt, 'submittedAt'),
    publishedAt: nullableDate(r.publishedAt, 'publishedAt'),
    completedAt: nullableDate(r.completedAt, 'completedAt'),
    canceledAt: nullableDate(r.canceledAt, 'canceledAt'),
    failedAt: nullableDate(r.failedAt, 'failedAt'),
    failureMessage: nullableString(r.failureMessage, 'failureMessage'),
    status: status(r.status, 'status'),
    generatedCount: expectInteger(r.generatedCount, 'generatedCount'),
    createdBy: expectInteger(r.createdBy, 'createdBy'),
    createdAt: expectISODate(r.createdAt, 'createdAt'),
    updatedAt: expectISODate(r.updatedAt, 'updatedAt'),
  }
}
export function parseNotificationTaskListItem(value: unknown): NotificationTaskListItem {
  const r = expectExactKeys(
    value,
    [
      'id',
      'platformId',
      'platformName',
      'title',
      'variant',
      'priority',
      'audienceType',
      'scheduledAt',
      'submittedAt',
      'completedAt',
      'status',
      'generatedCount',
      'updatedAt',
    ],
    'notification task list item',
  )
  return {
    id: expectInteger(r.id, 'id'),
    platformId: expectInteger(r.platformId, 'platformId'),
    platformName: expectString(r.platformName, 'platformName'),
    title: expectString(r.title, 'title'),
    variant: oneOf(r.variant, variants, 'variant'),
    priority: oneOf(r.priority, priorities, 'priority'),
    audienceType: oneOf(r.audienceType, audiences, 'audienceType'),
    scheduledAt: nullableDate(r.scheduledAt, 'scheduledAt'),
    submittedAt: nullableDate(r.submittedAt, 'submittedAt'),
    completedAt: nullableDate(r.completedAt, 'completedAt'),
    status: status(r.status, 'status'),
    generatedCount: expectInteger(r.generatedCount, 'generatedCount'),
    updatedAt: expectISODate(r.updatedAt, 'updatedAt'),
  }
}
export function parseNotificationTaskOptions(value: unknown): NotificationTaskOptions {
  const r = expectExactKeys(value, ['items', 'nextAfterId'], 'notification task options')
  return {
    items: expectArray(r.items, 'items').map((item) => {
      const option = expectExactKeys(item, ['id', 'label'], 'option')
      return { id: expectInteger(option.id, 'id'), label: expectString(option.label, 'label') }
    }),
    nextAfterId: nullableInteger(r.nextAfterId, 'nextAfterId'),
  }
}
export function parseNotificationTaskPage(value: unknown): NotificationTaskPage {
  const r = expectExactKeys(value, ['list', 'total', 'page', 'pageSize'], 'notification task page')
  return {
    list: expectArray(r.list, 'list').map(parseNotificationTaskListItem),
    total: expectInteger(r.total, 'total'),
    page: expectInteger(r.page, 'page'),
    pageSize: expectInteger(r.pageSize, 'pageSize'),
  }
}

const base = '/api/admin/v1/message/notificationtask'
export async function listNotificationTasks(
  params: NotificationTaskListQuery,
): Promise<NotificationTaskPage> {
  return parseNotificationTaskPage(
    await request({
      method: 'GET',
      url: base,
      params,
    }),
  )
}
export async function getNotificationTask(id: number): Promise<NotificationTask> {
  return parseNotificationTask(await request({ method: 'GET', url: `${base}/${id}` }))
}
export async function getNotificationTaskForUpdate(id: number): Promise<NotificationTask> {
  return parseNotificationTask(await request({ method: 'GET', url: `${base}/${id}/edit` }))
}
export async function createNotificationTask(
  data: NotificationTaskInput,
): Promise<NotificationTask> {
  return parseNotificationTask(await request({ method: 'POST', url: base, data }))
}
export async function updateNotificationTask(
  id: number,
  data: NotificationTaskInput,
): Promise<NotificationTask> {
  return parseNotificationTask(await request({ method: 'PUT', url: `${base}/${id}`, data }))
}
export async function deleteNotificationTask(id: number): Promise<void> {
  await request({ method: 'DELETE', url: `${base}/${id}` })
}
export async function commandNotificationTask(
  id: number,
  command: 'submit' | 'cancel' | 'copy',
): Promise<NotificationTask> {
  return parseNotificationTask(await request({ method: 'POST', url: `${base}/${id}/${command}` }))
}
export async function listNotificationTaskOptions(
  intent: 'create' | 'update',
  kind: 'platform' | 'user' | 'role',
  params: { platformId?: number; keyword?: string; afterId?: number; limit?: number },
): Promise<NotificationTaskOptions> {
  return parseNotificationTaskOptions(
    await request({ method: 'GET', url: `${base}/${intent}/option/${kind}`, params }),
  )
}
