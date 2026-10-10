import { request } from '@/utils/request'

export interface NotificationOptions {
  variants: Array<{ value: string; label: string }>
  priorities: Array<{ value: string; label: string }>
}

export function getNotificationOptions(): Promise<NotificationOptions> {
  return request.get<NotificationOptions>('/api/v1/message/notification/options')
}
