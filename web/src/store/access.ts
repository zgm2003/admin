import { defineStore } from 'pinia'
import { ref } from 'vue'

import { getAccess } from '../api/rbac/access'
import type { AccessMenuNode, AccessSnapshot } from '../api/rbac/access'
import { appI18n } from '../i18n'
import { ApiError, ProtocolError } from '../types/http'

export type AccessStatus = 'idle' | 'loading' | 'ready' | 'error'

export const useAccessStore = defineStore('access', () => {
  const roleCodes = ref<string[]>([])
  const menuTree = ref<AccessMenuNode[]>([])
  const permissionCodes = ref<string[]>([])
  const status = ref<AccessStatus>('idle')
  const errorMessage = ref('')
  let loadPromise: Promise<void> | null = null
  let generation = 0

  function hasPermission(code: string): boolean {
    return permissionCodes.value.includes(code)
  }

  function applySnapshot(snapshot: AccessSnapshot): void {
    roleCodes.value = [...snapshot.roleCodes]
    menuTree.value = snapshot.menuTree
    permissionCodes.value = [...snapshot.permissionCodes]
    status.value = 'ready'
    errorMessage.value = ''
  }

  function fail(error: unknown): void {
    roleCodes.value = []
    menuTree.value = []
    permissionCodes.value = []
    status.value = 'error'
    errorMessage.value = error instanceof ProtocolError
      ? appI18n.global.t('access.invalidProtocol')
      : error instanceof ApiError && error.message !== ''
        ? error.message
        : appI18n.global.t('access.loadFailed')
  }

  function reset(): void {
    generation += 1
    loadPromise = null
    roleCodes.value = []
    menuTree.value = []
    permissionCodes.value = []
    status.value = 'idle'
    errorMessage.value = ''
  }

  function load(): Promise<void> {
    if (status.value === 'ready') return Promise.resolve()
    if (loadPromise !== null) return loadPromise

    status.value = 'loading'
    const requestGeneration = generation
    const pending = getAccess()
      .then((snapshot) => {
        if (generation === requestGeneration) applySnapshot(snapshot)
      })
      .catch((error: unknown) => {
        if (generation === requestGeneration) fail(error)
        throw error
      })
    loadPromise = pending
    pending.finally(() => {
      if (loadPromise === pending) loadPromise = null
    }).catch(() => undefined)
    return pending
  }

  return {
    roleCodes,
    menuTree,
    permissionCodes,
    status,
    errorMessage,
    hasPermission,
    applySnapshot,
    fail,
    load,
    reset,
  }
})
