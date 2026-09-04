<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { getLoginLogs } from '@/api/user/loginlog'
import type { LoginLogItem, LoginLogListQuery } from '@/api/user/loginlog'
import { AppTable } from '@/components/AppTable'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { AppSearch } from '@/components/AppSearch'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { formatTime } from '@/utils/datetime'

const { t } = useI18n()
const rows = ref<LoginLogItem[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')
const query = ref<LoginLogListQuery>({ page: 1, pageSize: 20 })
const account = ref('')
const eventType = ref('')
const success = ref<'' | 0 | 1>('')
const timeRange = ref<[] | [string, string]>([])

const searchModel = computed<SearchFormModel>({
  get: () => ({
    account: account.value,
    eventType: eventType.value,
    success: success.value,
    timeRange: timeRange.value,
  }),
  set: (value) => {
    account.value = typeof value.account === 'string' ? value.account : ''
    eventType.value = typeof value.eventType === 'string' ? value.eventType : ''
    success.value = value.success === 0 || value.success === 1 ? value.success : ''
    timeRange.value =
      Array.isArray(value.timeRange) && value.timeRange.length === 2
        ? [String(value.timeRange[0]), String(value.timeRange[1])]
        : []
  },
})
const searchFields = computed<SearchField[]>(() => [
  {
    key: 'account',
    type: 'input',
    label: t('loginLog.account'),
    placeholder: t('loginLog.account'),
    width: 220,
    testId: 'login-log-account',
  },
  {
    key: 'eventType',
    type: 'select-v2',
    label: t('loginLog.eventType'),
    options: [
      { label: t('loginLog.all'), value: '' },
      { label: t('loginLog.login'), value: 'login' },
      { label: t('loginLog.logout'), value: 'logout' },
    ],
    width: 140,
  },
  {
    key: 'success',
    type: 'select-v2',
    label: t('loginLog.success'),
    options: [
      { label: t('loginLog.all'), value: '' },
      { label: t('loginLog.successYes'), value: 1 },
      { label: t('loginLog.successNo'), value: 0 },
    ],
    width: 140,
  },
  {
    key: 'timeRange',
    type: 'date-range',
    label: t('loginLog.timeRange'),
    placeholder: t('loginLog.timeRange'),
    valueFormat: 'YYYY-MM-DDTHH:mm:ssZ',
    rangeSeparator: '-',
    width: 360,
  },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const columns = computed<TableColumn<LoginLogItem>[]>(() => [
  { prop: 'loginAccount', label: t('loginLog.account'), minWidth: 190, overflowTooltip: true },
  { prop: 'platform', label: t('loginLog.platform'), width: 120 },
  { key: 'event', prop: 'id', label: t('loginLog.eventType'), width: 110 },
  { key: 'status', prop: 'id', label: t('loginLog.status'), width: 100 },
  { prop: 'clientIp', label: t('loginLog.clientIp'), minWidth: 140 },
  { prop: 'reasonCode', label: t('loginLog.reason'), minWidth: 160, overflowTooltip: true },
  { prop: 'createdAt', label: t('loginLog.createdAt'), minWidth: 190 },
])

async function load(): Promise<void> {
  if (loading.value) return
  loading.value = true
  loadError.value = ''
  try {
    const result = await getLoginLogs(query.value)
    rows.value = result.list
    total.value = result.total
  } catch (error: unknown) {
    loadError.value =
      error instanceof Error && error.message !== '' ? error.message : t('loginLog.loadFailed')
  } finally {
    loading.value = false
  }
}
function search(): void {
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(account.value.trim() ? { loginAccount: account.value.trim() } : {}),
    ...(eventType.value ? { eventType: eventType.value } : {}),
    ...(success.value === '' ? {} : { isSuccess: success.value }),
    ...(timeRange.value.length === 0 ? {} : { from: timeRange.value[0], to: timeRange.value[1] }),
  }
  void load()
}
function reset(): void {
  account.value = ''
  eventType.value = ''
  success.value = ''
  timeRange.value = []
  query.value = { page: 1, pageSize: query.value.pageSize }
  void load()
}
function updatePagination(next: TablePaginationState): void {
  query.value = {
    ...query.value,
    page: next.pageSize === query.value.pageSize ? next.currentPage : 1,
    pageSize: next.pageSize,
  }
  void load()
}
function eventLabel(value: string): string {
  return value === 'login' ? t('loginLog.login') : value === 'logout' ? t('loginLog.logout') : value
}

onMounted(() => {
  void load()
})
</script>

<template>
  <section class="login-log-page management-page">
    <AppSearch
      v-model="searchModel"
      class="management-page__filters"
      :fields="searchFields"
      :query-label="t('loginLog.search')"
      :reset-label="t('loginLog.reset')"
      query-test-id="login-log-search"
      reset-test-id="login-log-reset"
      @query="search"
      @reset="reset"
    />
    <el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :aria-label="t('loginLog.title')"
      :refresh-label="t('loginLog.refresh')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #cell-event="{ row }: { row: LoginLogItem }"
        ><el-tag size="small" effect="plain">{{ eventLabel(row.eventType) }}</el-tag></template
      >
      <template #cell-status="{ row }: { row: LoginLogItem }"
        ><el-tag size="small" :type="row.isSuccess === 1 ? 'success' : 'danger'">{{
          row.isSuccess === 1 ? t('loginLog.successYes') : t('loginLog.successNo')
        }}</el-tag></template
      >
      <template #cell-createdAt="{ row }: { row: LoginLogItem }">{{
        formatTime(row.createdAt)
      }}</template>
      <template #empty><el-empty :description="t('loginLog.empty')" /></template>
    </AppTable>
  </section>
</template>

<style scoped>
.login-log-page {
  min-width: 0;
}
</style>
