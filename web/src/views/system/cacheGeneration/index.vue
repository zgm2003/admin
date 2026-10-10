<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { View } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import {
  getCacheGenerations,
  getCacheGenerationOptions,
  type CacheGeneration,
  type CacheGenerationPublishState,
} from '@/api/system/cacheGeneration'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import CacheGenerationDetailDialog from './components/CacheGenerationDetailDialog/index.vue'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'

const { t, locale } = useI18n()
const {
  options,
  error: optionsError,
  loading: optionsLoading,
  reload: reloadOptions,
} = useLocalizedOptions(getCacheGenerationOptions, () => ({ publishStates: [] }))
const access = usePermissionStore()
const rows = ref<CacheGeneration[]>([])
const loading = ref(false)
const loadError = ref('')
const keyword = ref('')
const publishState = ref<'' | CacheGenerationPublishState>('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const selectedRow = ref<CacheGeneration | null>(null)
interface CacheGenerationSearchModel {
  keyword: string
  publishState: '' | CacheGenerationPublishState
}

let sequence = 0
let mounted = true
onBeforeUnmount(() => {
  mounted = false
  sequence++
})

const canList = computed(() => access.hasPermission('system:cacheGeneration:list'))
watch([locale, canList], () => {
  sequence++
  rows.value = []
  total.value = 0
  loading.value = false
  loadError.value = ''
  selectedRow.value = null
  detailVisible.value = false
  if (canList.value) void load()
})
const searchModel = computed<SearchFormModel<CacheGenerationSearchModel>>({
  get: () => ({ keyword: keyword.value, publishState: publishState.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    publishState.value = typeof value.publishState === 'string' ? value.publishState : ''
  },
})
const publishStateOptions = computed(() => options.value.publishStates)
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
    placeholder: t('cacheGeneration.allStates'),
    options: publishStateOptions.value,
    disabled: optionsLoading.value || optionsError.value !== '',
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
  { key: 'status', prop: 'status', label: t('cacheGeneration.status'), width: 130 },
  { prop: 'pendingCount', label: t('cacheGeneration.pendingCount'), width: 120 },
  {
    key: 'redisVersion',
    prop: 'latestPublishedGeneration',
    label: t('cacheGeneration.redisVersion'),
    width: 120,
  },
  {
    key: 'lastError',
    prop: 'lastError',
    label: t('cacheGeneration.lastError'),
    minWidth: 200,
    overflowTooltip: true,
  },
  {
    key: 'latestPublishedAt',
    prop: 'latestPublishedAt',
    label: t('cacheGeneration.latestPublishedAt'),
    width: 190,
  },
  { key: 'actions', prop: 'namespace', label: t('cacheGeneration.details'), width: 140 },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))

function displayTime(value: string | null): string {
  return value === null ? '-' : formatTime(value)
}
function displayRedisVersion(row: CacheGeneration): string {
  return row.publishedVersion === null ? '-' : String(row.publishedVersion)
}
function rowKey(row: CacheGeneration): string {
  return `${row.namespace}:${row.scopeKey}`
}
function openDetail(row: CacheGeneration): void {
  selectedRow.value = row
  detailVisible.value = true
}

async function load(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  const accepted = () => mounted && current === sequence && canList.value
  loading.value = true
  loadError.value = ''
  try {
    const result = await getCacheGenerations({
      page: page.value,
      pageSize: pageSize.value,
      ...(keyword.value.trim() ? { keyword: keyword.value.trim() } : {}),
      ...(publishState.value === '' ? {} : { publishState: publishState.value }),
    })
    if (accepted()) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (accepted()) {
      rows.value = []
      total.value = 0
      loadError.value = t('cacheGeneration.loadFailed')
    }
  } finally {
    if (accepted()) loading.value = false
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
    <el-alert
      v-if="optionsError"
      :title="t('cacheGeneration.loadFailed')"
      type="error"
      :closable="false"
      show-icon
    >
      <el-button @click="reloadOptions">{{ t('appTable.refresh') }}</el-button>
    </el-alert>
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
      <template #cell-namespace="{ row }: { row: CacheGeneration }">
        {{ row.namespaceLabel }}
      </template>
      <template #cell-scopeKey="{ row }: { row: CacheGeneration }">
        {{ row.scopeLabel }}
      </template>
      <template #cell-status="{ row }: { row: CacheGeneration }">
        <el-tooltip v-if="row.statusHint" :content="row.statusHint">
          <el-tag :type="row.statusTone" data-testid="cache-generation-status">
            {{ row.statusLabel }}
          </el-tag>
        </el-tooltip>
        <el-tag v-else :type="row.statusTone" data-testid="cache-generation-status">
          {{ row.statusLabel }}
        </el-tag>
      </template>
      <template #cell-latestPublishedAt="{ row }: { row: CacheGeneration }">{{
        displayTime(row.latestPublishedAt)
      }}</template>
      <template #cell-redisVersion="{ row }: { row: CacheGeneration }">
        {{ displayRedisVersion(row) }}
      </template>
      <template #cell-actions="{ row }: { row: CacheGeneration }">
        <el-button data-testid="cache-generation-detail" text :icon="View" @click="openDetail(row)">
          {{ t('cacheGeneration.details') }}
        </el-button>
      </template>
      <template #empty
        ><el-empty data-testid="cache-generation-empty" :description="t('cacheGeneration.empty')"
      /></template>
    </AppTable>
    <CacheGenerationDetailDialog v-model="detailVisible" :row="selectedRow" />
  </AppPage>
</template>

<style scoped lang="scss">
.cache-generation-page__error {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
  white-space: nowrap;
}
</style>
