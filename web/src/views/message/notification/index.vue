<script setup lang="ts">
import { CheckCheck } from 'lucide-vue-next'
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  notificationPriorityMetadata,
  notificationVariantMetadata,
  listNotifications,
  type NotificationItem,
  type NotificationPriority,
  type NotificationVariant,
} from '@/api/message/notification'
import { useNotificationStore } from '@/store/notification'
import { usePermissionStore } from '@/store/permission'

import NotificationCenterList from './components/NotificationCenterList/index.vue'

const access = usePermissionStore(),
  store = useNotificationStore(),
  router = useRouter(),
  { t } = useI18n()
const items = ref<NotificationItem[]>([]),
  loading = ref(false),
  error = ref(''),
  nextBeforeId = ref<number | null>(null),
  filter = ref<'all' | 'unread'>('all'),
  variant = ref<NotificationVariant | ''>(''),
  priority = ref<NotificationPriority | ''>('')
let sequence = 0
const canList = computed(() => access.hasPermission('message:notification:list'))
const filterOptions = computed(() => [
  { value: 'all', label: t('notification.filterAll') },
  { value: 'unread', label: t('notification.filterUnread') },
])
const variantOptions = computed(() => [
  { value: '', label: t('notification.variantAll') },
  ...notificationVariantMetadata.map((item) => ({
    value: item.value,
    label: t(item.i18nKey),
  })),
])
const priorityOptions = computed(() => [
  { value: '', label: t('notification.priorityAll') },
  ...notificationPriorityMetadata.map((item) => ({
    value: item.value,
    label: t(item.i18nKey),
  })),
])
async function load(append = false): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  error.value = ''
  try {
    const query: {
      beforeId?: number
      limit: number
      filter: 'all' | 'unread'
      variant?: NotificationVariant
      priority?: NotificationPriority
    } = { limit: 20, filter: filter.value }
    if (append && nextBeforeId.value !== null) query.beforeId = nextBeforeId.value
    if (variant.value !== '') query.variant = variant.value
    if (priority.value !== '') query.priority = priority.value
    const result = await listNotifications(query)
    if (current !== sequence) return
    items.value = append ? [...items.value, ...result.items] : result.items
    nextBeforeId.value = result.nextBeforeId
  } catch (e) {
    if (current === sequence)
      error.value = e instanceof Error ? e.message : t('notification.loadFailed')
  } finally {
    if (current === sequence) loading.value = false
  }
}
async function markRead(item: NotificationItem): Promise<void> {
  try {
    await store.markRead(item.id)
    if (filter.value === 'unread') items.value = items.value.filter((value) => value.id !== item.id)
    else item.isRead = true
  } catch {
    // request.ts emits the single API error notification
  }
}
async function markAllRead(): Promise<void> {
  try {
    await store.markAllRead()
    if (filter.value === 'unread') {
      items.value = []
      nextBeforeId.value = null
    } else items.value = items.value.map((item) => ({ ...item, isRead: true }))
  } catch {
    // request.ts emits the single API error notification
  }
}
async function remove(item: NotificationItem): Promise<void> {
  try {
    await store.remove(item.id)
    items.value = items.value.filter((value) => value.id !== item.id)
  } catch {
    // request.ts emits the single API error notification
  }
}
watch([filter, variant, priority], () => {
  items.value = []
  nextBeforeId.value = null
  void load()
})
function open(item: NotificationItem): void {
  if (item.linkType === 'external') window.open(item.link, '_blank', 'noopener,noreferrer')
  else if (item.linkType === 'internal') {
    const resolved = router.resolve(item.link)
    if (resolved.matched.length === 0) {
      ElMessage.error(t('notification.linkUnavailable'))
      return
    }
    void router.push(item.link)
  }
}
onMounted(() => void load())
</script>
<template>
  <AppPage class="notification-center">
    <header class="notification-center__header">
      <h1>{{ t('notification.center') }}</h1>
      <el-button
        v-if="access.hasPermission('message:notification:read')"
        :icon="CheckCheck"
        plain
        type="primary"
        @click="markAllRead"
      >
        {{ t('notification.readAll') }}
      </el-button>
    </header>
    <div class="notification-center__filters">
      <el-segmented
        v-model="filter"
        data-testid="notification-filter-mode"
        :options="filterOptions"
      />
      <div class="notification-center__selectors">
        <label class="notification-center__filter">
          <span class="notification-center__filter-label">{{
            t('notification.variantLabel')
          }}</span>
          <el-select-v2
            v-model="variant"
            data-testid="notification-variant-filter"
            :options="variantOptions"
            :aria-label="t('notification.variantLabel')"
          />
        </label>
        <label class="notification-center__filter">
          <span class="notification-center__filter-label">{{
            t('notification.priorityLabel')
          }}</span>
          <el-select-v2
            v-model="priority"
            data-testid="notification-priority-filter"
            :options="priorityOptions"
            :aria-label="t('notification.priorityLabel')"
          />
        </label>
      </div>
    </div>
    <el-empty v-if="!canList" :description="t('notification.noPermission')" />
    <div v-else-if="error" class="notification-center__state">
      {{ error }}<el-button link @click="load()">{{ t('notification.retry') }}</el-button>
    </div>
    <div v-else-if="loading && items.length === 0" class="notification-center__state">
      {{ t('notification.loading') }}
    </div>
    <el-empty v-else-if="!loading && items.length === 0" :description="t('notification.empty')" />
    <NotificationCenterList
      v-else
      :items="items"
      :loading="loading"
      :next-before-id="nextBeforeId"
      :can-read="access.hasPermission('message:notification:read')"
      :can-delete="access.hasPermission('message:notification:delete')"
      @read="markRead"
      @remove="remove"
      @open="open"
      @load-more="load(true)"
    />
  </AppPage>
</template>
<style scoped>
.notification-center {
  gap: 0;
}

.notification-center__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.notification-center__header h1 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 20px;
  font-weight: 650;
  letter-spacing: 0;
}

.notification-center__filters {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.notification-center__selectors,
.notification-center__filter {
  display: flex;
  align-items: center;
}

.notification-center__selectors {
  gap: 16px;
}

.notification-center__filter {
  gap: 8px;
}

.notification-center__filter-label {
  flex: 0 0 auto;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.notification-center__filter :deep(.el-select) {
  width: 150px;
}

.notification-center__state {
  display: flex;
  min-height: 180px;
  justify-content: center;
  align-items: center;
  gap: 8px;
  padding: 40px;
  color: var(--el-text-color-secondary);
}

@media (max-width: 768px) {
  .notification-center__header {
    align-items: flex-start;
  }

  .notification-center__filters,
  .notification-center__selectors {
    align-items: stretch;
    flex-direction: column;
  }

  .notification-center__filters :deep(.el-segmented) {
    align-self: flex-start;
  }

  .notification-center__filter {
    display: grid;
    grid-template-columns: 64px minmax(0, 1fr);
  }

  .notification-center__filter :deep(.el-select) {
    width: 100%;
  }
}
</style>
