import { request } from '@/utils/request'

export interface RealtimeTicket {
  ticket: string
  expiresAt: string
}

export async function requestRealtimeTicket(): Promise<RealtimeTicket> {
  return request.post<RealtimeTicket>('/api/v1/realtime/ticket', undefined)
}
