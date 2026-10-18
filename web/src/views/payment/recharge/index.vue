<!-- 页面：充值中心 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as api from '@/api/payment/order'
import type { Checkout, RechargeOptions, RechargeOrder } from '@/api/payment/order'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import AppDialog from '@/components/AppDialog/index.vue'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'

const { t, locale } = useI18n()
const access = usePermissionStore()
const canCreate = computed(() => access.hasPermission('payment:recharge:create'))
const canList = computed(() => access.hasPermission('payment:recharge:list'))
const canPay = computed(() => access.hasPermission('payment:recharge:pay'))
const options = ref<RechargeOptions | null>(null)
const optionsLoading = ref(false)
const optionsError = ref('')
const orders = ref<RechargeOrder[]>([])
const total = ref(0)
const query = ref({ page: 1, pageSize: 20 })
const loading = ref(false)
const loadError = ref('')
const submitting = ref(false)
const selectedPackage = ref<number | undefined>(undefined)
const selectedMethod = ref<number | undefined>(undefined)
const checkout = ref<Checkout | null>(null)
const checkoutVisible = ref(false)
let sequence = 0
let alive = true

const packageOptions = computed(() =>
  (options.value?.packages ?? []).map((item) => ({
    value: item.id,
    label: `${item.name} · ¥${item.amount}`,
  })),
)
const methodOptions = computed(() => options.value?.payMethods ?? [])
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
  { key: 'actions', label: t('payment.order.actions'), width: 130, fixed: 'right' },
])
function statusType(status: number): 'success' | 'warning' | 'info' {
  return status === 2 ? 'success' : status === 3 ? 'info' : 'warning'
}
function orderState(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return orders.value.length > 0 ? 'success' : 'empty'
}
function newRequestID(): string {
  return crypto.randomUUID()
}
async function loadOptions(): Promise<void> {
  if (!canCreate.value) return
  optionsLoading.value = true
  optionsError.value = ''
  try {
    options.value = await api.getRechargeOptions()
    const firstPackage = options.value.packages[0]
    const firstMethod = options.value.payMethods[0]
    selectedPackage.value = firstPackage?.id
    selectedMethod.value = firstMethod?.value
  } catch {
    options.value = null
    optionsError.value = t('payment.order.optionsUnavailable')
  } finally {
    optionsLoading.value = false
  }
}
async function loadOrders(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getMyRechargeOrders(query.value)
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
function updatePagination(value: TablePaginationState): void {
  query.value = {
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void loadOrders()
}
async function create(): Promise<void> {
  if (
    !canCreate.value ||
    submitting.value ||
    !options.value?.available ||
    selectedPackage.value === undefined ||
    selectedMethod.value === undefined
  )
    return
  submitting.value = true
  try {
    const returnUrl = new URL(window.location.href)
    returnUrl.hash = ''
    const result = await api.createRecharge({
      requestId: newRequestID(),
      packageId: selectedPackage.value,
      payMethod: selectedMethod.value,
      returnUrl: returnUrl.href,
    })
    checkout.value = result
    checkoutVisible.value = true
    await loadOrders()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    submitting.value = false
  }
}
async function pay(row: RechargeOrder): Promise<void> {
  if (!canPay.value || submitting.value || !row.actions.pay) return
  submitting.value = true
  try {
    checkout.value = await api.payRecharge(row.id)
    checkoutVisible.value = true
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
  [canCreate, locale],
  () => {
    if (canCreate.value) void loadOptions()
  },
  { immediate: true },
)
watch(
  [canList, locale],
  () => {
    sequence++
    orders.value = []
    total.value = 0
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
  <AppPage class="payment-recharge-page">
    <el-card v-if="canCreate" class="payment-recharge-page__form" shadow="never">
      <template #header
        ><span>{{ t('navigation.paymentRecharge') }}</span></template
      >
      <div v-if="optionsError"><el-button type="primary" plain @click="loadOptions">{{ t('appTable.refresh') }}</el-button></div>
      <el-alert
        v-if="options && !options.available"
        :title="t('payment.order.checkoutUnavailable')"
        type="warning"
        :closable="false"
        show-icon
      />
      <el-form label-position="top" @submit.prevent="create">
        <div class="payment-recharge-page__grid">
          <el-form-item :label="t('payment.order.package')">
            <el-select-v2
              v-model="selectedPackage"
              :options="packageOptions"
              :placeholder="t('payment.order.packagePlaceholder')"
              :loading="optionsLoading"
              :disabled="!options?.available || optionsLoading"
              data-testid="payment-recharge-package"
            />
          </el-form-item>
          <el-form-item :label="t('payment.order.payMethod')">
            <el-select-v2
              v-model="selectedMethod"
              :options="methodOptions"
              :placeholder="t('payment.order.payMethodPlaceholder')"
              :loading="optionsLoading"
              :disabled="!options?.available || optionsLoading"
              data-testid="payment-recharge-method"
            />
          </el-form-item>
        </div>
        
        <el-button
          type="primary"
          :loading="submitting"
          :disabled="!options?.available"
          data-testid="payment-recharge-create"
          @click="create"
          >{{ t('payment.order.create') }}</el-button
        >
      </el-form>
    </el-card>
    
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
      <template #cell-status="{ row }: { row: RechargeOrder }"
        ><el-tag size="small" :type="statusType(row.status)">{{
          row.statusLabel
        }}</el-tag></template
      >
      <template #cell-expiresAt="{ row }: { row: RechargeOrder }">{{
        formatTime(row.expiresAt)
      }}</template>
      <template #cell-actions="{ row }: { row: RechargeOrder }"
        ><el-button
          v-if="canPay && row.actions.pay"
          link
          type="primary"
          :disabled="submitting"
          @click="pay(row)"
          >{{ t('payment.order.pay') }}</el-button
        ></template
      >
      <template #empty><el-empty :description="t('payment.order.empty')" /></template>
    </AppTable>
    <AppDialog v-model="checkoutVisible" :title="t('payment.order.checkoutTitle')" width="520px">
      <el-descriptions v-if="checkout" :column="1" border>
        <el-descriptions-item :label="t('payment.order.orderNo')">{{
          checkout.order.orderNo
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.order.amount')">{{
          checkout.order.amount
        }}</el-descriptions-item>
      </el-descriptions>
      <el-alert
        v-if="checkout && !checkout.checkout"
        :title="t('payment.order.checkoutUnavailable')"
        type="warning"
        :closable="false"
      />
      <template #footer
        ><el-button type="primary" :disabled="!checkout?.checkout" @click="openCheckout">{{
          t('payment.order.openCheckout')
        }}</el-button></template
      >
    </AppDialog>
  </AppPage>
</template>

<style scoped lang="scss">
.payment-recharge-page {
  min-width: 0;
  display: grid;
  gap: 16px;
}
.payment-recharge-page__form {
  max-width: 900px;
}
.payment-recharge-page__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
@media (max-width: 680px) {
  .payment-recharge-page__grid {
    grid-template-columns: 1fr;
  }
}
</style>
