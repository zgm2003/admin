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
const selectedChoice = ref<number | 'custom' | undefined>(undefined)
const customAmount = ref<number | undefined>(undefined)
const selectedMethod = ref<number | undefined>(undefined)
const checkout = ref<Checkout | null>(null)
const checkoutVisible = ref(false)
let sequence = 0
let optionsSequence = 0
let alive = true
let pendingRequest: api.CreateRechargeInput | null = null

const selectedPackage = computed(() =>
  options.value?.packages.find((item) => item.id === selectedChoice.value),
)
const customAmountValid = computed(() => {
  const limits = options.value?.limits
  return (
    limits !== undefined &&
    typeof customAmount.value === 'number' &&
    Number.isFinite(customAmount.value) &&
    customAmount.value >= Number(limits.minimumAmount) &&
    customAmount.value <= Number(limits.maximumAmount)
  )
})
const amountError = computed(() =>
  selectedChoice.value === 'custom' && options.value && !customAmountValid.value
    ? t('payment.order.customAmountRule', {
        minimum: options.value.limits.minimumAmount,
        maximum: options.value.limits.maximumAmount,
      })
    : '',
)
const payableAmount = computed(() => {
  if (selectedChoice.value !== 'custom') return selectedPackage.value?.amount ?? '—'
  return customAmountValid.value && customAmount.value !== undefined && options.value
    ? customAmount.value.toFixed(options.value.limits.amountScale)
    : '—'
})
const canSubmit = computed(
  () =>
    canCreate.value &&
    !optionsLoading.value &&
    options.value?.available === true &&
    options.value.payMethods.some((item) => item.value === selectedMethod.value) &&
    (selectedChoice.value === 'custom'
      ? customAmountValid.value
      : selectedPackage.value !== undefined),
)
function selectChoice(value: number | 'custom'): void {
  if (!submitting.value && !optionsLoading.value) selectedChoice.value = value
}
function navigateChoice(event: KeyboardEvent): void {
  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key))
    return
  event.preventDefault()
  if (submitting.value || optionsLoading.value) return
  const choices: Array<number | 'custom'> = [
    ...(options.value?.packages.map((item) => item.id) ?? []),
    'custom',
  ]
  const current = choices.indexOf(selectedChoice.value ?? 'custom')
  const index =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? choices.length - 1
        : (current + (['ArrowLeft', 'ArrowUp'].includes(event.key) ? -1 : 1) + choices.length) %
          choices.length
  const choice = choices[index]
  if (choice === undefined) return
  selectChoice(choice)
  const target = event.currentTarget
  if (target instanceof HTMLElement)
    target.parentElement?.querySelectorAll<HTMLButtonElement>('[role="radio"]')[index]?.focus()
}
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
function newRequestID(): string {
  return crypto.randomUUID()
}
async function loadOptions(): Promise<void> {
  if (!canCreate.value) return
  const current = ++optionsSequence
  optionsLoading.value = true
  optionsError.value = ''
  try {
    const result = await api.getRechargeOptions()
    if (!alive || current !== optionsSequence || !canCreate.value) return
    options.value = result
    if (
      selectedChoice.value !== 'custom' &&
      !result.packages.some((item) => item.id === selectedChoice.value)
    ) {
      selectedChoice.value = result.packages[0]?.id ?? 'custom'
    }
    if (!result.payMethods.some((item) => item.value === selectedMethod.value)) {
      selectedMethod.value = result.payMethods[0]?.value
    }
    if (customAmount.value === undefined) customAmount.value = Number(result.limits.minimumAmount)
  } catch {
    if (alive && current === optionsSequence) {
      options.value = null
      optionsError.value = t('payment.order.optionsUnavailable')
    }
  } finally {
    if (alive && current === optionsSequence) optionsLoading.value = false
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
  if (!canSubmit.value || submitting.value || selectedMethod.value === undefined || !options.value)
    return
  const returnUrl = new URL(window.location.href)
  returnUrl.hash = ''
  const common = { payMethod: selectedMethod.value, returnUrl: returnUrl.href }
  const selection =
    selectedChoice.value === 'custom' && customAmount.value !== undefined
      ? { amount: customAmount.value.toFixed(options.value.limits.amountScale) }
      : selectedPackage.value
        ? { packageId: selectedPackage.value.id }
        : null
  if (selection === null) return
  const intent = { ...common, ...selection }
  const previousIntent = pendingRequest && {
    payMethod: pendingRequest.payMethod,
    returnUrl: pendingRequest.returnUrl,
    ...('amount' in pendingRequest
      ? { amount: pendingRequest.amount }
      : { packageId: pendingRequest.packageId }),
  }
  if (!pendingRequest || JSON.stringify(previousIntent) !== JSON.stringify(intent)) {
    pendingRequest = { requestId: newRequestID(), ...intent }
  }
  submitting.value = true
  try {
    const result = await api.createRecharge(pendingRequest)
    pendingRequest = null
    if (!alive || !canCreate.value) return
    checkout.value = result
    checkoutVisible.value = true
    await loadOrders()
  } catch {
    // request.ts owns API error notification; keep requestId for an uncertain retry.
  } finally {
    if (alive) submitting.value = false
  }
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
  [canCreate, locale],
  () => {
    optionsSequence++
    if (canCreate.value) void loadOptions()
    else {
      options.value = null
      optionsError.value = ''
      optionsLoading.value = false
      checkoutVisible.value = false
    }
  },
  { immediate: true },
)
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
  optionsSequence++
})
</script>

