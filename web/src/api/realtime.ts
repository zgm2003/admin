import { expectExactKeys, expectISODate, expectString } from '@/api/protocol'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

export interface RealtimeTicket {
  ticket: string
  expiresAt: string
}

export function parseRealtimeTicket(value: unknown): RealtimeTicket {
  const record = expectExactKeys(value, ['ticket', 'expiresAt'], 'realtime ticket')
  const ticket = expectString(record.ticket, 'ticket')
  if (ticket === '') throw new ProtocolError('ticket is required')
  return { ticket, expiresAt: expectISODate(record.expiresAt, 'expiresAt') }
}

export async function requestRealtimeTicket(): Promise<RealtimeTicket> {
  return parseRealtimeTicket(
    await request<unknown>({ method: 'POST', url: '/api/v1/realtime/ticket' }),
  )
}

export function buildRealtimeWebSocketURL(
  ticket: string,
  location: Pick<Location, 'protocol' | 'host'> | URL = window.location,
): URL {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = new URL(`${protocol}//${location.host}/api/v1/realtime/ws`)
  url.searchParams.set('ticket', ticket)
  return url
}
