import { isYesNo, type YesNo } from '../enums/yes-no'
import { ProtocolError } from '../types/http'
import type { PageRequest, PageResult } from '../types/pagination'

export interface AuthPlatformListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}

export interface AuthPlatformListItem {
  id: number
  code: string
  name: string
  policyVersion: number
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: YesNo
  bindIP: YesNo
  maxSessions: number
  allowRegister: YesNo
  isEnabled: YesNo
  isBuiltin: YesNo
  createdAt: string
  updatedAt: string
}

export interface AuthPlatformDeployment {
  cookieSecure: boolean
  corsOrigin: string
  trustedProxyMode: 'none' | 'allowlist'
  trustedProxyCount: number
  redisStatus: 'up' | 'down'
}

export interface CreateAuthPlatformInput {
  code: string
  name: string
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: YesNo
  bindIP: YesNo
  maxSessions: number
  allowRegister: YesNo
  isEnabled: YesNo
}

export interface UpdateAuthPlatformInput {
  name: string
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: YesNo
  bindIP: YesNo
  maxSessions: number
  allowRegister: YesNo
}

export interface AuthPlatformStatusResult {
  id: number
  isEnabled: YesNo
}

const platformCodePattern = /^[a-z][a-z0-9_]{1,48}$/
const timestampPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/

export function parseAuthPlatformPage(value: unknown): PageResult<AuthPlatformListItem> {
  const record = closed(value, ['list', 'page', 'pageSize', 'total'], 'authentication platform page')
  if (!Array.isArray(record.list)) {
    throw new ProtocolError('authentication platform page list must be an array')
  }
  const ids = new Set<number>()
  const codes = new Set<string>()
  let previousUpdatedAt = ''
  let previousID = Number.MAX_SAFE_INTEGER
  const list = record.list.map((item, index) => {
    const row = closed(item, [
      'accessCacheTTLSeconds', 'accessTTLSeconds', 'allowRegister', 'bindDevice', 'bindIP',
      'code', 'createdAt', 'id', 'isBuiltin', 'isEnabled', 'maxSessions', 'name',
      'policyVersion', 'refreshTTLSeconds', 'sessionCacheTTLSeconds', 'updatedAt',
    ], `authentication platform row ${index}`)
    const id = positive(row.id, 'platform id')
    const code = text(row.code, 'platform code')
    if (!platformCodePattern.test(code) || code !== code.toLowerCase()) {
      throw new ProtocolError('platform code is invalid')
    }
    if (ids.has(id) || codes.has(code)) {
      throw new ProtocolError('authentication platform page contains duplicate id or code')
    }
    const updatedAt = timestamp(row.updatedAt, 'updatedAt')
    if (previousUpdatedAt !== '' && (updatedAt > previousUpdatedAt || (updatedAt === previousUpdatedAt && id >= previousID))) {
      throw new ProtocolError('authentication platform page order is unstable')
    }
    previousUpdatedAt = updatedAt
    previousID = id
    ids.add(id)
    codes.add(code)
    if (!isYesNo(row.bindDevice) || !isYesNo(row.bindIP) || !isYesNo(row.allowRegister) || !isYesNo(row.isEnabled) || !isYesNo(row.isBuiltin)) {
      throw new ProtocolError('authentication platform Yes/No value is invalid')
    }
    return {
      id,
      code,
      name: boundedText(row.name, 'platform name', 64),
      policyVersion: positive(row.policyVersion, 'policyVersion'),
      accessTTLSeconds: range(row.accessTTLSeconds, 60, 2_592_000, 'accessTTLSeconds'),
      refreshTTLSeconds: range(row.refreshTTLSeconds, 60, 31_536_000, 'refreshTTLSeconds'),
      sessionCacheTTLSeconds: range(row.sessionCacheTTLSeconds, 60, 86_400, 'sessionCacheTTLSeconds'),
      accessCacheTTLSeconds: range(row.accessCacheTTLSeconds, 60, 86_400, 'accessCacheTTLSeconds'),
      bindDevice: row.bindDevice,
      bindIP: row.bindIP,
      maxSessions: range(row.maxSessions, 0, 100, 'maxSessions'),
      allowRegister: row.allowRegister,
      isEnabled: row.isEnabled,
      isBuiltin: row.isBuiltin,
      createdAt: timestamp(row.createdAt, 'createdAt'),
      updatedAt,
    }
  })

  const total = nonNegative(record.total, 'total')
  const page = positive(record.page, 'page')
  const pageSize = range(record.pageSize, 1, 100, 'pageSize')
  if (list.length > pageSize || total < list.length || (total === 0 && page !== 1) || (total > 0 && page > Math.ceil(total / pageSize))) {
    throw new ProtocolError('authentication platform page totals are invalid')
  }
  return { list, total, page, pageSize }
}

