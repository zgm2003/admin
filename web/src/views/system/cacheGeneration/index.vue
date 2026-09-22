<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  getCacheGenerations,
  type CacheGeneration,
  type CacheGenerationPublishState,
  type CacheGenerationStatus,
} from '@/api/system/cacheGeneration'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'

const { t } = useI18n()
const access = usePermissionStore()
const rows = ref<CacheGeneration[]>([])
const loading = ref(false)
const loadError = ref('')
const keyword = ref('')
const publishState = ref<'' | CacheGenerationPublishState>('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
interface CacheGenerationSearchModel {
  keyword: string
  publishState: '' | CacheGenerationPublishState
}

const publishStates: readonly CacheGenerationPublishState[] = ['ready', 'pending', 'retrying']

const canList = computed(() => access.hasPermission('system:cacheGeneration:list'))
const searchModel = computed<SearchFormModel<CacheGenerationSearchModel>>({
  get: () => ({ keyword: keyword.value, publishState: publishState.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    publishState.value = publishStates.includes(value.publishState as CacheGenerationPublishState)
      ? (value.publishState as CacheGenerationPublishState)
      : ''
  },
})
const publishStateOptions = computed<Array<{ label: string; value: CacheGenerationPublishState }>>(
  () => [
    { label: t('cacheGeneration.publishStateReady'), value: 'ready' },
    { label: t('cacheGeneration.publishStatePending'), value: 'pending' },
    { label: t('cacheGeneration.publishStateRetrying'), value: 'retrying' },
  ],
)
const searchFields = computed<SearchField<CacheGenerationSearchModel>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    resetValue: '',
    label: t('cacheGeneration.keyword'),
    placeholder: t('cacheGeneration.searchPlaceholder'),
    clearable: true,
    testId: 'cache-generation-keyword',
  },
  {
    key: 'publishState',
    type: 'select-v2',
    resetValue: '',
    label: t('cacheGeneration.publishState'),
    options: publishStateOptions.value,
    clearable: true,
    testId: 'cache-generation-publish-state',
  },
])
const state = computed<'loading' | 'error' | 'empty' | 'success'>(() =>
  loading.value
    ? 'loading'
    : loadError.value !== ''
      ? 'error'
      : rows.value.length === 0
        ? 'empty'
        : 'success',
)
const columns = computed<TableColumn<CacheGeneration>[]>(() => [
  { prop: 'namespace', label: t('cacheGeneration.namespace'), minWidth: 180 },
  { prop: 'scopeKey', label: t('cacheGeneration.scopeKey'), minWidth: 140 },
  { prop: 'generation', label: t('cacheGeneration.generation'), width: 110 },
  { key: 'status', prop: 'status', label: t('cacheGeneration.status'), width: 130 },
  { prop: 'pendingCount', label: t('cacheGeneration.pendingCount'), width: 110 },
  {
    key: 'oldestPendingAt',
    prop: 'oldestPendingAt',
    label: t('cacheGeneration.oldestPendingAt'),
    width: 190,
  },
  { prop: 'latestAttempts', label: t('cacheGeneration.latestAttempts'), width: 110 },
  {
    key: 'lastError',
    prop: 'lastError',
    label: t('cacheGeneration.lastError'),
    minWidth: 200,
    overflowTooltip: true,
  },
  {
    key: 'latestPublishedGeneration',
    prop: 'latestPublishedGeneration',
    label: t('cacheGeneration.latestPublishedGeneration'),
    width: 170,
  },
  {
    key: 'latestPublishedAt',
    prop: 'latestPublishedAt',
    label: t('cacheGeneration.latestPublishedAt'),
    width: 190,
  },
  { key: 'updatedAt', prop: 'updatedAt', label: t('cacheGeneration.updatedAt'), width: 190 },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))

const statusLabels: Readonly<Record<CacheGenerationStatus, string>> = {
  ready: 'cacheGeneration.statusReady',
  pending: 'cacheGeneration.statusPending',
  retrying: 'cacheGeneration.statusRetrying',
  invalidating: 'cacheGeneration.statusInvalidating',
  missing: 'cacheGeneration.statusMissing',
  corrupt: 'cacheGeneration.statusCorrupt',
  unavailable: 'cacheGeneration.statusUnavailable',
}
const statusTagTypes: Readonly<
  Record<CacheGenerationStatus, 'success' | 'warning' | 'danger' | 'info'>
> = {
  ready: 'success',
  pending: 'warning',
  retrying: 'danger',
  invalidating: 'warning',
  missing: 'danger',
  corrupt: 'danger',
  unavailable: 'danger',
}

function statusLabel(status: CacheGenerationStatus): string {
  return t(statusLabels[status])
}
function statusTagType(status: CacheGenerationStatus) {
  return statusTagTypes[status]
}
function displayTime(value: string | null): string {
  return value === null ? '-' : formatTime(value)
}
function displayNumber(value: number | null): string {
  return value === null ? '-' : String(value)
}
function rowKey(row: CacheGeneration): string {
  return `${row.namespace}:${row.scopeKey}`
}

async function load(): Promise<void> {
  if (!canList.value) return
  loading.value = true
  loadError.value = ''
  try {
    const result = await getCacheGenerations({
      page: page.value,
      pageSize: pageSize.value,
      ...(keyword.value.trim() ? { keyword: keyword.value.trim() } : {}),
      ...(publishState.value === '' ? {} : { publishState: publishState.value }),
    })
    rows.value = result.list
    total.value = result.total
  } catch {
    loadError.value = t('cacheGeneration.loadFailed')
  } finally {
    loading.value = false
  }
}
function search(): void {
  page.value = 1
  void load()
}
function reset(): void {
  keyword.value = ''
  publishState.value = ''
  page.value = 1
  void load()
}
function updatePagination(next: TablePaginationState): void {
  page.value = next.currentPage
  pageSize.value = next.pageSize
  void load()
}

onMounted(() => {
  void load()
})
</script>

<template>
  <AppPage class="cache-generation-page">
    <AppSearch
      v-model="searchModel"
      class="management-page__filters"
      :fields="searchFields"
      :query-label="t('search.query')"
      :reset-label="t('search.reset')"
      query-test-id="cache-generation-search"
      reset-test-id="cache-generation-reset"
      @query="search"
      @reset="reset"
    />

    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :result-state="state"
      :status-message="loadError"
      :row-key="rowKey"
      :pagination="pagination"
      :aria-label="t('cacheGeneration.title')"
      :refresh-label="t('appTable.refresh')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #cell-status="{ row }: { row: CacheGeneration }">
        <el-tag :type="statusTagType(row.status)" data-testid="cache-generation-status">
          {{ statusLabel(row.status) }}
        </el-tag>
      </template>
      <template #cell-oldestPendingAt="{ row }: { row: CacheGeneration }">{{
        displayTime(row.oldestPendingAt)
      }}</template>
      <template #cell-latestPublishedGeneration="{ row }: { row: CacheGeneration }">{{
        displayNumber(row.latestPublishedGeneration)
      }}</template>
      <template #cell-latestPublishedAt="{ row }: { row: CacheGeneration }">{{
        displayTime(row.latestPublishedAt)
      }}</template>
      <template #cell-updatedAt="{ row }: { row: CacheGeneration }">{{
        displayTime(row.updatedAt)
      }}</template>
      <template #empty
        ><el-empty data-testid="cache-generation-empty" :description="t('cacheGeneration.empty')"
      /></template>
    </AppTable>
  </AppPage>
</template>

<style scoped>
.cache-generation-page__error {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
  white-space: nowrap;
}
</style>
