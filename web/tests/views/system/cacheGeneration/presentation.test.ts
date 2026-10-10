import { describe, expect, it } from 'vitest'

import {
  displayNamespace,
  displayScope,
  displayStatus,
} from '@/views/system/cacheGeneration/presentation'

describe('cache generation presentation', () => {
  it('maps fixed namespaces and all statuses to translation keys', () => {
    expect(displayNamespace('system.setting')).toBe('cacheGeneration.namespaceSystemSetting')
    expect(displayNamespace('unknown')).toBe('cacheGeneration.namespaceUnknown')
    expect(displayStatus('missing')).toBe('cacheGeneration.statusMissing')
    expect(displayStatus('unavailable')).toBe('cacheGeneration.statusUnavailable')
  })

  it('distinguishes global, platform and unknown scopes without changing raw values', () => {
    expect(displayScope('system.setting', 'global')).toEqual({
      kind: 'global',
      labelKey: 'cacheGeneration.scopeGlobal',
    })
    expect(displayScope('storage.cosconfig', '2')).toEqual({
      kind: 'storage',
      labelKey: 'cacheGeneration.scopeStorageConfig',
      value: '2',
    })
    expect(displayScope('message.mail', 'custom')).toEqual({
      kind: 'unknown',
      labelKey: 'cacheGeneration.scopeUnknown',
      value: 'custom',
    })
  })
})
