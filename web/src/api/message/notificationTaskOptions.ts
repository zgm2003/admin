import { request } from '@/utils/request'
import type { NotificationAudience } from '@/api/message/notificationTask'

export interface NotificationTaskAdminOptions {
  statuses: Array<{ value: number; label: string }>
  audiences: Array<{ value: NotificationAudience; label: string }>
  variants: Array<{ value: string; label: string }>
  priorities: Array<{ value: string; label: string }>
  linkTypes: Array<{ value: string; label: string }>
  defaults: {
    variant: string
    priority: string
    linkType: string
    audienceType: NotificationAudience
  }
  constraints: { titleMaxLength: number }
}

export function getNotificationTaskOptions(): Promise<NotificationTaskAdminOptions> {
  return request.get<NotificationTaskAdminOptions>('/api/admin/v1/message/notificationtask/options')
}
