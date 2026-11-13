<!-- 页面：我的充值订单 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as api from '@/api/payment/order'
import type { Checkout, RechargeOrder, RechargeOrderQuery } from '@/api/payment/order'
import type { SearchField, SearchFormModel, SearchDateRange } from '@/components/AppSearch'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import AppDialog from '@/components/AppDialog/index.vue'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'

interface OrderFilters {
  keyword: string
  status: number
  timeRange: SearchDateRange
}

const { t, locale } = useI18n()
const access = usePermissionStore()
const canList = computed(() => access.hasPermission('payment:recharge:list'))
const canPay = computed(() => access.hasPermission('payment:recharge:pay'))
const filters = ref<SearchFormModel<OrderFilters>>({
  keyword: '',
  status: 0,
  timeRange: [],
})
const query = ref({ page: 1, pageSize: 20 })
const orders = ref<RechargeOrder[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')
const submitting = ref(false)
const checkout = ref<Checkout | null>(null)
const checkoutVisible = ref(false)
let sequence = 0
let alive = true

const searchFields = computed<SearchField<OrderFilters>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('payment.order.orderNo'),
    placeholder: t('payment.order.orderNoPlaceholder'),
    resetValue: '',
    width: 220,
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('payment.order.status'),
    placeholder: t('common.allStatuses'),
    resetValue: 0,
    width: 150,
    options: [
      { label: t('common.allStatuses'), value: 0 },
      { label: t('payment.order.statusPending'), value: 1 },
      { label: t('payment.order.statusPaid'), value: 2 },
      { label: t('payment.order.statusClosed'), value: 3 },
    ],
  },
  {
    key: 'timeRange',
    type: 'date-range',
    label: t('payment.order.timeRange'),
    resetValue: [],
    width: 360,
    valueFormat: 'YYYY-MM-DDTHH:mm:ssZ',
    startPlaceholder: t('common.startTime'),
    endPlaceholder: t('common.endTime'),
  },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const columns = computed<TableColumn<RechargeOrder>[]>(() => [
  { prop: 'orderNo', label: t('payment.order.orderNo'), minWidth: 220 },
  { prop: 'packageName', label: t('payment.order.packageName'), minWidth: 170 },
  { prop: 'amount', label: t('payment.order.amount'), width: 120 },
  { key: 'status', label: t('payment.order.status'), width: 150 },
  { key: 'expiresAt', label: t('payment.order.expiresAt'), minWidth: 180 },
  { key: 'actions', label: t('common.actions'), width: 130, fixed: 'right' },
])

function statusType(status: number): 'success' | 'warning' | 'info' {
  return status === 2 ? 'success' : status === 3 ? 'info' : 'warning'
}
function orderState(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return orders.value.length > 0 ? 'success' : 'empty'
}
function loadQuery(): RechargeOrderQuery {
  const next: RechargeOrderQuery = { page: query.value.page, pageSize: query.value.pageSize }
  const keyword = filters.value.keyword.trim()
  if (keyword !== '') next.keyword = keyword
  if (filters.value.status > 0) next.status = filters.value.status
  const [from, to] = filters.value.timeRange
  if (from !== undefined && to !== undefined) {
    next.from = from
    next.to = to
  }
  return next
}
async function loadOrders(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getMyRechargeOrders(loadQuery())
    if (alive && current === sequence && canList.value) {
      orders.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && current === sequence) {
      orders.value = []
      total.value = 0
      loadError.value = t('payment.order.loadFailed')
    }
  } finally {
    if (alive && current === sequence) loading.value = false
  }
}
function search(): void {
  query.value = { page: 1, pageSize: query.value.pageSize }
  void loadOrders()
}
function reset(): void {
  filters.value = { keyword: '', status: 0, timeRange: [] }
  search()
}
function updatePagination(value: TablePaginationState): void {
  query.value = {
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void loadOrders()
}
async function pay(row: RechargeOrder): Promise<void> {
  if (!canPay.value || submitting.value || !row.actions.pay) return
  submitting.value = true
  try {
    const result = await api.payRecharge(row.id)
    if (alive && canPay.value) {
      checkout.value = result
      checkoutVisible.value = true
    }
  } catch {
    // request.ts owns the API error notification.
  } finally {
    submitting.value = false
  }
}
function openCheckout(): void {
  if (checkout.value?.checkout)
    window.open(checkout.value.checkout, '_blank', 'noopener,noreferrer')
}
watch(
  [canList, locale],
  () => {
    sequence++
    orders.value = []
    total.value = 0
    loading.value = false
    if (canList.value) void loadOrders()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  alive = false
  sequence++
})
</script>

<template>
  <section class="payment-recharge-orders">
    <div class="payment-recharge-orders__heading">
      <h2>{{ t('payment.order.personalTitle') }}</h2>
    </div>
    <AppSearch
      v-model="filters"
      :fields="searchFields"
      :query-label="t('common.search')"
      @query="search"
      @reset="reset"
    />
    <AppTable
      v-if="canList"
      :columns="columns"
      :data="orders"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: RechargeOrder) => String(row.id)"
      :result-state="orderState()"
      :status-message="loadError"
      :aria-label="t('payment.order.personalTitle')"
      @refresh="loadOrders"
      @update:pagination="updatePagination"
    >
      <template #cell-status="{ row }: { row: RechargeOrder }">
        <el-tag size="small" :type="statusType(row.status)">{{ row.statusLabel }}</el-tag>
      </template>
      <template #cell-expiresAt="{ row }: { row: RechargeOrder }">
        {{ formatTime(row.expiresAt) }}
      </template>
      <template #cell-actions="{ row }: { row: RechargeOrder }">
        <el-button
          v-if="canPay && row.actions.pay"
          link
          type="primary"
          :disabled="submitting"
          @click="pay(row)"
        >
          {{ t('payment.order.pay') }}
        </el-button>
      </template>
      <template #empty><el-empty :description="t('payment.order.empty')" /></template>
    </AppTable>
    <AppDialog v-model="checkoutVisible" :title="t('payment.order.checkoutTitle')" width="520px">
      <el-descriptions v-if="checkout" :column="1" border>
        <el-descriptions-item :label="t('payment.order.orderNo')">
          {{ checkout.order.orderNo }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('payment.order.amount')">
          {{ checkout.order.amount }}
        </el-descriptions-item>
      </el-descriptions>
      <el-alert
        v-if="checkout && !checkout.checkout"
        :title="t('payment.order.checkoutUnavailable')"
        type="warning"
        :closable="false"
      />
      <template #footer>
        <el-button type="primary" :disabled="!checkout?.checkout" @click="openCheckout">
          {{ t('payment.order.openCheckout') }}
        </el-button>
      </template>
    </AppDialog>
  </section>
</template>

<style scoped lang="scss">
.payment-recharge-orders {
  display: grid;
  min-width: 0;
  gap: 16px;
}

.payment-recharge-orders__heading h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
</style>
