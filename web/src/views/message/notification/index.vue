<script setup lang="ts">
import { ArrowUpRight, Check, CheckCheck, Trash2 } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import {
  listNotifications,
  notificationPriorityMetadata,
  notificationVariantMetadata,
  type NotificationItem,
  type NotificationPriority,
  type NotificationQuery,
  type NotificationVariant,
} from '@/api/message/notification'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import type { TableColumn } from '@/components/AppTable'
import { useNotificationStore } from '@/store/notification'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'

const access = usePermissionStore(),
  store = useNotificationStore(),
  router = useRouter(),
  { t } = useI18n()
const rows = ref<NotificationItem[]>([]),
  loading = ref(false),
  loadError = ref(''),
  nextBeforeId = ref<number | null>(null),
  unreadOnly = ref<'' | 'unread'>(''),
  variant = ref<NotificationVariant | ''>(''),
  priority = ref<NotificationPriority | ''>('')
let sequence = 0
const canList = computed(() => access.hasPermission('message:notification:list'))
const canRead = computed(() => access.hasPermission('message:notification:read'))
const canDelete = computed(() => access.hasPermission('message:notification:delete'))

interface NotificationSearchModel {
  unreadOnly: '' | 'unread'
  variant: NotificationVariant | ''
  priority: NotificationPriority | ''
}

function isVariant(value: unknown): value is NotificationVariant {
  return notificationVariantMetadata.some((item) => item.value === value)
}

function isPriority(value: unknown): value is NotificationPriority {
  return notificationPriorityMetadata.some((item) => item.value === value)
}

const searchModel = computed<SearchFormModel<NotificationSearchModel>>({
  get: () => ({ unreadOnly: unreadOnly.value, variant: variant.value, priority: priority.value }),
  set: (value) => {
    unreadOnly.value = value.unreadOnly === 'unread' ? 'unread' : ''
    variant.value = isVariant(value.variant) ? value.variant : ''
    priority.value = isPriority(value.priority) ? value.priority : ''
  },
})
const searchFields = computed<SearchField<NotificationSearchModel>[]>(() => [
  {
    key: 'unreadOnly',
    type: 'select-v2',
    resetValue: '',
    label: t('notification.columnStatus'),
    placeholder: t('notification.allStatuses'),
    options: [{ label: t('notification.unread'), value: 'unread' }],
    clearable: true,
    width: 150,
    testId: 'notification-unread-filter',
  },
  {
    key: 'variant',
    type: 'select-v2',
    resetValue: '',
    label: t('notification.variantLabel'),
    placeholder: t('notification.variantAll'),
    options: notificationVariantMetadata.map((item) => ({
      label: t(item.i18nKey),
      value: item.value,
    })),
    clearable: true,
    width: 160,
    testId: 'notification-variant-filter',
  },
  {
    key: 'priority',
    type: 'select-v2',
    resetValue: '',
    label: t('notification.priorityLabel'),
    placeholder: t('notification.priorityAll'),
    options: notificationPriorityMetadata.map((item) => ({
      label: t(item.i18nKey),
      value: item.value,
    })),
    clearable: true,
    width: 160,
    testId: 'notification-priority-filter',
  },
])
const variantTagTypes: Record<NotificationVariant, 'primary' | 'success' | 'warning' | 'danger'> = {
  info: 'primary',
  success: 'success',
  warning: 'warning',
  error: 'danger',
}
const columns = computed<TableColumn<NotificationItem>[]>(() => [
  { key: 'expand', prop: 'id', label: '', width: 48, expand: true },
  { key: 'title', prop: 'title', label: t('notification.columnTitle'), minWidth: 260 },
  { key: 'variant', prop: 'variant', label: t('notification.columnVariant'), width: 100 },
  { key: 'priority', prop: 'priority', label: t('notification.columnPriority'), width: 100 },
  { key: 'isRead', prop: 'isRead', label: t('notification.columnStatus'), width: 100 },
  {
    key: 'publishedAt',
    prop: 'publishedAt',
    label: t('notification.columnPublishedAt'),
    width: 190,
  },
  { key: 'actions', prop: 'id', label: t('notification.columnActions'), width: 190 },
])

