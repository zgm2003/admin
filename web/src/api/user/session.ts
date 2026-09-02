import { request } from '@/utils/request'
import {
  expectBoolean,
  expectInteger,
  expectNullableString,
  expectPage,
  expectRecord,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'
export type SessionStatus = 'active' | 'expired' | 'revoked'
export interface SessionListQuery {
  page: number
  pageSize: number
  username?: string
  platform?: string
  status?: SessionStatus
}
export interface SessionItem {
  id: number
  userId: number
  username: string
  platform: string
  deviceId: string
  clientIp: string
  userAgent: string
  createdAt: string
  updatedAt: string
  refreshExpiresAt: string
  revokedAt: string | null
  status: SessionStatus
  isCurrent: boolean
}
export interface SessionPage {
  list: SessionItem[]
  total: number
  page: number
  pageSize: number
}
export interface SessionStats {
  activeTotal: number
  platforms: Record<string, number>
}
export interface SessionRevokeResult {
  revoked: number
  skippedCurrent: number
  skippedRevoked: number
}

export async function getSessions(query: SessionListQuery): Promise<SessionPage> {
  return expectPage(
    await request<unknown>({ method: 'GET', url: '/api/admin/v1/sessions', params: query }),
    parseSessionItem,
    'sessions',
  )
}
export async function getSessionStats(): Promise<SessionStats> {
  const value = expectRecord(
    await request<unknown>({ method: 'GET', url: '/api/admin/v1/sessions/stats' }),
    'session stats',
  )
  const platforms = expectRecord(value.platforms, 'session stats.platforms')
  const normalized: Record<string, number> = {}
  for (const [key, item] of Object.entries(platforms))
    normalized[key] = expectInteger(item, `session stats.platforms.${key}`)
  return {
    activeTotal: expectInteger(value.activeTotal, 'session stats.activeTotal'),
    platforms: normalized,
  }
}
export async function revokeSession(id: number): Promise<SessionRevokeResult> {
  return parseRevokeResult(
    await request<unknown>({ method: 'DELETE', url: '/api/admin/v1/sessions/' + id }),
  )
}
export async function revokeSessions(ids: number[]): Promise<SessionRevokeResult> {
  return parseRevokeResult(
    await request<unknown>({
      method: 'DELETE',
      url: '/api/admin/v1/sessions',
      data: { ids },
    }),
  )
}

function parseSessionItem(value: unknown, index: number): SessionItem {
  const item = expectRecord(value, `sessions.list[${index}]`)
  const status = expectString(item.status, `sessions.list[${index}].status`)
  if (status !== 'active' && status !== 'expired' && status !== 'revoked')
    throw new ProtocolError('session status is invalid')
  return {
    id: expectInteger(item.id, `sessions.list[${index}].id`),
    userId: expectInteger(item.userId, `sessions.list[${index}].userId`),
    username: expectString(item.username, `sessions.list[${index}].username`),
    platform: expectString(item.platform, `sessions.list[${index}].platform`),
    deviceId: expectString(item.deviceId, `sessions.list[${index}].deviceId`),
    clientIp: expectString(item.clientIp, `sessions.list[${index}].clientIp`),
    userAgent: expectString(item.userAgent, `sessions.list[${index}].userAgent`),
    createdAt: expectString(item.createdAt, `sessions.list[${index}].createdAt`),
    updatedAt: expectString(item.updatedAt, `sessions.list[${index}].updatedAt`),
    refreshExpiresAt: expectString(
      item.refreshExpiresAt,
      `sessions.list[${index}].refreshExpiresAt`,
    ),
    revokedAt: expectNullableString(item.revokedAt, `sessions.list[${index}].revokedAt`),
    status,
    isCurrent: expectBoolean(item.isCurrent, `sessions.list[${index}].isCurrent`),
  }
}

function parseRevokeResult(value: unknown): SessionRevokeResult {
  const item = expectRecord(value, 'session revoke result')
  return {
    revoked: expectInteger(item.revoked, 'session revoke result.revoked'),
    skippedCurrent: expectInteger(item.skippedCurrent, 'session revoke result.skippedCurrent'),
    skippedRevoked: expectInteger(item.skippedRevoked, 'session revoke result.skippedRevoked'),
  }
}