export function parseAuthPlatformDeployment(value: unknown): AuthPlatformDeployment {
  const record = closed(value, ['cookieSecure', 'corsOrigin', 'redisStatus', 'trustedProxyCount', 'trustedProxyMode'], 'authentication platform deployment')
  if (typeof record.cookieSecure !== 'boolean' || typeof record.corsOrigin !== 'string' || record.corsOrigin.trim() === '') {
    throw new ProtocolError('deployment configuration is invalid')
  }
  if (record.trustedProxyMode !== 'none' && record.trustedProxyMode !== 'allowlist') {
    throw new ProtocolError('trusted proxy mode is invalid')
  }
  if (typeof record.redisStatus !== 'string' || (record.redisStatus !== 'up' && record.redisStatus !== 'down')) {
    throw new ProtocolError('redis status is invalid')
  }
  return {
    cookieSecure: record.cookieSecure,
    corsOrigin: record.corsOrigin,
    trustedProxyMode: record.trustedProxyMode,
    trustedProxyCount: nonNegative(record.trustedProxyCount, 'trustedProxyCount'),
    redisStatus: record.redisStatus,
  }
}

export function parseAuthPlatformIDResult(value: unknown): { id: number } {
  const record = closed(value, ['id'], 'authentication platform id result')
  return { id: positive(record.id, 'id') }
}

export function parseAuthPlatformStatusResult(value: unknown): AuthPlatformStatusResult {
  const record = closed(value, ['id', 'isEnabled'], 'authentication platform status')
  if (!isYesNo(record.isEnabled)) throw new ProtocolError('authentication platform status is invalid')
  return { id: positive(record.id, 'id'), isEnabled: record.isEnabled }
}

export function parseEmptyAuthPlatformResult(value: unknown): Record<string, never> {
  closed(value, [], 'authentication platform empty result')
  return {}
}

function closed(value: unknown, keys: readonly string[], label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ProtocolError(`${label} must be an object`)
  const record = value as Record<string, unknown>
  const actual = Object.keys(record).sort()
  const expected = [...keys].sort()
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) throw new ProtocolError(`${label} has missing or extra fields`)
  return record
}

function positive(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) throw new ProtocolError(`${label} must be positive integer`)
  return value
}

function nonNegative(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new ProtocolError(`${label} must be non-negative integer`)
  return value
}

function range(value: unknown, minimum: number, maximum: number, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < minimum || value > maximum) throw new ProtocolError(`${label} is out of range`)
  return value
}

function text(value: unknown, label: string): string {
  if (typeof value !== 'string' || value === '' || value.trim() !== value) throw new ProtocolError(`${label} must be non-empty trimmed string`)
  return value
}

function boundedText(value: unknown, label: string, maximumLength: number): string {
  const result = text(value, label)
  if ([...result].length > maximumLength) throw new ProtocolError(`${label} is too long`)
  return result
}

function timestamp(value: unknown, label: string): string {
  const result = text(value, label)
  if (!timestampPattern.test(result) || !Number.isFinite(Date.parse(result))) throw new ProtocolError(`${label} must be RFC3339`)
  return result
}
