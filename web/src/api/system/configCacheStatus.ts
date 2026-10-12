import { request } from '@/utils/request'

export type ConfigCacheStatusStatus = string

export type ConfigCacheStatusPublishState = string

export interface ConfigCacheStatus {
  namespaceLabel: string
  scopeLabel: string
  scopeName: string
  scopeDeleted: boolean
  scopeMissing: boolean
  statusLabel: string
  statusTone: 'success' | 'warning' | 'danger' | 'info'
  statusHint: string
  publishedVersion: number | null
  namespace: string
  scopeKey: string
  generation: number
  status: ConfigCacheStatusStatus
  pendingCount: number
  oldestPendingAt: string | null
  latestAttempts: number
  lastError: string
  latestPublishedGeneration: number | null
  latestPublishedAt: string | null
  updatedAt: string
}

export interface ConfigCacheStatusPage {
  list: ConfigCacheStatus[]
  total: number
  page: number
  pageSize: number
}

export async function getConfigCacheStatuses(params: {
  page: number
  pageSize: number
  keyword?: string
  publishState?: ConfigCacheStatusPublishState
}): Promise<ConfigCacheStatusPage> {
  return request.get<ConfigCacheStatusPage>('/api/admin/v1/system/config-cache-status', { params })
}

export interface ConfigCacheStatusOptions {
  publishStates: Array<{ value: string; label: string }>
}
export function getConfigCacheStatusOptions(): Promise<ConfigCacheStatusOptions> {
  return request.get<ConfigCacheStatusOptions>('/api/admin/v1/system/config-cache-status/options')
}
