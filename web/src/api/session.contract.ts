import { ProtocolError } from '../types/http'

export type SessionStatus = 'active' | 'expired' | 'revoked'
export interface SessionListQuery { page: number; pageSize: number; username?: string; platform?: string; status?: SessionStatus }
export interface SessionItem {
  id: number; userId: number; username: string; platform: string; deviceId: string; clientIp: string; userAgent: string
  createdAt: string; updatedAt: string; refreshExpiresAt: string; revokedAt: string | null; status: SessionStatus; isCurrent: boolean
}
export interface SessionPage { list: SessionItem[]; total: number; page: number; pageSize: number }
export interface SessionStats { activeTotal: number; platforms: Record<string, number> }
export interface SessionRevokeResult { revoked: number; skippedCurrent: number; skippedRevoked: number }

const timestamp = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/
function record(value: unknown, keys: readonly string[], label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ProtocolError(label + ' must be an object')
  const actual = Object.keys(value).sort(); const expected = [...keys].sort()
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) throw new ProtocolError('record contains unexpected or missing fields')
  return value as Record<string, unknown>
}
function positive(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isInteger(value) || value <= 0) throw new ProtocolError(label + ' must be positive')
  return value
}
function nonNegative(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isInteger(value) || value < 0) throw new ProtocolError(label + ' must be non-negative')
  return value
}
function text(value: unknown, label: string): string {
  if (typeof value !== 'string') throw new ProtocolError(label + ' must be a string')
  return value
}
function time(value: unknown, label: string): string {
  const result = text(value, label); if (!timestamp.test(result)) throw new ProtocolError(label + ' must be an ISO timestamp'); return result
}
export function parseSessionPage(value: unknown): SessionPage {
  const raw = record(value, ['list', 'page', 'pageSize', 'total'], 'session page')
  if (!Array.isArray(raw.list)) throw new ProtocolError('session page list must be an array')
  const list = raw.list.map((item) => {
    const row = record(item, ['clientIp', 'createdAt', 'deviceId', 'id', 'isCurrent', 'platform', 'refreshExpiresAt', 'revokedAt', 'status', 'updatedAt', 'userAgent', 'userId', 'username'], 'session row')
    if (row.status !== 'active' && row.status !== 'expired' && row.status !== 'revoked') throw new ProtocolError('invalid session status')
		if (typeof row.isCurrent !== 'boolean') throw new ProtocolError('session isCurrent must be a boolean')
    if (row.revokedAt !== null) time(row.revokedAt, 'revokedAt')
    return { id: positive(row.id, 'session id'), userId: positive(row.userId, 'user id'), username: text(row.username, 'username'), platform: text(row.platform, 'platform'), deviceId: text(row.deviceId, 'deviceId'), clientIp: text(row.clientIp, 'clientIp'), userAgent: text(row.userAgent, 'userAgent'), createdAt: time(row.createdAt, 'createdAt'), updatedAt: time(row.updatedAt, 'updatedAt'), refreshExpiresAt: time(row.refreshExpiresAt, 'refreshExpiresAt'), revokedAt: row.revokedAt as string | null, status: row.status as SessionStatus, isCurrent: row.isCurrent }
  })
  return { list, total: nonNegative(raw.total, 'total'), page: positive(raw.page, 'page'), pageSize: positive(raw.pageSize, 'pageSize') }
}
export function parseSessionStats(value: unknown): SessionStats {
  const raw = record(value, ['activeTotal', 'platforms'], 'session stats')
  if (typeof raw.platforms !== 'object' || raw.platforms === null || Array.isArray(raw.platforms)) throw new ProtocolError('platforms must be an object')
  const platforms: Record<string, number> = {}
  for (const [key, count] of Object.entries(raw.platforms)) platforms[key] = nonNegative(count, 'platform ' + key)
  return { activeTotal: nonNegative(raw.activeTotal, 'activeTotal'), platforms }
}
export function parseSessionRevokeResult(value: unknown): SessionRevokeResult {
  const raw = record(value, ['revoked', 'skippedCurrent', 'skippedRevoked'], 'session revoke result')
  return { revoked: nonNegative(raw.revoked, 'revoked'), skippedCurrent: nonNegative(raw.skippedCurrent, 'skippedCurrent'), skippedRevoked: nonNegative(raw.skippedRevoked, 'skippedRevoked') }
}
