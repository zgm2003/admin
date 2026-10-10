import { request } from '@/utils/request'

export type CacheGenerationStatus = string

export type CacheGenerationPublishState = string

export interface CacheGeneration {
  namespaceLabel: string
  scopeLabel: string
  statusLabel: string
  statusTone: 'success' | 'warning' | 'danger' | 'info'
  statusHint: string
  publishedVersion: number | null
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

export async function getCacheGenerations(params: {
  page: number
  pageSize: number
  keyword?: string
  publishState?: CacheGenerationPublishState
}): Promise<CacheGenerationPage> {
  return request.get<CacheGenerationPage>('/api/admin/v1/system/cachegeneration', { params })
}

export interface CacheGenerationOptions {
  publishStates: Array<{ value: string; label: string }>
}
export function getCacheGenerationOptions(): Promise<CacheGenerationOptions> {
  return request.get<CacheGenerationOptions>('/api/admin/v1/system/cachegeneration/options')
}
