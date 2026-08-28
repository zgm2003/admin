import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  defaultUIPreferences,
  uiPreferencesStorageKey,
  writeUIPreferences,
} from '@src/utils/ui-preferences'
import { useUIPreferencesStore } from '@src/store/ui-preferences'

describe('ui preferences store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.style.cssText = ''
  })

  it('applies valid persisted preferences during initialization', () => {
    localStorage.setItem(uiPreferencesStorageKey, JSON.stringify({
      version: 2,
      preferences: { ...persistedDefaults(), primaryColor: '#059669' },
    }))
    const store = useUIPreferencesStore()

    store.initialize()

    expect(store.preferences.theme).toBe('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.style.getPropertyValue('--el-color-primary')).toBe('#059669')
    expect(store.initialized).toBe(true)
    expect(store.persistenceError).toBeNull()
  })

  it('keeps defaults and records an explicit error when initialization fails', () => {
    localStorage.setItem(uiPreferencesStorageKey, '{broken')
    const store = useUIPreferencesStore()

    store.initializeSafely()

    expect(store.preferences).toEqual(defaultUIPreferences)
    expect(store.persistenceError).toBe('invalid')
    expect(store.initialized).toBe(true)
  })

  it('does not mutate persistent preferences when persistence fails', () => {
    const store = useUIPreferencesStore()
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('quota')
    })

    store.update({ primaryColor: '#059669' })

    expect(store.preferences.primaryColor).toBe(defaultUIPreferences.primaryColor)
    expect(store.persistenceError).toBe('write')
    setItem.mockRestore()
  })

  it('updates theme only for the active session without writing storage', () => {
    writeUIPreferences({ ...defaultUIPreferences, primaryColor: '#059669' })
    const store = useUIPreferencesStore()
    store.initialize()
    const before = localStorage.getItem(uiPreferencesStorageKey)
    const setItem = vi.spyOn(Storage.prototype, 'setItem')

    store.update({ theme: 'dark' })

    expect(store.preferences.theme).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(setItem).not.toHaveBeenCalled()
    expect(localStorage.getItem(uiPreferencesStorageKey)).toBe(before)
    setItem.mockRestore()
  })

  it('persists non-theme updates and reset restores every runtime default', () => {
    const store = useUIPreferencesStore()

    store.update({ theme: 'dark', showFooter: false })
    expect(store.preferences.theme).toBe('dark')
    expect(store.preferences.showFooter).toBe(false)
    expect(JSON.parse(localStorage.getItem(uiPreferencesStorageKey) ?? '')).toEqual({
      version: 2,
      preferences: { ...persistedDefaults(), showFooter: false },
    })

    store.reset()
    expect(store.preferences).toEqual(defaultUIPreferences)
    expect(store.persistenceError).toBeNull()
  })
})

function persistedDefaults(): Omit<typeof defaultUIPreferences, 'theme'> {
  const { theme: _theme, ...preferences } = defaultUIPreferences
  return preferences
}
