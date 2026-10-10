import type { CacheGenerationStatus } from '@/api/system/cacheGeneration'

const namespaceLabels: Readonly<Record<string, string>> = {
  'system.setting': 'cacheGeneration.namespaceSystemSetting',
  'system.dictionary': 'cacheGeneration.namespaceSystemDictionary',
  'message.mail': 'cacheGeneration.namespaceMessageMail',
  'message.sms': 'cacheGeneration.namespaceMessageSms',
  'storage.cosconfig': 'cacheGeneration.namespaceStorageCosConfig',
}

const statusLabels: Readonly<Record<CacheGenerationStatus, string>> = {
  ready: 'cacheGeneration.statusReady',
  pending: 'cacheGeneration.statusPending',
  retrying: 'cacheGeneration.statusRetrying',
  invalidating: 'cacheGeneration.statusInvalidating',
  missing: 'cacheGeneration.statusMissing',
  corrupt: 'cacheGeneration.statusCorrupt',
  unavailable: 'cacheGeneration.statusUnavailable',
}

export function displayNamespace(namespace: string): string {
  return namespaceLabels[namespace] ?? 'cacheGeneration.namespaceUnknown'
}

export type ScopeDisplay =
  | { kind: 'global'; labelKey: 'cacheGeneration.scopeGlobal' }
  | { kind: 'storage'; labelKey: 'cacheGeneration.scopeStorageConfig'; value: string }
  | { kind: 'unknown'; labelKey: 'cacheGeneration.scopeUnknown'; value: string }

export function displayScope(namespace: string, scopeKey: string): ScopeDisplay {
  if (scopeKey === 'global') return { kind: 'global', labelKey: 'cacheGeneration.scopeGlobal' }
  if (namespace === 'storage.cosconfig' && /^\d+$/.test(scopeKey)) {
    return { kind: 'storage', labelKey: 'cacheGeneration.scopeStorageConfig', value: scopeKey }
  }
  return { kind: 'unknown', labelKey: 'cacheGeneration.scopeUnknown', value: scopeKey }
}

export function displayStatus(status: CacheGenerationStatus): string {
  return statusLabels[status]
}
