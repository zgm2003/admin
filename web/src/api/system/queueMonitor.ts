import { expectExactKeys, expectString } from '@/api/protocol'
import { request } from '@/utils/request'
import { ProtocolError } from '@/types/http'

export const QUEUE_MONITOR_UI_URL = '/api/admin/v1/system/queuemonitor/ui/'

export interface QueueMonitorGrantResponse {
  expiresAt: string
}

export async function grantQueueMonitor(): Promise<QueueMonitorGrantResponse> {
  const value = expectExactKeys(
    await request<unknown>({ method: 'POST', url: '/api/admin/v1/system/queuemonitor/grant' }),
    ['expiresAt'],
    'queue monitor grant',
  )
  const expiresAt = expectString(value.expiresAt, 'queue monitor grant.expiresAt')
  if (expiresAt.trim() === '' || Number.isNaN(Date.parse(expiresAt))) {
    throw new ProtocolError('queue monitor grant.expiresAt is invalid')
  }
  return { expiresAt }
}
