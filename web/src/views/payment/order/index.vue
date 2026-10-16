<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as api from '@/api/payment/order'
import type { RechargeOrder, OrderQuery } from '@/api/payment/order'
import * as callbackApi from '@/api/payment/callbackEvent'
import type { CallbackEvent } from '@/api/payment/callbackEvent'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import AppDialog from '@/components/AppDialog/index.vue'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'

const { t, locale } = useI18n()
const access = usePermissionStore()
const canList = computed(() => access.hasPermission('payment:order:list'))
const canDetail = computed(() => access.hasPermission('payment:order:detail'))
const canCallbackList = computed(() => access.hasPermission('payment:callbackEvent:list'))
const canCallbackDetail = computed(() => access.hasPermission('payment:callbackEvent:detail'))
const filters = ref<SearchFormModel<{ userId: string; platformId: string }>>({
  userId: '',
  platformId: '',
})
const query = ref<OrderQuery>({ page: 1, pageSize: 20 })
const rows = ref<RechargeOrder[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')
const detail = ref<RechargeOrder | null>(null)
const detailVisible = ref(false)
const detailError = ref('')
const callbackRows = ref<CallbackEvent[]>([])
const callbackTotal = ref(0)
const callbackLoading = ref(false)
const callbackError = ref('')
const callbackQuery = ref({ page: 1, pageSize: 10 })
const callbackDetail = ref<CallbackEvent | null>(null)
const callbackDetailVisible = ref(false)
let sequence = 0
let alive = true

const searchFields = computed<SearchField<{ userId: string; platformId: string }>[]>(() => [
  {
    key: 'userId',
    type: 'input',
    label: t('payment.order.userId'),
    placeholder: t('payment.wallet.userIdPlaceholder'),
    resetValue: '',
    width: 180,
  },
  {
    key: 'platformId',
    type: 'input',
    label: t('payment.order.platformId'),
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
const callbackPagination = computed<TablePaginationState>(() => ({
  currentPage: callbackQuery.value.page,
  pageSize: callbackQuery.value.pageSize,
  total: callbackTotal.value,
}))
const columns = computed<TableColumn<RechargeOrder>[]>(() => [
  { prop: 'orderNo', label: t('payment.order.orderNo'), minWidth: 220 },
  { prop: 'packageName', label: t('payment.order.packageName'), minWidth: 170 },
  { prop: 'amount', label: t('payment.order.amount'), width: 120 },
  { key: 'status', label: t('payment.order.status'), width: 150 },
  { key: 'expiresAt', label: t('payment.order.expiresAt'), minWidth: 180 },
  { key: 'actions', label: t('payment.order.actions'), width: 100, fixed: 'right' },
])
const callbackColumns = computed<TableColumn<CallbackEvent>[]>(() => [
  { prop: 'id', label: t('payment.callback.id'), width: 90 },
  { prop: 'processStatus', label: t('payment.callback.processStatus'), width: 120 },
  { prop: 'errorCode', label: t('payment.callback.errorCode'), width: 100 },
  { key: 'tradeStatus', label: t('payment.callback.tradeStatus'), minWidth: 140 },
  { key: 'createdAt', label: t('payment.callback.createdAt'), minWidth: 180 },
])
function message(error: unknown, fallback: string): string {
  return error instanceof Error && error.message !== '' ? error.message : fallback
}
function positive(raw: string): number | undefined {
  const value = raw.trim()
  if (!/^\d+$/.test(value) || Number(value) < 1 || !Number.isSafeInteger(Number(value)))
    return undefined
  return Number(value)
}
function loadQuery(): OrderQuery {
  const next: OrderQuery = { page: query.value.page, pageSize: query.value.pageSize }
  const userId = positive(filters.value.userId)
  const platformId = positive(filters.value.platformId)
  if (userId !== undefined) next.userId = userId
  if (platformId !== undefined) next.platformId = platformId
  return next
}
function orderState(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}
function statusType(status: number): 'success' | 'warning' | 'info' | 'danger' {
  return status === 2 ? 'success' : status === 3 ? 'info' : 'warning'
}
async function load(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getOrders(loadQuery())
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch (error: unknown) {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = message(error, t('payment.order.loadFailed'))
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
async function openDetail(row: RechargeOrder): Promise<void> {
  if (!canDetail.value) return
  detailVisible.value = true
  detail.value = null
  detailError.value = ''
  callbackRows.value = []
  callbackTotal.value = 0
  try {
    detail.value = await api.getOrder(row.id)
    if (canCallbackList.value) await loadCallbacks(row.id)
  } catch (error: unknown) {
    detailError.value = message(error, t('payment.order.loadFailed'))
  }
}
async function loadCallbacks(orderId = detail.value?.id): Promise<void> {
  if (!canCallbackList.value || orderId === undefined) return
  callbackLoading.value = true
  callbackError.value = ''
  try {
    const result = await callbackApi.getOrderCallbackEvents(orderId, callbackQuery.value)
    callbackRows.value = result.list
    callbackTotal.value = result.total
  } catch (error: unknown) {
    callbackRows.value = []
    callbackTotal.value = 0
    callbackError.value = message(error, t('payment.callback.loadFailed'))
  } finally {
    callbackLoading.value = false
  }
}
function updateCallbackPagination(value: TablePaginationState): void {
  callbackQuery.value = {
    page: value.pageSize === callbackQuery.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void loadCallbacks()
}
async function openCallback(row: CallbackEvent): Promise<void> {
  if (!canCallbackDetail.value || detail.value === null) return
  try {
    callbackDetail.value = await callbackApi.getOrderCallbackEvent(detail.value.id, row.id)
    callbackDetailVisible.value = true
  } catch (error: unknown) {
    callbackError.value = message(error, t('payment.callback.loadFailed'))
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
  <AppPage class="payment-order-page">
    <AppSearch
      v-model="filters"
      class="management-page__filters"
      :fields="searchFields"
      :query-label="t('payment.order.search')"
      :reset-label="t('payment.order.reset')"
      @query="search"
      @reset="reset"
    />
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: RechargeOrder) => String(row.id)"
      :result-state="orderState()"
      :status-message="loadError"
      :aria-label="t('payment.order.title')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #cell-status="{ row }: { row: RechargeOrder }"
        ><el-tag size="small" :type="statusType(row.status)">{{
          row.statusLabel
        }}</el-tag></template
      >
      <template #cell-expiresAt="{ row }: { row: RechargeOrder }">{{
        formatTime(row.expiresAt)
      }}</template>
      <template #cell-actions="{ row }: { row: RechargeOrder }"
        ><el-button v-if="canDetail" link type="primary" @click="openDetail(row)">{{
          t('payment.order.view')
        }}</el-button></template
      >
      <template #empty><el-empty :description="t('payment.order.empty')" /></template>
    </AppTable>

    <AppDialog v-model="detailVisible" :title="t('payment.order.detail')" width="920px">
      <el-alert v-if="detailError" :title="detailError" type="error" :closable="false" />
      <template v-if="detail">
        <el-descriptions :column="3" border>
          <el-descriptions-item :label="t('payment.order.orderNo')">{{
            detail.orderNo
          }}</el-descriptions-item>
          <el-descriptions-item :label="t('payment.order.packageName')">{{
            detail.packageName
          }}</el-descriptions-item>
          <el-descriptions-item :label="t('payment.order.amount')">{{
            detail.amount
          }}</el-descriptions-item>
          <el-descriptions-item :label="t('payment.order.status')"
            ><el-tag size="small" :type="statusType(detail.status)">{{
              detail.statusLabel
            }}</el-tag></el-descriptions-item
          >
          <el-descriptions-item :label="t('payment.order.paidAt')">{{
            detail.paidAt ? formatTime(detail.paidAt) : '-'
          }}</el-descriptions-item>
          <el-descriptions-item :label="t('payment.order.creditedAt')">{{
            detail.creditedAt ? formatTime(detail.creditedAt) : '-'
          }}</el-descriptions-item>
        </el-descriptions>
        <div v-if="canCallbackList" class="payment-order-page__callbacks">
          <el-divider content-position="left">{{ t('payment.order.callbackEvents') }}</el-divider>
          <el-alert v-if="callbackError" :title="callbackError" type="error" :closable="false" />
          <AppTable
            :columns="callbackColumns"
            :data="callbackRows"
            :loading="callbackLoading"
            :pagination="callbackPagination"
            :row-key="(row: CallbackEvent) => String(row.id)"
            :aria-label="t('payment.callback.title')"
            @refresh="loadCallbacks"
            @update:pagination="updateCallbackPagination"
          >
            <template #cell-tradeStatus="{ row }: { row: CallbackEvent }">{{
              row.payload.tradeStatus
            }}</template>
            <template #cell-createdAt="{ row }: { row: CallbackEvent }">{{
              formatTime(row.createdAt)
            }}</template>
            <template #cell-id="{ row }: { row: CallbackEvent }"
              ><el-button v-if="canCallbackDetail" link type="primary" @click="openCallback(row)">{{
                row.id
              }}</el-button
              ><span v-else>{{ row.id }}</span></template
            >
            <template #empty><el-empty :description="t('payment.callback.empty')" /></template>
          </AppTable>
        </div>
      </template>
    </AppDialog>
    <AppDialog v-model="callbackDetailVisible" :title="t('payment.callback.title')" width="720px">
      <el-descriptions v-if="callbackDetail" :column="2" border>
        <el-descriptions-item :label="t('payment.callback.id')">{{
          callbackDetail.id
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.callback.processStatus')">{{
          callbackDetail.processStatus
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.callback.errorCode')">{{
          callbackDetail.errorCode ?? '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.callback.tradeStatus')">{{
          callbackDetail.payload.tradeStatus
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.callback.tradeNo')">{{
          callbackDetail.payload.tradeNo ?? '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.callback.totalAmount')">{{
          callbackDetail.payload.totalAmount
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.callback.createdAt')">{{
          formatTime(callbackDetail.createdAt)
        }}</el-descriptions-item>
      </el-descriptions>
    </AppDialog>
  </AppPage>
</template>

<style scoped lang="scss">
.payment-order-page {
  min-width: 0;
}
.payment-order-page__callbacks {
  margin-top: 16px;
}
</style>
