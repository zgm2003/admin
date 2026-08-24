import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  defaultUIPreferences,
  parseStoredUIPreferences,
  readUIPreferences,
  uiPreferencesStorageKey,
  UIPreferencesError,
  writeUIPreferences,
} from './ui-preferences'

describe('ui preferences storage contract', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('uses explicit defaults only when storage is absent', () => {
    expect(readUIPreferences()).toEqual(defaultUIPreferences)
  })

  it('parses a valid versioned record', () => {
    expect(parseStoredUIPreferences({ version: 1, preferences: defaultUIPreferences })).toEqual(defaultUIPreferences)
  })

  it('rejects unknown and missing persisted fields', () => {
    const valid = { version: 1, preferences: defaultUIPreferences }
    expect(() => parseStoredUIPreferences({ ...valid, extra: true })).toThrow(UIPreferencesError)
    const { showFooter: _removed, ...incomplete } = defaultUIPreferences
    expect(() => parseStoredUIPreferences({ version: 1, preferences: incomplete })).toThrow(UIPreferencesError)
  })

  it('rejects invalid version, enums, and colors', () => {
    expect(() => parseStoredUIPreferences({ version: 2, preferences: defaultUIPreferences })).toThrow(UIPreferencesError)
    expect(() => parseStoredUIPreferences({
      version: 1,
      preferences: { ...defaultUIPreferences, theme: 'system' },
    })).toThrow(UIPreferencesError)
    expect(() => parseStoredUIPreferences({
      version: 1,
      preferences: { ...defaultUIPreferences, primaryColor: 'blue' },
    })).toThrow(UIPreferencesError)
    expect(() => parseStoredUIPreferences({
      version: 1,
      preferences: { ...defaultUIPreferences, transitionName: 'none' },
    })).toThrow(UIPreferencesError)
  })

  it('rejects malformed JSON instead of returning defaults', () => {
    localStorage.setItem(uiPreferencesStorageKey, '{broken')
    expect(() => readUIPreferences()).toThrow(UIPreferencesError)
  })

  it('round trips one versioned record', () => {
    writeUIPreferences({ ...defaultUIPreferences, theme: 'dark', primaryColor: '#059669' })
    expect(JSON.parse(localStorage.getItem(uiPreferencesStorageKey) ?? '')).toMatchObject({ version: 1 })
    expect(readUIPreferences()).toEqual({ ...defaultUIPreferences, theme: 'dark', primaryColor: '#059669' })
  })

  it('surfaces storage write failures', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('quota')
    })
    expect(() => writeUIPreferences({ ...defaultUIPreferences })).toThrow(UIPreferencesError)
  })
})
