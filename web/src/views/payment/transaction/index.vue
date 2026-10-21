<!-- 页面：资金流水 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as api from '@/api/payment/transaction'
import type { Transaction, TransactionQuery } from '@/api/payment/transaction'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import AppDialog from '@/components/AppDialog/index.vue'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'

const { t, locale } = useI18n()
const access = usePermissionStore()
const canList = computed(() => access.hasPermission('payment:transaction:list'))
const canDetail = computed(() => access.hasPermission('payment:transaction:detail'))
const filters = ref<SearchFormModel<{ userId: string; platformId: string }>>({
  userId: '',
  platformId: '',
})
const query = ref<TransactionQuery>({ page: 1, pageSize: 20 })
const rows = ref<Transaction[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')
const detail = ref<Transaction | null>(null)
const detailVisible = ref(false)
let sequence = 0
let alive = true

const searchFields = computed<SearchField<{ userId: string; platformId: string }>[]>(() => [
  {
    key: 'userId',
    type: 'input',
    label: t('payment.transaction.userId'),
    placeholder: t('payment.wallet.userIdPlaceholder'),
    resetValue: '',
    width: 180,
  },
  {
    key: 'platformId',
    type: 'input',
    label: t('payment.transaction.platformId'),
    placeholder: t('payment.order.platformIdPlaceholder'),
    resetValue: '',
    width: 180,
  },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const columns = computed<TableColumn<Transaction>[]>(() => [
  { prop: 'transactionNo', label: t('payment.transaction.transactionNo'), minWidth: 190 },
  { prop: 'userId', label: t('payment.transaction.userId'), width: 100 },
  { prop: 'username', label: t('payment.transaction.username'), minWidth: 140 },
  { prop: 'platformCode', label: t('payment.transaction.platformCode'), width: 130 },
  { key: 'sourceType', label: t('payment.transaction.sourceType'), width: 120 },
  { prop: 'amount', label: t('payment.transaction.amount'), width: 130 },
  { prop: 'balanceAfter', label: t('payment.transaction.balanceAfter'), width: 140 },
  { key: 'createdAt', label: t('common.createdAt'), minWidth: 180 },
  { key: 'actions', label: t('common.actions'), width: 90, fixed: 'right' },
])
function source(value: number): string {
  if (value === 1) return t('payment.transaction.payment')
  if (value === 2) return t('payment.transaction.redeemCode')
  return String(value)
}
function state(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}
function positive(raw: string): number | undefined {
  const value = raw.trim()
  if (!/^\d+$/.test(value) || Number(value) < 1 || !Number.isSafeInteger(Number(value)))
    return undefined
  return Number(value)
}
function loadQuery(): TransactionQuery {
  const next: TransactionQuery = { page: query.value.page, pageSize: query.value.pageSize }
  const userId = positive(filters.value.userId)
  const platformId = positive(filters.value.platformId)
  if (userId !== undefined) next.userId = userId
  if (platformId !== undefined) next.platformId = platformId
  return next
}
async function load(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getTransactions(loadQuery())
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = t('payment.transaction.loadFailed')
    }
  } finally {
    if (alive && current === sequence) loading.value = false
  }
}
function search(): void {
  query.value = { page: 1, pageSize: query.value.pageSize }
  void load()
}
function reset(): void {
  filters.value = { userId: '', platformId: '' }
  search()
}
function updatePagination(value: TablePaginationState): void {
  query.value = {
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}
async function openDetail(row: Transaction): Promise<void> {
  if (!canDetail.value) return
  detailVisible.value = true
  detail.value = null
  try {
    detail.value = await api.getTransaction(row.id)
  } catch {
    detailVisible.value = false
  }
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
  <AppPage class="payment-transaction-page">
    <AppSearch
      v-model="filters"
      class="management-page__filters"
      :fields="searchFields"
      :query-label="t('payment.transaction.title')"
      @query="search"
      @reset="reset"
    />
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: Transaction) => String(row.id)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('payment.transaction.title')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #cell-sourceType="{ row }: { row: Transaction }"
        ><el-tag size="small" effect="plain" type="info">{{
          source(row.sourceType)
        }}</el-tag></template
      >
      <template #cell-createdAt="{ row }: { row: Transaction }">{{
        formatTime(row.createdAt)
      }}</template>
      <template #cell-actions="{ row }: { row: Transaction }"
        ><el-button v-if="canDetail" link type="primary" @click="openDetail(row)">{{
          t('payment.order.view')
        }}</el-button></template
      >
      <template #empty><el-empty :description="t('payment.transaction.empty')" /></template>
    </AppTable>
    <AppDialog v-model="detailVisible" :title="t('payment.transaction.title')" width="620px">
      
      <el-descriptions v-if="detail" :column="2" border>
        <el-descriptions-item :label="t('payment.transaction.transactionNo')">{{
          detail.transactionNo
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.transaction.userId')">{{
          detail.userId
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.transaction.username')">{{
          detail.username || '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.transaction.platformCode')">{{
          detail.platformCode || '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.transaction.amount')">{{
          detail.amount
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.transaction.sourceType')">{{
          source(detail.sourceType)
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.transaction.balanceBefore')">{{
          detail.balanceBefore
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.transaction.balanceAfter')">{{
          detail.balanceAfter
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.transaction.orderId')">{{
          detail.orderId ?? '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('common.createdAt')">{{
          formatTime(detail.createdAt)
        }}</el-descriptions-item>
      </el-descriptions>
    </AppDialog>
  </AppPage>
</template>

<style scoped lang="scss">
.payment-transaction-page {
  min-width: 0;
}
</style>
