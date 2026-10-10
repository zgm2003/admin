import { request } from '@/utils/request'

export type NotificationVariant = string

export type NotificationPriority = string

export type NotificationLinkType = string

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

export async function getNotificationSummary(): Promise<NotificationSummary> {
  return request.get<NotificationSummary>('/api/v1/message/notification/summary')
}

export async function listNotifications(params: NotificationQuery): Promise<NotificationList> {
  return request.get<NotificationList>('/api/v1/message/notification', { params })
}

export async function readNotification(id: number): Promise<void> {
  return request.patch<void>(`/api/v1/message/notification/${id}/read`, undefined)
}

export async function readAllNotifications(): Promise<void> {
  return request.patch<void>('/api/v1/message/notification/read-all', undefined)
}

export async function deleteNotification(id: number): Promise<void> {
  return request.delete<void>(`/api/v1/message/notification/${id}`)
}