async function load(append = false): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const query: NotificationQuery = {
      limit: 20,
      filter: unreadOnly.value === 'unread' ? 'unread' : 'all',
    }
    if (append && nextBeforeId.value !== null) query.beforeId = nextBeforeId.value
    if (variant.value !== '') query.variant = variant.value
    if (priority.value !== '') query.priority = priority.value
    const result = await listNotifications(query)
    if (current !== sequence) return
    rows.value = append ? [...rows.value, ...result.items] : result.items
    nextBeforeId.value = result.nextBeforeId
  } catch (error: unknown) {
    if (current === sequence)
      loadError.value =
        error instanceof Error && error.message !== ''
          ? error.message
          : t('notification.loadFailed')
  } finally {
    if (current === sequence) loading.value = false
  }
}
function reload(): void {
  rows.value = []
  nextBeforeId.value = null
  void load()
}
function search(): void {
  reload()
}
function reset(): void {
  unreadOnly.value = ''
  variant.value = ''
  priority.value = ''
  reload()
}
async function markRead(item: NotificationItem): Promise<void> {
  try {
    await store.markRead(item.id)
    if (unreadOnly.value === 'unread') rows.value = rows.value.filter((row) => row.id !== item.id)
    else item.isRead = true
  } catch {
    // request.ts emits the single API error notification
  }
}
async function markAllRead(): Promise<void> {
  try {
    await store.markAllRead()
    if (unreadOnly.value === 'unread') {
      rows.value = []
      nextBeforeId.value = null
    } else rows.value = rows.value.map((item) => ({ ...item, isRead: true }))
  } catch {
    // request.ts emits the single API error notification
  }
}
async function remove(item: NotificationItem): Promise<void> {
  try {
    await store.remove(item.id)
    rows.value = rows.value.filter((row) => row.id !== item.id)
  } catch {
    // request.ts emits the single API error notification
  }
}
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
    <AppSearch
      v-model="searchModel"
      class="management-page__filters"
      :fields="searchFields"
      query-test-id="notification-search"
      reset-test-id="notification-reset"
      @query="search"
      @reset="reset"
    />

    <el-empty v-if="!canList" :description="t('notification.noPermission')" />
    <template v-else>
      <el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
      <el-skeleton
        v-if="loading && rows.length === 0"
        data-testid="notification-loading"
        :rows="6"
        animated
      />
      <AppTable
        v-else
        data-testid="notification-table"
        :columns="columns"
        :data="rows"
        row-key="id"
        :aria-label="t('notification.center')"
        :refresh-label="t('notification.refresh')"
        @refresh="reload"
      >
        <template #toolbar-left>
          <el-button
            v-if="canRead"
            data-testid="notification-read-all"
            :icon="CheckCheck"
            plain
            type="primary"
            @click="markAllRead"
          >
            {{ t('notification.readAll') }}
          </el-button>
        </template>
        <template #cell-title="{ row }: { row: NotificationItem }">
          <div class="notification-center__title">
            <span>{{ row.title }}</span>
            <small v-if="row.summary !== ''">{{ row.summary }}</small>
          </div>
        </template>
        <template #cell-variant="{ row }: { row: NotificationItem }">
          <el-tag
            :data-testid="`notification-variant-${row.id}`"
            :type="variantTagTypes[row.variant]"
            effect="plain"
            size="small"
          >
            {{ t(`notification.variant.${row.variant}`) }}
          </el-tag>
        </template>
        <template #cell-priority="{ row }: { row: NotificationItem }">
          <el-tag
            v-if="row.priority === 'urgent'"
            :data-testid="`notification-priority-${row.id}`"
            effect="plain"
            size="small"
            type="danger"
          >
            {{ t('notification.priority.urgent') }}
          </el-tag>
          <span v-else>-</span>
        </template>
        <template #cell-isRead="{ row }: { row: NotificationItem }">
          <el-tag :type="row.isRead ? 'info' : 'primary'" effect="plain" size="small">
            {{ row.isRead ? t('notification.read') : t('notification.unread') }}
          </el-tag>
        </template>
        <template #cell-publishedAt="{ row }: { row: NotificationItem }">
          {{ formatTime(row.publishedAt) }}
        </template>
        <template #cell-actions="{ row }: { row: NotificationItem }">
          <el-button
            v-if="!row.isRead && canRead"
            :data-testid="`notification-read-${row.id}`"
            :icon="Check"
            text
            type="primary"
            @click="markRead(row)"
          >
            {{ t('notification.markRead') }}
          </el-button>
          <el-button
            v-if="canDelete"
            :data-testid="`notification-delete-${row.id}`"
            :icon="Trash2"
            text
            type="danger"
            @click="remove(row)"
          >
            {{ t('notification.delete') }}
          </el-button>
        </template>
        <template #expand="{ row }: { row: NotificationItem }">
          <div class="notification-center__detail">
            <div class="notification-center__body" v-html="row.contentHtml" />
            <el-button
              v-if="row.linkType !== 'none'"
              :icon="ArrowUpRight"
              link
              type="primary"
              @click="open(row)"
            >
              {{ t('notification.openLink') }}
            </el-button>
          </div>
        </template>
        <template #empty>
          <el-empty data-testid="notification-empty" :description="t('notification.empty')" />
        </template>
      </AppTable>
      <div v-if="nextBeforeId !== null" class="notification-center__more">
        <el-button
          data-testid="notification-load-more"
          :loading="loading"
          plain
          @click="load(true)"
        >
          {{ t('notification.loadMore') }}
        </el-button>
      </div>
    </template>
  </AppPage>
</template>
<style scoped>
.notification-center__title {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.notification-center__title small {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.notification-center__detail {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 4px 8px;
}

.notification-center__body {
  color: var(--el-text-color-regular);
  font-size: 14px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.notification-center__body :deep(> :first-child) {
  margin-top: 0;
}

.notification-center__body :deep(> :last-child) {
  margin-bottom: 0;
}

.notification-center__more {
  display: flex;
  justify-content: center;
  padding: 4px 0;
}
</style>