<template>
  <AppPage class="payment-recharge-page">
    <section v-if="canCreate" class="payment-recharge-page__form" :aria-busy="optionsLoading">
      <div v-if="optionsError" class="payment-recharge-page__error" role="status">
        <span>{{ optionsError }}</span>
        <el-button type="primary" plain @click="loadOptions">{{ t('common.refresh') }}</el-button>
      </div>
      <el-skeleton v-else-if="optionsLoading && !options" :rows="3" animated />
      <el-form v-else label-position="top" @submit.prevent="create">
        <h2 class="payment-recharge-page__section-title">{{ t('payment.order.chooseAmount') }}</h2>
        <p v-if="options?.packages.length === 0" class="payment-recharge-page__hint">
          {{ t('payment.order.noPackages') }}
        </p>
        <div
          class="payment-recharge-page__packages"
          role="radiogroup"
          :aria-label="t('payment.order.chooseAmount')"
        >
          <button
            v-for="item in options?.packages ?? []"
            :key="item.id"
            type="button"
            role="radio"
            class="payment-recharge-page__package"
            :class="{ 'is-selected': selectedChoice === item.id }"
            :aria-checked="selectedChoice === item.id"
            :tabindex="selectedChoice === item.id ? 0 : -1"
            :disabled="optionsLoading || submitting"
            :data-testid="`payment-recharge-package-${item.id}`"
            @click="selectChoice(item.id)"
            @keydown="navigateChoice"
          >
            <span class="payment-recharge-page__package-heading">
              <span class="payment-recharge-page__package-name">{{ item.name }}</span>
              <span v-if="item.badge" class="payment-recharge-page__badge">{{ item.badge }}</span>
            </span>
            <span class="payment-recharge-page__package-amount"
              ><small>¥</small>{{ item.amount }}</span
            >
            <span class="payment-recharge-page__selection" aria-hidden="true">{{
              selectedChoice === item.id ? '✓' : ''
            }}</span>
          </button>
          <button
            type="button"
            role="radio"
            class="payment-recharge-page__package payment-recharge-page__package--custom"
            :class="{ 'is-selected': selectedChoice === 'custom' }"
            :aria-checked="selectedChoice === 'custom'"
            :tabindex="selectedChoice === 'custom' ? 0 : -1"
            :disabled="!options || optionsLoading || submitting"
            data-testid="payment-recharge-custom"
            @click="selectChoice('custom')"
            @keydown="navigateChoice"
          >
            <span class="payment-recharge-page__package-name">{{
              t('payment.order.customAmount')
            }}</span>
            <span class="payment-recharge-page__hint">{{
              t('payment.order.customAmountHint')
            }}</span>
            <span class="payment-recharge-page__selection" aria-hidden="true">{{
              selectedChoice === 'custom' ? '✓' : ''
            }}</span>
          </button>
        </div>
        <el-form-item
          v-if="selectedChoice === 'custom' && options"
          class="payment-recharge-page__custom"
          :label="t('payment.order.customAmount')"
          :error="amountError"
        >
          <el-input-number
            v-model="customAmount"
            :min="Number(options.limits.minimumAmount)"
            :max="Number(options.limits.maximumAmount)"
            :precision="options.limits.amountScale"
            :step="10 ** -options.limits.amountScale"
            :disabled="optionsLoading || submitting"
            controls-position="right"
            data-testid="payment-recharge-amount"
          />
          <span class="payment-recharge-page__hint">{{
            t('payment.order.customMinimum', { minimum: options.limits.minimumAmount })
          }}</span>
        </el-form-item>
        <div class="payment-recharge-page__checkout">
          <el-form-item class="payment-recharge-page__method" :label="t('payment.order.payMethod')">
            <el-select-v2
              v-model="selectedMethod"
              :options="methodOptions"
              :placeholder="t('payment.order.payMethodPlaceholder')"
              :loading="optionsLoading"
              :disabled="!options?.available || optionsLoading || submitting"
              data-testid="payment-recharge-method"
            />
          </el-form-item>
          <div class="payment-recharge-page__pay">
            <div class="payment-recharge-page__total" data-testid="payment-recharge-total">
              <span>{{ t('payment.order.payableAmount') }}</span>
              <strong><small>¥</small>{{ payableAmount }}</strong>
            </div>
            <el-button
              type="primary"
              size="large"
              native-type="button"
              @click="create"
              :loading="submitting"
              :disabled="!canSubmit || submitting"
              data-testid="payment-recharge-create"
              >{{ t('payment.order.rechargeNow') }}</el-button
            >
          </div>
        </div>
        <el-alert
          v-if="options && !options.available"
          :title="t('payment.order.checkoutUnavailable')"
          type="warning"
          :closable="false"
          show-icon
        />
      </el-form>
    </section>
    <h2 v-if="canList" class="payment-recharge-page__section-title">
      {{ t('payment.order.personalTitle') }}
    </h2>
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
  min-width: 0;
  padding: 24px;
  border: 1px solid var(--el-border-color-light);
  border-radius: var(--el-border-radius-base);
  background: var(--el-bg-color);
}
.payment-recharge-page__section-title {
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 600;
}
.payment-recharge-page > .payment-recharge-page__section-title {
  margin: 8px 0 0;
}
.payment-recharge-page__packages {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
}
.payment-recharge-page__package {
  position: relative;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  gap: 16px;
  padding: 20px 20px 24px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  color: var(--el-text-color-primary);
  background: var(--el-bg-color);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.15s,
    background-color 0.15s;
  &:hover:not(:disabled) {
    border-color: var(--el-color-primary-light-3);
  }
  &:focus-visible {
    outline: 2px solid var(--el-color-primary);
    outline-offset: 3px;
  }
  &.is-selected {
    border-color: var(--el-color-primary);
    background: var(--el-color-primary-light-9);
  }
  &:disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }
}
.payment-recharge-page__package-heading {
  width: 100%;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}
