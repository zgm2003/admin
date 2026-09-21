import { request } from '@/utils/request'
import {
  expectArray,
  expectExactKeys,
  expectInteger,
  expectNullableString,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'

export type CacheGenerationStatus =
  'ready' | 'pending' | 'retrying' | 'invalidating' | 'missing' | 'corrupt' | 'unavailable'

export type CacheGenerationPublishState = 'ready' | 'pending' | 'retrying'

export interface CacheGeneration {
  namespace: string
  scopeKey: string
  generation: number
  status: CacheGenerationStatus
  pendingCount: number
  oldestPendingAt: string | null
  latestAttempts: number
  lastError: string
  latestPublishedGeneration: number | null
  latestPublishedAt: string | null
  updatedAt: string
}

export interface CacheGenerationPage {
  list: CacheGeneration[]
  total: number
  page: number
  pageSize: number
}

const generationStatuses: readonly CacheGenerationStatus[] = [
  'ready',
  'pending',
  'retrying',
  'invalidating',
  'missing',
  'corrupt',
  'unavailable',
]

const timestampPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/

function parseStatus(value: unknown, context: string): CacheGenerationStatus {
  if (typeof value !== 'string' || !generationStatuses.includes(value as CacheGenerationStatus)) {
    throw new ProtocolError(`${context} is invalid`)
  }
  return value as CacheGenerationStatus
}

function parseTimestamp(value: unknown, context: string): string {
  const text = expectString(value, context)
  if (!timestampPattern.test(text)) {
    throw new ProtocolError(`${context} must be a UTC RFC3339 timestamp`)
  }
  return text
}

function parseNullableTimestamp(value: unknown, context: string): string | null {
  const text = expectNullableString(value, context)
  return text === null ? null : parseTimestamp(text, context)
}

function parseNullableInteger(value: unknown, context: string): number | null {
  if (value === null) return null
  return expectInteger(value, context)
}

function parseCacheGeneration(value: unknown, context: string): CacheGeneration {
  const record = expectExactKeys(
    value,
    [
      'namespace',
      'scopeKey',
      'generation',
      'status',
      'pendingCount',
      'oldestPendingAt',
      'latestAttempts',
      'lastError',
      'latestPublishedGeneration',
      'latestPublishedAt',
      'updatedAt',
    ],
    context,
  )
  const generation = expectInteger(record.generation, `${context}.generation`)
  const pendingCount = expectInteger(record.pendingCount, `${context}.pendingCount`)
  const latestAttempts = expectInteger(record.latestAttempts, `${context}.latestAttempts`)
  if (generation < 1 || pendingCount < 0 || latestAttempts < 0) {
    throw new ProtocolError(`${context} has invalid counters`)
  }
  const latestPublishedGeneration = parseNullableInteger(
    record.latestPublishedGeneration,
    `${context}.latestPublishedGeneration`,
  )
  if (latestPublishedGeneration !== null && latestPublishedGeneration < 1) {
    throw new ProtocolError(`${context}.latestPublishedGeneration is invalid`)
  }
  return {
    namespace: expectString(record.namespace, `${context}.namespace`),
    scopeKey: expectString(record.scopeKey, `${context}.scopeKey`),
    generation,
    status: parseStatus(record.status, `${context}.status`),
    pendingCount,
    oldestPendingAt: parseNullableTimestamp(record.oldestPendingAt, `${context}.oldestPendingAt`),
    latestAttempts,
    lastError: expectString(record.lastError, `${context}.lastError`),
    latestPublishedGeneration,
    latestPublishedAt: parseNullableTimestamp(
      record.latestPublishedAt,
      `${context}.latestPublishedAt`,
    ),
    updatedAt: parseTimestamp(record.updatedAt, `${context}.updatedAt`),
  }
}

export async function getCacheGenerations(params: {
  page: number
  pageSize: number
  keyword?: string
  publishState?: CacheGenerationPublishState
}): Promise<CacheGenerationPage> {
  const value = await request({
    method: 'GET',
    url: '/api/admin/v1/system/cachegeneration',
    params,
  })
  const record = expectExactKeys(value, ['list', 'total', 'page', 'pageSize'], 'cache generations')
  return {
    list: expectArray(record.list, 'cache generations.list').map((item, index) =>
      parseCacheGeneration(item, `cache generations.list[${index}]`),
    ),
    total: expectInteger(record.total, 'cache generations.total'),
    page: expectInteger(record.page, 'cache generations.page'),
    pageSize: expectInteger(record.pageSize, 'cache generations.pageSize'),
  }
}
