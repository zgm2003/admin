import { request } from '../utils/request'
export type SessionStatus = 'active' | 'expired' | 'revoked'
export interface SessionListQuery { page: number; pageSize: number; username?: string; platform?: string; status?: SessionStatus }
export interface SessionItem { id: number; userId: number; username: string; platform: string; deviceId: string; clientIp: string; userAgent: string; createdAt: string; updatedAt: string; refreshExpiresAt: string; revokedAt: string | null; status: SessionStatus; isCurrent: boolean }
export interface SessionPage { list: SessionItem[]; total: number; page: number; pageSize: number }
export interface SessionStats { activeTotal: number; platforms: Record<string, number> }
export interface SessionRevokeResult { revoked: number; skippedCurrent: number; skippedRevoked: number }

export async function getSessions(query: SessionListQuery): Promise<SessionPage> {
  return request<SessionPage>({ method: 'GET', url: '/api/v1/sessions', params: query })
}
export async function getSessionStats(): Promise<SessionStats> {
  return request<SessionStats>({ method: 'GET', url: '/api/v1/sessions/stats' })
}
export async function revokeSession(id: number): Promise<SessionRevokeResult> {
  return request<SessionRevokeResult>({ method: 'DELETE', url: '/api/v1/sessions/' + id })
}
export async function revokeSessions(ids: number[]): Promise<SessionRevokeResult> {
  return request<SessionRevokeResult>({ method: 'DELETE', url: '/api/v1/sessions', data: { ids } })
}
