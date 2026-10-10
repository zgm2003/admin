import { request } from '@/utils/request'

export interface SessionOptions {
  statuses: Array<{ value: string; label: string }>
}

export function getSessionOptions(): Promise<SessionOptions> {
  return request.get<SessionOptions>('/api/admin/v1/user/session/options')
}
