<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { getLoginLogs } from '@/api/user/loginLog'
import type {
  LoginLogEventType,
  LoginLogItem,
  LoginLogListQuery,
  LoginLogType,
} from '@/api/user/loginLog'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { formatTime } from '@/utils/datetime'

const { t } = useI18n()
const rows = ref<LoginLogItem[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')
const query = ref<LoginLogListQuery>({ page: 1, pageSize: 20 })
const account = ref('')
const eventType = ref<LoginLogEventType | ''>('')
const loginType = ref<LoginLogType | ''>('')
const success = ref<'' | 0 | 1>('')
const timeRange = ref<[] | [string, string]>([])
interface LoginLogSearchModel {
  account: string
  eventType: LoginLogEventType | ''
  loginType: LoginLogType | ''
  success: '' | 0 | 1
  timeRange: [] | [string, string]
}

const searchModel = computed<SearchFormModel<LoginLogSearchModel>>({
  get: () => ({
    account: account.value,
    eventType: eventType.value,
    loginType: loginType.value,
    success: success.value,
    timeRange: timeRange.value,
  }),
  set: (value) => {
    account.value = typeof value.account === 'string' ? value.account : ''
    eventType.value =
      value.eventType === 1 || value.eventType === 2 || value.eventType === 3 ? value.eventType : ''
    loginType.value =
      value.loginType === 1 || value.loginType === 2 || value.loginType === 3 ? value.loginType : ''
    success.value = value.success === 0 || value.success === 1 ? value.success : ''
    timeRange.value =
      Array.isArray(value.timeRange) && value.timeRange.length === 2
        ? [String(value.timeRange[0]), String(value.timeRange[1])]
        : []
  },
})
const searchFields = computed<SearchField<LoginLogSearchModel>[]>(() => [
  {
    key: 'account',
    type: 'input',
    resetValue: '',
    label: t('loginLog.account'),
    placeholder: t('loginLog.account'),
    width: 220,
    testId: 'login-log-account',
  },
  {
    key: 'eventType',
    type: 'select-v2',
    resetValue: '',
    label: t('loginLog.eventType'),
    placeholder: t('loginLog.allEventTypes'),
    options: [
      { label: t('loginLog.register'), value: 1 },
      { label: t('loginLog.login'), value: 2 },
      { label: t('loginLog.logout'), value: 3 },
    ],
    width: 140,
  },
  {
    key: 'loginType',
    type: 'select-v2',
    resetValue: '',
    label: t('loginLog.loginType'),
    placeholder: t('loginLog.allLoginTypes'),
    options: [
      { label: t('loginLog.password'), value: 1 },
      { label: t('loginLog.email'), value: 2 },
      { label: t('loginLog.phone'), value: 3 },
    ],
    width: 150,
  },
  {
    key: 'success',
    type: 'select-v2',
    resetValue: '',
    label: t('loginLog.success'),
    placeholder: t('loginLog.allResults'),
    options: [
      { label: t('loginLog.successYes'), value: 1 },
      { label: t('loginLog.successNo'), value: 0 },
    ],
    width: 140,
  },
  {
    key: 'timeRange',
    type: 'date-range',
    resetValue: [],
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
  { prop: 'account', label: t('loginLog.account'), minWidth: 190, overflowTooltip: true },
  { prop: 'platform', label: t('loginLog.platform'), width: 120 },
  { key: 'event', prop: 'id', label: t('loginLog.eventType'), width: 110 },
  { key: 'loginType', prop: 'id', label: t('loginLog.loginType'), width: 120 },
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
    ...(account.value.trim() ? { account: account.value.trim() } : {}),
    ...(eventType.value ? { eventType: eventType.value } : {}),
    ...(loginType.value ? { loginType: loginType.value } : {}),
    ...(success.value === '' ? {} : { isSuccess: success.value }),
    ...(timeRange.value.length === 0 ? {} : { from: timeRange.value[0], to: timeRange.value[1] }),
  }
  void load()
}
function reset(): void {
  account.value = ''
  eventType.value = ''
  loginType.value = ''
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
function eventLabel(value: LoginLogEventType): string {
  return value === 1
    ? t('loginLog.register')
    : value === 2
      ? t('loginLog.login')
      : t('loginLog.logout')
}
function eventTagType(value: LoginLogEventType): 'primary' | 'success' | 'danger' {
  return value === 1 ? 'primary' : value === 2 ? 'success' : 'danger'
}
function loginTypeLabel(value: LoginLogType | null): string {
  if (value === null) return '-'
  return value === 1
    ? t('loginLog.password')
    : value === 2
      ? t('loginLog.email')
      : t('loginLog.phone')
}

onMounted(() => {
  void load()
})
</script>

<template>
  <AppPage class="login-log-page">
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
        ><el-tag size="small" effect="plain" :type="eventTagType(row.eventType)">{{
          eventLabel(row.eventType)
        }}</el-tag></template
      >
      <template #cell-loginType="{ row }: { row: LoginLogItem }">{{
        loginTypeLabel(row.loginType)
      }}</template>
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
  </AppPage>
</template>

<style scoped>
.login-log-page {
  min-width: 0;
}
</style>
