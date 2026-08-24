import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  defaultUIPreferences,
  uiPreferencesStorageKey,
} from '../utils/ui-preferences'
import { useUIPreferencesStore } from './ui-preferences'

describe('ui preferences store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.style.cssText = ''
  })

  it('applies valid persisted preferences during initialization', () => {
    localStorage.setItem(uiPreferencesStorageKey, JSON.stringify({
      version: 1,
      preferences: { ...defaultUIPreferences, theme: 'dark', primaryColor: '#059669' },
    }))
    const store = useUIPreferencesStore()

    store.initialize()

    expect(store.preferences.theme).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
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

  it('does not mutate live preferences when persistence fails', () => {
    const store = useUIPreferencesStore()
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('quota')
    })

    store.update({ theme: 'dark' })

    expect(store.preferences.theme).toBe('light')
    expect(store.persistenceError).toBe('write')
    setItem.mockRestore()
  })

  it('persists updates and reset restores every default preference', () => {
    const store = useUIPreferencesStore()

    store.update({ theme: 'dark', showFooter: false })
    expect(store.preferences.theme).toBe('dark')
    expect(store.preferences.showFooter).toBe(false)

    store.reset()
    expect(store.preferences).toEqual(defaultUIPreferences)
    expect(store.persistenceError).toBeNull()
  })
})
