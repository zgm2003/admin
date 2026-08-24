import { defineStore } from 'pinia'
import { ref } from 'vue'

import { applyPrimaryColor, applyTheme } from '../utils/theme'
import {
  defaultUIPreferences,
  readUIPreferences,
  writeUIPreferences,
  type UIPreferences,
} from '../utils/ui-preferences'

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
    } catch {
      replace(defaultUIPreferences)
      persistenceError.value = 'invalid'
      initialized.value = true
    }
  }

  function update(patch: Partial<UIPreferences>): void {
    const next = { ...preferences.value, ...patch }
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
