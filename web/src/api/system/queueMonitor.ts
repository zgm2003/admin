import { request } from '@/utils/request'

export const QUEUE_MONITOR_UI_URL = '/api/admin/v1/system/queuemonitor/ui/'

export interface QueueMonitorGrantResponse {
  expiresAt: string
}

export async function grantQueueMonitor(): Promise<QueueMonitorGrantResponse> {
  return request.post<QueueMonitorGrantResponse>(
    '/api/admin/v1/system/queuemonitor/grant',
    undefined,
  )
}
