<script setup lang="ts">
import { Bell, Link } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { usePermissionStore } from '@/store/permission'
import { useNotificationStore } from '@/store/notification'
import type { NotificationRecent } from '@/api/message/notification'

const permission = usePermissionStore(),
  notification = useNotificationStore(),
  router = useRouter(),
  { t } = useI18n()
const visible = computed(() => permission.hasPermission('message:notification:list'))
const canRead = computed(() => permission.hasPermission('message:notification:read'))
const canView = computed(() => permission.hasPermission('message:notification:view'))
const badge = computed(() => (notification.unreadCount > 99 ? '99+' : notification.unreadCount))
watch(
  visible,
  (value) => {
    if (value) void notification.load().catch(() => undefined)
  },
  { immediate: true },
)
function domain(link: string): string {
  try {
    return new URL(link).hostname
  } catch {
    return link
  }
}
async function openItem(item: NotificationRecent): Promise<void> {
  if (!item.isRead && canRead.value) await notification.markRead(item.id).catch(() => undefined)
  if (item.linkType === 'external') {
    window.open(item.link, '_blank', 'noopener,noreferrer')
    return
  }
  if (item.linkType === 'internal') {
    const resolved = router.resolve(item.link)
    if (resolved.matched.length === 0) {
      ElMessage.error(t('notification.linkUnavailable'))
      return
    }
    try {
      await router.push(item.link)
    } catch {
      ElMessage.error(t('notification.linkUnavailable'))
    }
  }
}
async function viewAll(): Promise<void> {
  const path = '/message/notification'
  if (router.resolve(path).matched.length === 0) {
    ElMessage.error(t('notification.linkUnavailable'))
    return
  }
  try {
    await router.push(path)
  } catch {
    ElMessage.error(t('notification.linkUnavailable'))
  }
}
function readAll(): void {
  void notification.markAllRead().catch(() => undefined)
}
function retry(): void {
  void notification.load().catch(() => undefined)
}
</script>

<template>
  <el-popover v-if="visible" placement="bottom-end" :width="360" trigger="click">
    <template #reference>
      <el-badge :value="badge" :hidden="notification.unreadCount === 0" :max="99">
        <el-button
          data-testid="notification-bell"
          text
          :icon="Bell"
          :aria-label="t('notification.title')"
          :title="t('notification.title')"
        />
      </el-badge>
    </template>
    <div class="notification-bell__header">
      <strong>{{ t('notification.title') }}</strong
      ><el-button
        v-if="notification.unreadCount > 0 && canRead"
        data-testid="notification-bell-read-all"
        link
        type="primary"
        @click="readAll"
        >{{ t('notification.readAll') }}</el-button
      >
    </div>
    <div v-if="notification.loading" class="notification-bell__state">
      {{ t('notification.loading') }}
    </div>
    <div v-else-if="notification.errorMessage" class="notification-bell__state">
      <span>{{ notification.errorMessage }}</span
      ><el-button link type="primary" @click="retry">{{ t('notification.retry') }}</el-button>
    </div>
    <div v-else-if="notification.recent.length === 0" class="notification-bell__state">
      {{ t('notification.empty') }}
    </div>
    <button
      v-for="item in notification.recent"
      v-else
      :key="item.id"
      :class="['notification-bell__item', { 'is-unread': !item.isRead }]"
      :data-testid="`notification-bell-item-${item.id}`"
      type="button"
      @click="openItem(item)"
    >
      <span class="notification-bell__heading">
        <span class="notification-bell__title">{{ item.title }}</span>
        <span
          v-if="!item.isRead"
          class="notification-bell__unread"
          :title="t('notification.unread')"
          :aria-label="t('notification.unread')"
        />
      </span>
      <span class="notification-bell__summary">{{ item.summary }}</span>
      <span v-if="item.linkType === 'external'" class="notification-bell__link"
        ><el-icon><Link /></el-icon>{{ domain(item.link) }}</span
      >
    </button>
    <el-button
      v-if="canView"
      class="notification-bell__all"
      data-testid="notification-bell-view-all"
      link
      type="primary"
      @click="viewAll"
      >{{ t('notification.viewAll') }}</el-button
    >
  </el-popover>
</template>

<style scoped>
.notification-bell__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.notification-bell__state {
  display: flex;
  min-height: 72px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--el-text-color-secondary);
}
.notification-bell__item {
  position: relative;
  display: flex;
  width: 100%;
  min-height: 64px;
  flex-direction: column;
  gap: 4px;
  padding: 10px 8px 10px 12px;
  border: 0;
  border-top: 1px solid var(--el-border-color-lighter);
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background-color 160ms ease;
}
.notification-bell__item.is-unread {
  background: var(--el-color-primary-light-9);
}
.notification-bell__item.is-unread::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: var(--el-color-primary);
  content: '';
}
.notification-bell__item:hover {
  background: var(--el-fill-color-light);
}
.notification-bell__item.is-unread:hover {
  background: var(--el-color-primary-light-8);
}
.notification-bell__item:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: -2px;
}
.notification-bell__heading {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}
.notification-bell__title {
  min-width: 0;
  font-weight: 500;
  color: var(--el-text-color-primary);
  overflow-wrap: anywhere;
}
.notification-bell__item.is-unread .notification-bell__title {
  font-weight: 650;
}
.notification-bell__unread {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--el-color-primary);
}
.notification-bell__summary {
  color: var(--el-text-color-secondary);
  overflow-wrap: anywhere;
}
.notification-bell__link {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--el-color-primary);
}
.notification-bell__all {
  display: flex;
  margin: 8px auto 0;
}
</style>