.payment-recharge-page__package-name {
  font-weight: 600;
  overflow-wrap: anywhere;
}
.payment-recharge-page__badge {
  flex-shrink: 0;
  max-width: 50%;
  padding: 3px 7px;
  color: var(--el-color-warning-dark-2);
  background: var(--el-color-warning-light-9);
  border-radius: 4px;
  font-size: 11px;
  overflow-wrap: anywhere;
}
.payment-recharge-page__package-amount {
  font-size: 28px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.payment-recharge-page small {
  margin-right: 4px;
  font-size: 16px;
  font-weight: 400;
}
.payment-recharge-page__selection {
  position: absolute;
  bottom: 8px;
  right: 10px;
  color: var(--el-color-primary);
  font-weight: 700;
}
.payment-recharge-page__hint {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.5;
}
.payment-recharge-page__custom {
  margin-top: 24px;
}
.payment-recharge-page__custom :deep(.el-form-item__content) {
  gap: 12px;
}
.payment-recharge-page__custom :deep(.el-input-number) {
  width: 240px;
  max-width: 100%;
}
.payment-recharge-page__checkout {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 24px;
  margin: 24px 0 16px;
  padding-top: 20px;
  border-top: 1px solid var(--el-border-color-lighter);
}
.payment-recharge-page__method {
  width: 260px;
  max-width: 100%;
  margin: 0;
}
.payment-recharge-page__pay {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 24px;
}
.payment-recharge-page__total {
  display: flex;
  align-items: baseline;
  gap: 12px;
  color: var(--el-text-color-secondary);
}
.payment-recharge-page__total strong {
  color: var(--el-color-primary);
  font-size: 26px;
  font-variant-numeric: tabular-nums;
}
.payment-recharge-page__error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  color: var(--el-color-danger);
}
@media (max-width: 600px) {
  .payment-recharge-page__form {
    padding: 16px;
  }
  .payment-recharge-page__packages {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .payment-recharge-page__package {
    padding: 16px 12px 24px;
  }
  .payment-recharge-page__package-heading {
    flex-direction: column;
  }
  .payment-recharge-page__badge {
    max-width: 100%;
  }
  .payment-recharge-page__package-amount {
    font-size: 23px;
  }
  .payment-recharge-page__pay {
    width: 100%;
    justify-content: space-between;
    gap: 12px;
  }
  .payment-recharge-page__method {
    width: 100%;
  }
}
@media (prefers-reduced-motion: reduce) {
  .payment-recharge-page__package {
    transition: none;
  }
}
</style>
