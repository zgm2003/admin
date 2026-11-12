<!-- 页面：AI 对话运行日志 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { View } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'

import * as api from '@/api/ai/chat'
import type { ChatRun, ChatRunDetail, ChatRunQuery } from '@/api/ai/chat'
import { usePermissionStore } from '@/store/permission'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'

defineOptions({ name: 'AIRunPage' })

const { t, locale } = useI18n()
const access = usePermissionStore()
const rows = ref<ChatRun[]>([])
const total = ref(0)
const query = ref<ChatRunQuery>({ page: 1, pageSize: 20 })
const filters = ref<SearchFormModel<{ keyword: string; status: number | ''; userId: string }>>({
  keyword: '',
  status: '',
  userId: '',
})
const loading = ref(false)
const loadError = ref('')
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<ChatRunDetail | null>(null)
let sequence = 0
let alive = true

const canViewAll = computed(() => access.hasPermission('ai:run:viewAll'))
const canList = computed(() => access.hasPermission('ai:run:list') || canViewAll.value)
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))

const statusOptions = computed(() => [
  { value: 1, label: t('ai.run.statusQueued') },
  { value: 2, label: t('ai.run.statusRunning') },
  { value: 3, label: t('ai.run.statusSucceeded') },
  { value: 4, label: t('ai.run.statusFailed') },
  { value: 5, label: t('ai.run.statusCanceled') },
  { value: 6, label: t('ai.run.statusTimeout') },
])

const searchFields = computed<
  SearchField<{ keyword: string; status: number | ''; userId: string }>[]
>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('ai.run.keyword'),
    placeholder: t('ai.run.keywordPlaceholder'),
    resetValue: '',
    width: 240,
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('ai.run.status'),
    placeholder: t('ai.run.allStatuses'),
    resetValue: '',
    options: statusOptions.value,
    width: 140,
  },
  ...(canViewAll.value
    ? [
        {
          key: 'userId' as const,
          type: 'input' as const,
          label: t('ai.run.userId'),
          placeholder: t('ai.run.userIdPlaceholder'),
          resetValue: '',
          width: 130,
        },
      ]
    : []),
])

const columns = computed<TableColumn<ChatRun>[]>(() => [
  { key: 'run', label: t('ai.run.run'), minWidth: 230 },
  ...(canViewAll.value ? [{ key: 'user', label: t('ai.run.user'), width: 140 }] : []),
  { key: 'status', label: t('ai.run.status'), width: 100 },
  { key: 'usage', label: t('ai.run.usage'), minWidth: 170 },
  { key: 'billing', label: t('ai.run.billing'), minWidth: 150 },
  { key: 'duration', label: t('ai.run.duration'), width: 120 },
  { key: 'createdAt', label: t('ai.run.createdAt'), width: 170 },
  { key: 'actions', label: t('common.actions'), width: 90, fixed: 'right' },
])

function state(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}

async function load(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getRuns(query.value, canViewAll.value)
    if (alive && current === sequence) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = t('ai.run.requestFailed')
    }
  } finally {
    if (alive && current === sequence) loading.value = false
  }
}

function search(): void {
  const userId = filters.value.userId.trim()
  if (userId !== '' && (!/^[1-9]\d*$/.test(userId) || !Number.isSafeInteger(Number(userId)))) {
    ElMessage.warning(t('ai.run.userIdInvalid'))
    return
  }
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(filters.value.keyword.trim() ? { keyword: filters.value.keyword.trim() } : {}),
    ...(filters.value.status !== '' ? { status: filters.value.status } : {}),
    ...(canViewAll.value && userId !== '' ? { userId: Number(userId) } : {}),
  }
  void load()
}

function reset(): void {
  filters.value = { keyword: '', status: '', userId: '' }
  search()
}

