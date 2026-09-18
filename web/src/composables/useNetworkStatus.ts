import { computed, onMounted, onUnmounted, readonly, shallowRef } from 'vue'

export function useNetworkStatus() {
  const isOnline = shallowRef(readOnlineStatus())
  const lastOfflineAt = shallowRef<Date | null>(isOnline.value ? null : new Date())

  function markOnline(): void {
    isOnline.value = true
  }

  function markOffline(): void {
    if (isOnline.value || lastOfflineAt.value === null) lastOfflineAt.value = new Date()
    isOnline.value = false
  }

  function syncStatus(): void {
    if (readOnlineStatus()) {
      markOnline()
      return
    }
    markOffline()
  }

  onMounted(() => {
    syncStatus()
    window.addEventListener('online', markOnline)
    window.addEventListener('offline', markOffline)
  })

  onUnmounted(() => {
    window.removeEventListener('online', markOnline)
    window.removeEventListener('offline', markOffline)
  })

  return {
    isOnline: readonly(isOnline),
    isOffline: computed(() => !isOnline.value),
    lastOfflineAt: readonly(lastOfflineAt),
    refreshPage: () => window.location.reload(),
  }
}

function readOnlineStatus(): boolean {
  return typeof navigator === 'undefined' || navigator.onLine !== false
}
