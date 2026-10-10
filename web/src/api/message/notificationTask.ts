import {
  type NotificationLinkType,
  type NotificationPriority,
  type NotificationVariant,
} from '@/api/message/notification'
import { request } from '@/utils/request'

export type NotificationAudience = 'user' | 'role' | 'platform'

export type NotificationTaskStatus = number

export interface NotificationTaskActions {
  edit: boolean
  delete: boolean
  submit: boolean
  cancel: boolean
  copy: boolean
}

export interface NotificationTask {
  actions: NotificationTaskActions
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
  actions: NotificationTaskActions
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
  audienceType?: string
  keyword?: string
  from?: string
  to?: string
}

const base = '/api/admin/v1/message/notificationtask'

export async function listNotificationTasks(
  params: NotificationTaskListQuery,
): Promise<NotificationTaskPage> {
  return request.get<NotificationTaskPage>(base, { params })
}

export async function getNotificationTask(id: number): Promise<NotificationTask> {
  return request.get<NotificationTask>(`${base}/${id}`)
}

export async function getNotificationTaskForUpdate(id: number): Promise<NotificationTask> {
  return request.get<NotificationTask>(`${base}/${id}/edit`)
}

export async function createNotificationTask(
  data: NotificationTaskInput,
): Promise<NotificationTask> {
  return request.post<NotificationTask>(base, data)
}

export async function updateNotificationTask(
  id: number,
  data: NotificationTaskInput,
): Promise<NotificationTask> {
  return request.put<NotificationTask>(`${base}/${id}`, data)
}

export async function deleteNotificationTask(id: number): Promise<void> {
  return request.delete<void>(`${base}/${id}`)
}

export async function commandNotificationTask(
  id: number,
  command: 'submit' | 'cancel' | 'copy',
): Promise<NotificationTask> {
  return request.post<NotificationTask>(`${base}/${id}/${command}`, undefined)
}

export async function listNotificationTaskOptions(
  intent: 'create' | 'update',
  kind: 'platform' | 'user' | 'role',
  params: { platformId?: number; keyword?: string; afterId?: number; limit?: number },
): Promise<NotificationTaskOptions> {
  return request.get<NotificationTaskOptions>(`${base}/${intent}/option/${kind}`, { params })
}
