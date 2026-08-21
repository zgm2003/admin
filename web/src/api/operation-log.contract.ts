import { ProtocolError } from '../types/http'
import { isYesNo, type YesNo } from '../enums/yes-no'

export interface OperationLogListQuery { page: number; pageSize: number; userId?: number; action?: string; route?: string; isSuccess?: YesNo; from?: string; to?: string }
export interface OperationLogItem {
  id: number; requestId: string; userId: number | null; sessionId: number | null; platform: string
  method: string; route: string; module: string; action: string; clientIp: string; userAgent: string
  statusCode: number; isSuccess: YesNo; latencyMs: number; requestData: unknown; responseData: unknown; createdAt: string; updatedAt: string
}
export interface OperationLogPage { list: OperationLogItem[]; total: number; page: number; pageSize: number }

function record(value: unknown, keys: readonly string[], label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ProtocolError(label + ' must be an object')
  const actual = Object.keys(value).sort(); const expected = [...keys].sort()
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) throw new ProtocolError(label + ' contains unexpected or missing fields')
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
export function parseOperationLogPage(value: unknown): OperationLogPage {
  const raw = record(value, ['list', 'page', 'pageSize', 'total'], 'operation log page')
  if (!Array.isArray(raw.list)) throw new ProtocolError('operation log list must be an array')
  const list = raw.list.map((item) => {
    const row = record(item, ['action', 'clientIp', 'createdAt', 'id', 'isSuccess', 'latencyMs', 'method', 'module', 'platform', 'requestData', 'requestId', 'responseData', 'route', 'sessionId', 'statusCode', 'updatedAt', 'userAgent', 'userId'], 'operation log row')
    if (!isYesNo(row.isSuccess)) throw new ProtocolError('operation log isSuccess must be 0 or 1')
    if (row.userId !== null) positive(row.userId, 'userId')
    if (row.sessionId !== null) positive(row.sessionId, 'sessionId')
    return { id: positive(row.id, 'id'), requestId: text(row.requestId, 'requestId'), userId: row.userId as number | null, sessionId: row.sessionId as number | null, platform: text(row.platform, 'platform'), method: text(row.method, 'method'), route: text(row.route, 'route'), module: text(row.module, 'module'), action: text(row.action, 'action'), clientIp: text(row.clientIp, 'clientIp'), userAgent: text(row.userAgent, 'userAgent'), statusCode: positive(row.statusCode, 'statusCode'), isSuccess: row.isSuccess, latencyMs: nonNegative(row.latencyMs, 'latencyMs'), requestData: row.requestData, responseData: row.responseData, createdAt: text(row.createdAt, 'createdAt'), updatedAt: text(row.updatedAt, 'updatedAt') }
  })
  return { list, total: nonNegative(raw.total, 'total'), page: positive(raw.page, 'page'), pageSize: positive(raw.pageSize, 'pageSize') }
}
