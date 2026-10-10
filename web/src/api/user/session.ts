import { request } from '@/utils/request'

export type SessionStatus = string

export interface SessionListQuery {
  page: number
  pageSize: number
  username?: string
  platform?: string
  status?: SessionStatus
}

export interface SessionItem {
  actions: { revoke: boolean }
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
  return request.get<SessionPage>('/api/admin/v1/user/session', { params: query })
}

export async function getSessionStats(): Promise<SessionStats> {
  return request.get<SessionStats>('/api/admin/v1/user/session/stats')
}

export async function revokeSession(id: number): Promise<SessionRevokeResult> {
  return request.delete<SessionRevokeResult>('/api/admin/v1/user/session/' + id)
}

export async function revokeSessions(ids: number[]): Promise<SessionRevokeResult> {
  return request.delete<SessionRevokeResult, { ids: number[] }>('/api/admin/v1/user/session', {
    data: { ids },
  })
}
