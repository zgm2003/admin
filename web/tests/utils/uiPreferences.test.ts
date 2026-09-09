import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  defaultUIPreferences,
  parseStoredUIPreferences,
  readUIPreferences,
  uiPreferencesStorageKey,
  UIPreferencesError,
  writeUIPreferences,
} from '@/utils/uiPreferences'

describe('ui preferences storage contract', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('uses explicit defaults only when storage is absent', () => {
    expect(readUIPreferences()).toEqual(defaultUIPreferences)
  })

  it('parses a valid v3 record with the light runtime theme', () => {
    expect(parseStoredUIPreferences({ version: 3, preferences: persistedDefaults() })).toEqual(
      defaultUIPreferences,
    )
  })

  it('rejects unknown and missing v3 persisted fields', () => {
    const valid = { version: 3, preferences: persistedDefaults() }
    expect(() => parseStoredUIPreferences({ ...valid, extra: true })).toThrow(UIPreferencesError)
    const { showFooter: _removed, ...incomplete } = persistedDefaults()
    expect(() => parseStoredUIPreferences({ version: 3, preferences: incomplete })).toThrow(
      UIPreferencesError,
    )
  })

  it('rejects theme, invalid versions, enums, and colors in stored records', () => {
    expect(() =>
      parseStoredUIPreferences({
        version: 3,
        preferences: { ...persistedDefaults(), theme: 'dark' },
      }),
    ).toThrow(UIPreferencesError)
    expect(() =>
      parseStoredUIPreferences({ version: 1, preferences: defaultUIPreferences }),
    ).toThrow(UIPreferencesError)
    expect(() =>
      parseStoredUIPreferences({
        version: 4,
        preferences: persistedDefaults(),
      }),
    ).toThrow(UIPreferencesError)
    expect(() =>
      parseStoredUIPreferences({
        version: 3,
        preferences: { ...persistedDefaults(), primaryColor: 'blue' },
      }),
    ).toThrow(UIPreferencesError)
    expect(() =>
      parseStoredUIPreferences({
        version: 3,
        preferences: { ...persistedDefaults(), transitionName: 'none' },
      }),
    ).toThrow(UIPreferencesError)
    expect(() =>
      parseStoredUIPreferences({
        version: 3,
        preferences: { ...persistedDefaults(), layout: 'grid' },
      }),
    ).toThrow(UIPreferencesError)
  })

  it('rejects malformed JSON instead of returning defaults', () => {
    localStorage.setItem(uiPreferencesStorageKey, '{broken')
    expect(() => readUIPreferences()).toThrow(UIPreferencesError)
  })

  it('writes v3 without the runtime-only theme', () => {
    writeUIPreferences({ ...defaultUIPreferences, theme: 'dark', primaryColor: '#059669' })
    const stored = JSON.parse(localStorage.getItem(uiPreferencesStorageKey) ?? '') as {
      version: number
      preferences: Record<string, unknown>
    }

    expect(stored.version).toBe(3)
    expect(stored.preferences.theme).toBeUndefined()
    expect(stored.preferences.primaryColor).toBe('#059669')
  })

  it('migrates a valid v2 record once and injects the default layout', () => {
    const { layout: _layout, theme: _theme, ...v2Persisted } = defaultUIPreferences
    localStorage.setItem(
      uiPreferencesStorageKey,
      JSON.stringify({ version: 2, preferences: { ...v2Persisted, showFooter: false } }),
    )

    expect(readUIPreferences()).toEqual({
      ...defaultUIPreferences,
      theme: 'light',
      showFooter: false,
    })

    const stored = JSON.parse(localStorage.getItem(uiPreferencesStorageKey) ?? '') as {
      version: number
      preferences: Record<string, unknown>
    }
    expect(stored.version).toBe(3)
    expect(stored.preferences.layout).toBe('side')
  })

  it('migrates a valid v1 record once and uses light for the cold start theme', () => {
    const { layout: _layout, ...v1Preferences } = defaultUIPreferences
    localStorage.setItem(
      uiPreferencesStorageKey,
      JSON.stringify({
        version: 1,
        preferences: { ...v1Preferences, theme: 'dark', showFooter: false },
      }),
    )

    expect(readUIPreferences()).toEqual({
      ...defaultUIPreferences,
      theme: 'light',
      showFooter: false,
    })

    const stored = JSON.parse(localStorage.getItem(uiPreferencesStorageKey) ?? '') as {
      version: number
      preferences: Record<string, unknown>
    }
    expect(stored.version).toBe(3)
    expect(stored.preferences.theme).toBeUndefined()
  })

  it('surfaces storage write failures', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('quota')
    })
    expect(() => writeUIPreferences({ ...defaultUIPreferences })).toThrow(UIPreferencesError)
  })
})

function persistedDefaults(): Omit<typeof defaultUIPreferences, 'theme'> {
  const { theme: _theme, ...preferences } = defaultUIPreferences
  return preferences
}
