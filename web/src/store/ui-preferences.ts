import { defineStore } from 'pinia'
import { ref } from 'vue'

import { applyPrimaryColor, applyTheme } from '@/utils/theme'
import {
  defaultUIPreferences,
  readUIPreferences,
  UIPreferencesError,
  writeUIPreferences,
  type UIPreferences,
} from '@/utils/ui-preferences'

export type UIPersistenceError = 'invalid' | 'write' | null

export const useUIPreferencesStore = defineStore('uiPreferences', () => {
  const preferences = ref<UIPreferences>({ ...defaultUIPreferences })
  const persistenceError = ref<UIPersistenceError>(null)
  const initialized = ref(false)

  function initialize(): void {
    replace(readUIPreferences())
    persistenceError.value = null
    initialized.value = true
  }

  function initializeSafely(): void {
    try {
      initialize()
    } catch (error: unknown) {
      replace(defaultUIPreferences)
      persistenceError.value =
        error instanceof UIPreferencesError && error.operation === 'write' ? 'write' : 'invalid'
      initialized.value = true
    }
  }

  function update(patch: Partial<UIPreferences>): void {
    const next = { ...preferences.value, ...patch }
    const hasPersistedUpdate = Object.keys(patch).some((key) => key !== 'theme')
    if (!hasPersistedUpdate) {
      replace(next)
      persistenceError.value = null
      return
    }
    try {
      writeUIPreferences(next)
    } catch {
      persistenceError.value = 'write'
      return
    }
    replace(next)
    persistenceError.value = null
  }

  function reset(): void {
    const next = { ...defaultUIPreferences }
    try {
      writeUIPreferences(next)
    } catch {
      persistenceError.value = 'write'
      return
    }
    replace(next)
    persistenceError.value = null
  }

  function replace(next: Readonly<UIPreferences>): void {
    const normalized = { ...next, primaryColor: next.primaryColor.toUpperCase() }
    preferences.value = normalized
    applyTheme(normalized.theme)
    applyPrimaryColor(normalized.primaryColor)
  }

  return {
    preferences,
    persistenceError,
    initialized,
    initialize,
    initializeSafely,
    update,
    reset,
  }
})