function updatePagination(value: TablePaginationState): void {
  query.value = {
    ...query.value,
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}

async function openDetail(row: ChatRun): Promise<void> {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    detail.value = await api.getRunDetail(row.id, canViewAll.value)
  } catch {
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

function statusTagType(status: number): 'primary' | 'success' | 'warning' | 'danger' | 'info' {
  switch (status) {
    case 2:
      return 'warning'
    case 3:
      return 'success'
    case 4:
      return 'danger'
    case 6:
      return 'warning'
    default:
      return 'info'
  }
}

function statusLabel(status: number): string {
  return statusOptions.value.find((item) => item.value === status)?.label ?? String(status)
}

function billingLabel(status: number): string {
  switch (status) {
    case 2:
      return t('ai.run.billingSettled')
    case 3:
      return t('ai.run.billingUnbilled')
    case 4:
      return t('ai.run.billingFailed')
    default:
      return t('ai.run.billingPending')
  }
}

function formatDate(value: string | null): string {
  if (value === null || value === '') return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(date)
}

function formatDuration(ms: number | null): string {
  if (ms === null) return '-'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(2)} s`
}

watch(
  [canList, locale],
  () => {
    sequence++
    rows.value = []
    total.value = 0
    if (canList.value) void load()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  alive = false
  sequence++
})
</script>

<template>
  <AppPage class="ai-run-page">
    <AppSearch
      v-model="filters"
      class="management-page__filters"
      :fields="searchFields"
      @query="search"
      @reset="reset"
    />

    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: ChatRun) => String(row.id)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('ai.run.title')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #toolbar-right>
        <span class="ai-run-page__hint">
          {{ canViewAll ? t('ai.run.viewAllHint') : t('ai.run.ownHint') }}
        </span>
      </template>
      <template #cell-run="{ row }: { row: ChatRun }">
        <div class="ai-run-page__stack">
          <strong>#{{ row.id }} · {{ row.agentName }}</strong>
          <span>{{ row.modelName || row.modelKey }}</span>
          <small>{{ row.requestId }}</small>
        </div>
      </template>
      <template #cell-user="{ row }: { row: ChatRun }">
        <div class="ai-run-page__stack">
          <strong>{{ row.username || `#${row.userId}` }}</strong>
          <small>{{ row.userId }}</small>
        </div>
      </template>
      <template #cell-status="{ row }: { row: ChatRun }">
        <el-tag :type="statusTagType(row.status)" effect="plain">{{
          statusLabel(row.status)
        }}</el-tag>
        <div v-if="row.errorCode" class="ai-run-page__error">{{ row.errorCode }}</div>
      </template>
      <template #cell-usage="{ row }: { row: ChatRun }">
        <div class="ai-run-page__stack">
          <span>{{
            t('ai.run.tokens', { input: row.inputTokens, output: row.outputTokens })
          }}</span>
          <small>{{ t('ai.run.cachedTokens', { count: row.cachedInputTokens }) }}</small>
        </div>
      </template>
      <template #cell-billing="{ row }: { row: ChatRun }">
        <div class="ai-run-page__stack">
          <strong>{{ row.amount || '-' }}</strong>
          <small>{{ billingLabel(row.billingStatus) }}</small>
        </div>
      </template>
      <template #cell-duration="{ row }: { row: ChatRun }">
        <div class="ai-run-page__stack">
          <span>{{ formatDuration(row.durationMs) }}</span>
          <small>{{ t('ai.run.firstToken', { value: formatDuration(row.firstTokenMs) }) }}</small>
        </div>
      </template>
      <template #cell-createdAt="{ row }: { row: ChatRun }">
        <span class="ai-run-page__muted">{{ formatDate(row.createdAt) }}</span>
      </template>
      <template #cell-actions="{ row }: { row: ChatRun }">
        <el-button link type="primary" :icon="View" @click="openDetail(row)">
          {{ t('common.detail') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('ai.run.empty')" />
      </template>
    </AppTable>

    <AppDialog
      v-model="detailVisible"
      :title="detail ? `#${detail.id} · ${detail.agentName}` : t('ai.run.detail')"
      width="760px"
    >
      <div v-loading="detailLoading" class="ai-run-page__detail">
        <template v-if="detail">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item :label="t('ai.run.user')">
              {{ detail.username || `#${detail.userId}` }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.status')">
              <el-tag :type="statusTagType(detail.status)" effect="plain">
                {{ statusLabel(detail.status) }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.model')">
              {{ detail.providerName }} · {{ detail.modelName }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.requestId')">
              <code>{{ detail.requestId }}</code>
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.tokensLabel')">
              {{ t('ai.run.tokens', { input: detail.inputTokens, output: detail.outputTokens }) }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.cachedTokensLabel')">
              {{ detail.cachedInputTokens }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.amount')">
              {{ detail.amount || '-' }}（{{ billingLabel(detail.billingStatus) }}）
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.balance')">
              {{ detail.balanceBefore || '-' }} → {{ detail.balanceAfter || '-' }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.firstTokenLabel')">
              {{ formatDuration(detail.firstTokenMs) }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.durationLabel')">
              {{ formatDuration(detail.durationMs) }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.createdAt')">
              {{ formatDate(detail.createdAt) }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('ai.run.finishedAt')">
              {{ formatDate(detail.finishedAt) }}
            </el-descriptions-item>
          </el-descriptions>

          <el-alert
            v-if="detail.errorMessage"
            class="ai-run-page__alert"
            type="error"
            :title="detail.errorMessage"
            :closable="false"
            show-icon
          />

          <h4>{{ t('ai.run.timeline') }}</h4>
          <el-timeline v-if="detail.events.length > 0">
            <el-timeline-item
              v-for="event in detail.events"
              :key="event.seq"
              :timestamp="formatDate(event.createdAt)"
              placement="top"
            >
              <strong>{{ event.type }}</strong>
              <span v-if="event.message"> · {{ event.message }}</span>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-else :description="t('ai.run.noEvents')" />
        </template>
      </div>
    </AppDialog>
  </AppPage>
</template>

<style scoped lang="scss">
.ai-run-page {
  min-width: 0;

  &__hint {
    color: var(--admin-text-muted);
    font-size: 12px;
  }

  &__stack {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 3px;

    strong,
    span,
    small {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    strong {
      color: var(--admin-text);
      font-size: 13px;
      font-weight: 600;
    }

    span {
      color: var(--admin-text-soft);
      font-size: 12px;
    }

    small {
      color: var(--admin-text-muted);
      font-size: 11px;
    }
  }

  &__error {
    margin-top: 4px;
    color: var(--el-color-danger);
    font-size: 11px;
  }

  &__muted {
    color: var(--admin-text-muted);
    font-size: 12px;
  }

  &__detail {
    min-height: 160px;
  }

  &__alert {
    margin-top: 12px;
  }

  &__detail h4 {
    margin: 20px 0 12px;
    color: var(--admin-text);
    font-size: 14px;
  }

  &__detail code {
    font-family: var(--admin-font-data);
    font-size: 11px;
    word-break: break-all;
  }
}
</style>
