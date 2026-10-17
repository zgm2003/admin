<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  getConfigCacheStatuses,
  getConfigCacheStatusOptions,
  type ConfigCacheStatus,
  type ConfigCacheStatusPublishState,
} from '@/api/system/configCacheStatus'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import ConfigCacheStatusDetailDialog from './components/ConfigCacheStatusDetailDialog/index.vue'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'

const { t, locale } = useI18n()
const {
  options,
  error: optionsError,
  loading: optionsLoading,
  reload: reloadOptions,
} = useLocalizedOptions(getConfigCacheStatusOptions, () => ({ publishStates: [] }))
const access = usePermissionStore()
const rows = ref<ConfigCacheStatus[]>([])
const loading = ref(false)
const loadError = ref('')
const keyword = ref('')
const publishState = ref<'' | ConfigCacheStatusPublishState>('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const detailVisible = ref(false)
const selectedRow = ref<ConfigCacheStatus | null>(null)
interface ConfigCacheStatusSearchModel {
  keyword: string
  publishState: '' | ConfigCacheStatusPublishState
}

let sequence = 0
let mounted = true
onBeforeUnmount(() => {
  mounted = false
  sequence++
})

const canList = computed(() => access.hasPermission('system:configCacheStatus:list'))
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
const searchModel = computed<SearchFormModel<ConfigCacheStatusSearchModel>>({
  get: () => ({ keyword: keyword.value, publishState: publishState.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    publishState.value = typeof value.publishState === 'string' ? value.publishState : ''
  },
})
const publishStateOptions = computed(() => options.value.publishStates)
const searchFields = computed<SearchField<ConfigCacheStatusSearchModel>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    resetValue: '',
    label: t('configCacheStatus.keyword'),
    placeholder: t('configCacheStatus.searchPlaceholder'),
    clearable: true,
    testId: 'config-cache-status-keyword',
  },
  {
    key: 'publishState',
    type: 'select-v2',
    resetValue: '',
    label: t('configCacheStatus.publishState'),
    placeholder: t('configCacheStatus.allStates'),
    options: publishStateOptions.value,
    disabled: optionsLoading.value || optionsError.value !== '',
    clearable: true,
    testId: 'config-cache-status-publish-state',
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
const columns = computed<TableColumn<ConfigCacheStatus>[]>(() => [
  { prop: 'namespace', label: t('configCacheStatus.namespace'), minWidth: 180 },
  { prop: 'scopeKey', label: t('configCacheStatus.scopeKey'), minWidth: 140 },
  { key: 'status', prop: 'status', label: t('configCacheStatus.status'), width: 130 },
  { prop: 'pendingCount', label: t('configCacheStatus.pendingCount'), width: 120 },
  {
    key: 'redisVersion',
    prop: 'latestPublishedGeneration',
    label: t('configCacheStatus.redisVersion'),
    width: 120,
  },
  {
    key: 'lastError',
    prop: 'lastError',
    label: t('configCacheStatus.lastError'),
    minWidth: 200,
    overflowTooltip: true,
  },
  {
    key: 'latestPublishedAt',
    prop: 'latestPublishedAt',
    label: t('configCacheStatus.latestPublishedAt'),
    width: 190,
  },
  { key: 'actions', prop: 'namespace', label: t('configCacheStatus.details'), width: 140 },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))

function displayTime(value: string | null): string {
  return value === null ? '-' : formatTime(value)
}
function displayRedisVersion(row: ConfigCacheStatus): string {
  return row.publishedVersion === null ? '-' : String(row.publishedVersion)
}
function rowKey(row: ConfigCacheStatus): string {
  return `${row.namespace}:${row.scopeKey}`
}
function openDetail(row: ConfigCacheStatus): void {
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
    const result = await getConfigCacheStatuses({
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
      loadError.value = t('configCacheStatus.loadFailed')
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
  <AppPage class="config-cache-status-page">
    <div v-if="optionsError"><el-button @click="reloadOptions">{{ t('appTable.refresh') }}</el-button></div>
    <AppSearch
      v-model="searchModel"
      class="management-page__filters"
      :fields="searchFields"
      :query-label="t('search.query')"
      :reset-label="t('search.reset')"
      query-test-id="config-cache-status-search"
      reset-test-id="config-cache-status-reset"
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
      :aria-label="t('configCacheStatus.title')"
      :refresh-label="t('appTable.refresh')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #cell-namespace="{ row }: { row: ConfigCacheStatus }">
        {{ row.namespaceLabel }}
      </template>
      <template #cell-scopeKey="{ row }: { row: ConfigCacheStatus }">
        <template v-if="row.namespace === 'storage.cosconfig'">
          <span>
            {{ row.scopeName || row.scopeLabel }}
          </span>
          <span v-if="row.scopeName" class="config-cache-status-page__scope-id"
            >#{{ row.scopeKey }}</span
          >
          <el-tag
            v-if="row.scopeDeleted || row.scopeMissing"
            size="small"
            type="info"
            data-testid="config-cache-status-cos-deleted"
          >
            {{
              row.scopeMissing
                ? t('configCacheStatus.cosConfigMissing')
                : t('configCacheStatus.cosConfigDeleted')
            }}
          </el-tag>
        </template>
        <span v-else>{{ row.scopeLabel }}</span>
      </template>
      <template #cell-status="{ row }: { row: ConfigCacheStatus }">
        <el-tooltip v-if="row.statusHint" :content="row.statusHint">
          <el-tag :type="row.statusTone" data-testid="config-cache-status-status">
            {{ row.statusLabel }}
          </el-tag>
        </el-tooltip>
        <el-tag v-else :type="row.statusTone" data-testid="config-cache-status-status">
          {{ row.statusLabel }}
        </el-tag>
      </template>
      <template #cell-latestPublishedAt="{ row }: { row: ConfigCacheStatus }">{{
        displayTime(row.latestPublishedAt)
      }}</template>
      <template #cell-redisVersion="{ row }: { row: ConfigCacheStatus }">
        {{ displayRedisVersion(row) }}
      </template>
      <template #cell-actions="{ row }: { row: ConfigCacheStatus }">
        <el-button
          data-testid="config-cache-status-detail"
          text
          type="primary"
          @click="openDetail(row)"
        >
          {{ t('configCacheStatus.details') }}
        </el-button>
      </template>
      <template #empty
        ><el-empty
          data-testid="config-cache-status-empty"
          :description="t('configCacheStatus.empty')"
      /></template>
    </AppTable>
    <ConfigCacheStatusDetailDialog v-model="detailVisible" :row="selectedRow" />
  </AppPage>
</template>

<style scoped lang="scss">
.config-cache-status-page__error {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
  white-space: nowrap;
}

.config-cache-status-page__scope-id {
  margin-left: 6px;
  color: var(--el-text-color-secondary);
}
</style>
