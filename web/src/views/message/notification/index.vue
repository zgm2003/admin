<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  listNotifications,
  type NotificationItem,
  type NotificationPriority,
  type NotificationVariant,
} from '@/api/message/notification'
import { useNotificationStore } from '@/store/notification'
import { usePermissionStore } from '@/store/permission'
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
  <AppPage
    ><header class="notification-center__toolbar">
      <h1>{{ t('notification.center') }}</h1>
      <el-select-v2 v-model="filter" :options="filterOptions" /><el-select-v2
        v-model="variant"
        :options="[
          { value: '', label: t('notification.any') },
          { value: 'info', label: 'Info' },
          { value: 'success', label: 'Success' },
          { value: 'warning', label: 'Warning' },
          { value: 'error', label: 'Error' },
        ]"
      /><el-select-v2
        v-model="priority"
        :options="[
          { value: '', label: t('notification.any') },
          { value: 'normal', label: t('notification.normal') },
          { value: 'urgent', label: t('notification.urgent') },
        ]"
      /><el-button v-if="access.hasPermission('message:notification:read')" @click="markAllRead">{{
        t('notification.readAll')
      }}</el-button>
    </header>
    <el-empty v-if="!canList" :description="t('notification.noPermission')" />
    <div v-else-if="error" class="notification-center__state">
      {{ error }}<el-button link @click="load()">{{ t('notification.retry') }}</el-button>
    </div>
    <el-empty v-else-if="!loading && items.length === 0" :description="t('notification.empty')" />
    <div v-else class="notification-center__list">
      <article v-for="item in items" :key="item.id" class="notification-center__item">
        <div class="notification-center__heading">
          <h2>{{ item.title }}</h2>
          <div>
            <el-button
              v-if="!item.isRead && access.hasPermission('message:notification:read')"
              :data-testid="`notification-read-${item.id}`"
              link
              @click="markRead(item)"
              >{{ t('notification.markRead') }}</el-button
            ><el-button
              v-if="access.hasPermission('message:notification:delete')"
              :data-testid="`notification-delete-${item.id}`"
              link
              type="danger"
              @click="remove(item)"
              >{{ t('notification.delete') }}</el-button
            >
          </div>
        </div>
        <div class="notification-center__body" v-html="item.contentHtml" />
        <el-button v-if="item.linkType !== 'none'" link @click="open(item)">{{
          item.link
        }}</el-button>
      </article>
      <el-button v-if="nextBeforeId !== null" :loading="loading" @click="load(true)">{{
        t('notification.loadMore')
      }}</el-button>
    </div></AppPage
  >
</template>
<style scoped>
.notification-center__toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.notification-center__toolbar h1 {
  margin: 0 auto 0 0;
  font-size: 20px;
}
.notification-center__list {
  border-top: 1px solid var(--el-border-color);
}
.notification-center__item {
  min-height: 132px;
  padding: 16px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.notification-center__heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.notification-center__heading h2 {
  margin: 0;
  font-size: 16px;
  overflow-wrap: anywhere;
}
.notification-center__body {
  margin: 10px 0;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.notification-center__state {
  display: flex;
  justify-content: center;
  gap: 8px;
  padding: 40px;
}
</style>
