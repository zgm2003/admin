<!-- 页面：我的钱包 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as redeemApi from '@/api/payment/redeemCode'
import { getMyTransactions, getTransactionOptions } from '@/api/payment/transaction'
import type {
  PersonalTransaction,
  PersonalTransactionQuery,
  TransactionOptions,
} from '@/api/payment/transaction'
import { getMyWallet } from '@/api/payment/wallet'
import type { MyWallet } from '@/api/payment/wallet'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import AppDialog from '@/components/AppDialog/index.vue'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'

const emit = defineEmits<{ recharge: [] }>()
const { t, locale } = useI18n()
const access = usePermissionStore()
const canDetail = computed(() => access.hasPermission('payment:myWallet:detail'))
const canList = computed(() => access.hasPermission('payment:myWallet:list'))
const canRecharge = computed(() => access.hasPermission('payment:recharge:view'))
const canRedeem = computed(() => access.hasPermission('payment:myWallet:redeem'))
const wallet = ref<MyWallet | null>(null)
const walletLoading = ref(false)
const walletError = ref('')
const rows = ref<PersonalTransaction[]>([])
const total = ref(0)
const query = ref<PersonalTransactionQuery>({ page: 1, pageSize: 20 })
const filters = ref<SearchFormModel<{ direction: number | ''; sourceType: number | '' }>>({
  direction: '',
  sourceType: '',
})
const {
  options,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<TransactionOptions | null>(getTransactionOptions, () => null)
const searchFields = computed<SearchField<{ direction: number | ''; sourceType: number | '' }>[]>(
  () => [
    {
      key: 'direction',
      type: 'select-v2',
      label: t('payment.transaction.direction'),
      options: options.value === null ? [] : options.value.directions,
      resetValue: '',
      clearable: true,
      disabled: options.value === null,
      width: 140,
    },
    {
      key: 'sourceType',
      type: 'select-v2',
      label: t('payment.transaction.sourceType'),
      options: options.value === null ? [] : options.value.sourceTypes,
      resetValue: '',
      clearable: true,
      disabled: options.value === null,
      width: 140,
    },
  ],
)
const loading = ref(false)
const loadError = ref('')
const redeemVisible = ref(false)
const redeemCode = ref('')
const redeemSubmitting = ref(false)
const redeemError = ref('')
const redeemResult = ref<redeemApi.RedeemResult | null>(null)
let sequence = 0
let walletSequence = 0
let alive = true

const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const columns = computed<TableColumn<PersonalTransaction>[]>(() => [
  { prop: 'transactionNo', label: t('payment.transaction.transactionNo'), minWidth: 190 },
  { key: 'sourceType', label: t('payment.transaction.sourceType'), width: 120 },
  { key: 'direction', label: t('payment.transaction.direction'), width: 100 },
  { prop: 'amount', label: t('payment.transaction.amount'), width: 130 },
  { prop: 'balanceBefore', label: t('payment.transaction.balanceBefore'), width: 140 },
  { prop: 'balanceAfter', label: t('payment.transaction.balanceAfter'), width: 140 },
  { key: 'createdAt', label: t('common.createdAt'), minWidth: 180 },
])
function transactionSource(value: number): string {
  return options.value?.sourceTypes.find((item) => item.value === value)?.label ?? String(value)
}
function transactionDirection(value: number): string {
  return options.value?.directions.find((item) => item.value === value)?.label ?? String(value)
}
function search(): void {
  query.value.page = 1
  void loadTransactions()
}
function reset(): void {
  filters.value = { direction: '', sourceType: '' }
  search()
}
function state(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}
async function loadWallet(): Promise<void> {
  if (!canDetail.value) return
  const current = ++walletSequence
  walletLoading.value = true
  walletError.value = ''
  try {
    const result = await getMyWallet()
    if (alive && current === walletSequence && canDetail.value) wallet.value = result
  } catch {
    if (alive && current === walletSequence) {
      wallet.value = null
      walletError.value = t('payment.wallet.loadFailed')
    }
  } finally {
    if (alive && current === walletSequence) walletLoading.value = false
  }
}
async function loadTransactions(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const currentQuery: PersonalTransactionQuery = {
      page: query.value.page,
      pageSize: query.value.pageSize,
    }
    if (filters.value.direction !== '') currentQuery.direction = filters.value.direction
    if (filters.value.sourceType !== '') currentQuery.sourceType = filters.value.sourceType
    const result = await getMyTransactions(currentQuery)
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
function openRedeem(): void {
  if (!canRedeem.value) return
  redeemCode.value = ''
  redeemError.value = ''
  redeemResult.value = null
  redeemVisible.value = true
}
async function redeem(): Promise<void> {
  if (!canRedeem.value || redeemSubmitting.value) return
  if (redeemCode.value.trim() === '') {
    if (redeemCode.value.trim() === '') redeemError.value = t('payment.redeemCode.codeRequired')
    return
  }
  redeemSubmitting.value = true
  redeemError.value = ''
  try {
    redeemResult.value = await redeemApi.redeemWalletCode(redeemCode.value.trim())
    redeemCode.value = ''
    await Promise.all([loadWallet(), loadTransactions()])
  } catch {
    // request.ts owns the API error notification.
  } finally {
    redeemSubmitting.value = false
  }
}
function updatePagination(value: TablePaginationState): void {
  query.value = {
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void loadTransactions()
}
watch(
  [canDetail, canList, locale],
  () => {
    sequence++
    walletSequence++
    wallet.value = null
    walletError.value = ''
    walletLoading.value = false
    loading.value = false
    rows.value = []
    total.value = 0
    if (canDetail.value) void loadWallet()
    if (canList.value) void loadTransactions()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  alive = false
  sequence++
  walletSequence++
})
</script>

<template>
  <AppPage class="payment-my-wallet-page">
    <section v-if="canDetail || canRecharge || canRedeem" class="payment-my-wallet-page__summary">
      <div v-if="canDetail" class="payment-my-wallet-page__funds">
        <span class="payment-my-wallet-page__label">{{ t('payment.wallet.myBalance') }}</span>
        <el-skeleton v-if="walletLoading" :rows="1" animated />
        <div v-else-if="walletError" role="status">
          <span>{{ walletError }}</span>
          <el-button link type="primary" @click="loadWallet">{{ t('common.refresh') }}</el-button>
        </div>
        <p v-else-if="wallet" class="payment-my-wallet-page__amount">
          <strong class="payment-my-wallet-page__balance">{{ wallet.balance }}</strong>
          <span class="payment-my-wallet-page__currency">{{ wallet.currency }}</span>
        </p>
        <p v-else class="payment-my-wallet-page__label">{{ t('payment.wallet.noWallet') }}</p>
      </div>
      <div class="payment-my-wallet-page__actions">
        <el-button
          v-if="canRecharge"
          type="primary"
          data-testid="payment-wallet-recharge"
          @click="emit('recharge')"
        >
          {{ t('navigation.paymentRecharge') }}
        </el-button>
        <el-button v-if="canRedeem" data-testid="payment-wallet-redeem" @click="openRedeem">
          {{ t('payment.redeemCode.redeem') }}
        </el-button>
      </div>
    </section>
    <el-alert
      v-if="optionsError"
      :title="t('payment.transaction.optionsFailed')"
      type="error"
      :closable="false"
      ><el-button link @click="reloadOptions">{{ t('common.refresh') }}</el-button></el-alert
    >
    <AppSearch
      v-if="canList"
      v-model="filters"
      :fields="searchFields"
      :query-label="t('common.search')"
      @query="search"
      @reset="reset"
    />
    <AppDialog v-model="redeemVisible" :title="t('payment.redeemCode.redeemTitle')" width="520px">
      <el-form label-position="top" @submit.prevent="redeem">
        <el-form-item :label="t('payment.redeemCode.code')">
          <el-input
            v-model="redeemCode"
            autocomplete="off"
            :placeholder="t('payment.redeemCode.codePlaceholder')"
            data-testid="payment-wallet-redeem-code"
            @keyup.enter="redeem"
          />
        </el-form-item>
        <el-alert v-if="redeemError" :title="redeemError" type="error" :closable="false" />
        <el-alert
          v-if="redeemResult"
          :title="t('payment.redeemCode.redeemSuccess')"
          type="success"
          :closable="false"
        >
          <p>{{ t('payment.redeemCode.redeemAmount', { amount: redeemResult.amount }) }}</p>
          <p>{{ t('payment.redeemCode.redeemBalance', { balance: redeemResult.balance }) }}</p>
        </el-alert>
      </el-form>
      <template #footer>
        <el-button :disabled="redeemSubmitting" @click="redeemVisible = false">{{
          t('common.close')
        }}</el-button>
        <el-button
          type="primary"
          :loading="redeemSubmitting"
          data-testid="payment-wallet-redeem-submit"
          @click="redeem"
          >{{ t('payment.redeemCode.redeem') }}</el-button
        >
      </template>
    </AppDialog>

    <AppTable
      v-if="canList"
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: PersonalTransaction) => String(row.id)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('payment.transaction.title')"
      @refresh="loadTransactions"
      @update:pagination="updatePagination"
    >
      <template #toolbar-left
        ><h2 class="payment-my-wallet-page__section-title">
          {{ t('payment.transaction.title') }}
        </h2></template
      >
      <template #cell-sourceType="{ row }: { row: PersonalTransaction }">
        <el-tag size="small" effect="plain" type="info">{{
          transactionSource(row.sourceType)
        }}</el-tag>
      </template>
      <template #cell-direction="{ row }: { row: PersonalTransaction }">{{
        transactionDirection(row.direction)
      }}</template>
      <template #cell-createdAt="{ row }: { row: PersonalTransaction }">{{
        formatTime(row.createdAt)
      }}</template>
      <template #empty><el-empty :description="t('payment.transaction.empty')" /></template>
    </AppTable>
  </AppPage>
</template>

<style scoped lang="scss">
.payment-my-wallet-page {
  min-width: 0;
  display: grid;
  gap: 16px;
}
.payment-my-wallet-page__summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 24px;
  padding: 24px;
  background: var(--el-fill-color-extra-light);
  border: 1px solid var(--el-border-color-light);
  border-radius: var(--el-border-radius-base);
}
.payment-my-wallet-page__funds {
  min-width: 0;
}
.payment-my-wallet-page__label,
.payment-my-wallet-page__currency {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.payment-my-wallet-page__amount {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 12px;
  margin: 8px 0 0;
}
.payment-my-wallet-page__balance {
  color: var(--el-text-color-primary);
  font-size: clamp(26px, 4vw, 36px);
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.payment-my-wallet-page__actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.payment-my-wallet-page__actions :deep(.el-button + .el-button) {
  margin-left: 0;
}
.payment-my-wallet-page__section-title {
  margin: 8px 0 0;
  font-size: 15px;
  font-weight: 600;
}
</style>
