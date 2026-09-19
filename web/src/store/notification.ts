import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  deleteNotification,
  getNotificationSummary,
  readAllNotifications,
  readNotification,
  type NotificationRecent,
} from '@/api/message/notification'
import { appI18n } from '@/i18n'

export const useNotificationStore = defineStore('notification', () => {
  const unreadCount = ref(0),
    recent = ref<NotificationRecent[]>([]),
    loading = ref(false),
    errorMessage = ref('')
  let generation = 0,
    pending: Promise<void> | null = null,
    refreshTimer: number | null = null
  function load(): Promise<void> {
    if (pending !== null) return pending
    const current = generation
    loading.value = true
    const request = getNotificationSummary()
      .then((summary) => {
        if (current !== generation) return
        unreadCount.value = summary.unreadCount
        recent.value = summary.recent.slice(0, 5)
        errorMessage.value = ''
      })
      .catch((error: unknown) => {
        if (current === generation)
          errorMessage.value =
            error instanceof Error ? error.message : appI18n.global.t('notification.loadFailed')
        throw error
      })
      .finally(() => {
        if (current === generation) loading.value = false
        if (pending === request) pending = null
      })
    pending = request
    return request
  }
  function scheduleRefresh(): void {
    if (refreshTimer !== null) return
    refreshTimer = window.setTimeout(() => {
      refreshTimer = null
      void load().catch(() => undefined)
    }, 250)
  }
  async function markRead(id: number): Promise<void> {
    await readNotification(id)
    const item = recent.value.find((v) => v.id === id)
    if (item && !item.isRead) {
      item.isRead = true
      unreadCount.value = Math.max(0, unreadCount.value - 1)
    }
    ElMessage.success(appI18n.global.t('notification.readSuccess'))
    scheduleRefresh()
  }
  async function markAllRead(): Promise<void> {
    await readAllNotifications()
    unreadCount.value = 0
    recent.value = recent.value.map((v) => ({ ...v, isRead: true }))
    ElMessage.success(appI18n.global.t('notification.readAllSuccess'))
    scheduleRefresh()
  }
  async function remove(id: number): Promise<void> {
    await deleteNotification(id)
    const item = recent.value.find((v) => v.id === id)
    if (item && !item.isRead) unreadCount.value = Math.max(0, unreadCount.value - 1)
    recent.value = recent.value.filter((v) => v.id !== id)
    ElMessage.success(appI18n.global.t('notification.deleteSuccess'))
    scheduleRefresh()
  }
  function reset(): void {
    generation++
    pending = null
    if (refreshTimer !== null) window.clearTimeout(refreshTimer)
    refreshTimer = null
    unreadCount.value = 0
    recent.value = []
    loading.value = false
    errorMessage.value = ''
  }
  return {
    unreadCount,
    recent,
    loading,
    errorMessage,
    load,
    scheduleRefresh,
    markRead,
    markAllRead,
    remove,
    reset,
  }
})
