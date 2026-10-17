<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as redeemApi from '@/api/payment/redeemCode'
import { getMyTransactions } from '@/api/payment/transaction'
import type { PersonalTransaction } from '@/api/payment/transaction'
import { getMyWallet } from '@/api/payment/wallet'
import type { MyWallet } from '@/api/payment/wallet'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import AppDialog from '@/components/AppDialog/index.vue'

const { t, locale } = useI18n()
const access = usePermissionStore()
const canDetail = computed(() => access.hasPermission('payment:myWallet:detail'))
const canList = computed(() => access.hasPermission('payment:myWallet:list'))
const canRedeem = computed(() => access.hasPermission('payment:myWallet:redeem'))
const wallet = ref<MyWallet | null>(null)
const walletLoading = ref(false)
const walletError = ref('')
const rows = ref<PersonalTransaction[]>([])
const total = ref(0)
const query = ref({ page: 1, pageSize: 20 })
const loading = ref(false)
const loadError = ref('')
const redeemVisible = ref(false)
const redeemCode = ref('')
const redeemSubmitting = ref(false)
const redeemError = ref('')
const redeemResult = ref<redeemApi.RedeemResult | null>(null)
let sequence = 0
let alive = true

const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const columns = computed<TableColumn<PersonalTransaction>[]>(() => [
  { prop: 'transactionNo', label: t('payment.transaction.transactionNo'), minWidth: 190 },
  { key: 'sourceType', label: t('payment.transaction.sourceType'), width: 120 },
  { prop: 'amount', label: t('payment.transaction.amount'), width: 130 },
  { prop: 'balanceBefore', label: t('payment.transaction.balanceBefore'), width: 140 },
  { prop: 'balanceAfter', label: t('payment.transaction.balanceAfter'), width: 140 },
  { key: 'createdAt', label: t('payment.transaction.createdAt'), minWidth: 180 },
])
function message(error: unknown, fallback: string): string {
  return error instanceof Error && error.message !== '' ? error.message : fallback
}
function transactionSource(value: number): string {
  if (value === 1) return t('payment.transaction.payment')
  if (value === 2) return t('payment.transaction.redeemCode')
  return String(value)
}
function state(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}
async function loadWallet(): Promise<void> {
  if (!canDetail.value) return
  walletLoading.value = true
  walletError.value = ''
  try {
    wallet.value = await getMyWallet()
  } catch (error: unknown) {
    wallet.value = null
    walletError.value = message(error, t('payment.wallet.loadFailed'))
  } finally {
    walletLoading.value = false
  }
}
async function loadTransactions(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await getMyTransactions(query.value)
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch (error: unknown) {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = message(error, t('payment.transaction.loadFailed'))
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
  if (redeemSubmitting.value || redeemCode.value.trim() === '') {
    if (redeemCode.value.trim() === '') redeemError.value = t('payment.redeemCode.codeRequired')
    return
  }
  redeemSubmitting.value = true
  redeemError.value = ''
  try {
    redeemResult.value = await redeemApi.redeemWalletCode(redeemCode.value.trim())
    redeemCode.value = ''
    await Promise.all([loadWallet(), loadTransactions()])
  } catch (error: unknown) {
    redeemError.value = message(error, t('payment.redeemCode.requestFailed'))
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
})
</script>

<template>
  <AppPage class="payment-my-wallet-page">
    <el-alert v-if="walletError" :title="walletError" type="error" :closable="false" show-icon />
    <el-card class="payment-my-wallet-page__summary" shadow="never">
      <template #header>
        <div class="payment-my-wallet-page__summary-header">
          <span>{{ t('payment.wallet.myTitle') }}</span>
          <el-button
            v-if="canRedeem"
            type="primary"
            data-testid="payment-wallet-redeem"
            @click="openRedeem"
          >
            {{ t('payment.redeemCode.redeem') }}
          </el-button>
        </div>
      </template>
      <el-skeleton v-if="walletLoading" :rows="2" animated />
      <el-empty v-else-if="wallet === null" :description="t('payment.wallet.noWallet')" />
      <el-descriptions v-else :column="2" border>
        <el-descriptions-item :label="t('payment.wallet.myBalance')">
          <strong class="payment-my-wallet-page__balance">{{ wallet.balance }}</strong>
        </el-descriptions-item>
        <el-descriptions-item :label="t('payment.wallet.myCurrency')">{{
          wallet.currency
        }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
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
          t('payment.redeemCode.close')
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
    <el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
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
      <template #cell-sourceType="{ row }: { row: PersonalTransaction }">
        <el-tag size="small" effect="plain" type="info">{{
          transactionSource(row.sourceType)
        }}</el-tag>
      </template>
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
  max-width: 720px;
}
.payment-my-wallet-page__summary-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.payment-my-wallet-page__balance {
  font-size: 24px;
  color: var(--el-color-primary);
}
</style>
