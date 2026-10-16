<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as api from '@/api/payment/wallet'
import type { Wallet } from '@/api/payment/wallet'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import AppDialog from '@/components/AppDialog/index.vue'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'

const { t, locale } = useI18n()
const access = usePermissionStore()
const canList = computed(() => access.hasPermission('payment:wallet:list'))
const canDetail = computed(() => access.hasPermission('payment:wallet:detail'))
const rows = ref<Wallet[]>([])
const total = ref(0)
const query = ref({ page: 1, pageSize: 20 })
const loading = ref(false)
const loadError = ref('')
const detail = ref<Wallet | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const detailVisible = ref(false)
let sequence = 0
let alive = true

const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const columns = computed<TableColumn<Wallet>[]>(() => [
  { prop: 'userId', label: t('payment.wallet.userId'), width: 110 },
  { prop: 'username', label: t('payment.wallet.username'), minWidth: 160, overflowTooltip: true },
  { prop: 'currency', label: t('payment.wallet.currency'), width: 90 },
  { prop: 'balance', label: t('payment.wallet.balance'), width: 140 },
  { key: 'createdAt', label: t('payment.wallet.createdAt'), minWidth: 180 },
  { key: 'updatedAt', label: t('payment.wallet.updatedAt'), minWidth: 180 },
  { key: 'actions', label: t('payment.order.actions'), width: 100, fixed: 'right' },
])

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message !== ''
    ? error.message
    : t('payment.wallet.loadFailed')
}
function state(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}
async function load(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getWallets(query.value)
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch (error: unknown) {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = errorMessage(error)
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
  void load()
}
async function openDetail(row: Wallet): Promise<void> {
  if (!canDetail.value) return
  detailVisible.value = true
  detailLoading.value = true
  detailError.value = ''
  detail.value = null
  try {
    detail.value = await api.getWallet(row.userId)
  } catch (error: unknown) {
    detailError.value = errorMessage(error)
  } finally {
    detailLoading.value = false
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
  <AppPage class="payment-wallet-page">
    <el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: Wallet) => String(row.userId)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('payment.wallet.title')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #cell-createdAt="{ row }: { row: Wallet }">{{
        formatTime(row.createdAt)
      }}</template>
      <template #cell-updatedAt="{ row }: { row: Wallet }">{{
        formatTime(row.updatedAt)
      }}</template>
      <template #cell-actions="{ row }: { row: Wallet }">
        <el-button v-if="canDetail" link type="primary" @click="openDetail(row)">{{
          t('payment.wallet.detail')
        }}</el-button>
      </template>
      <template #empty><el-empty :description="t('payment.wallet.empty')" /></template>
    </AppTable>

    <AppDialog v-model="detailVisible" :title="t('payment.wallet.detail')" width="520px">
      <el-skeleton v-if="detailLoading" :rows="4" animated />
      <el-alert v-else-if="detailError" :title="detailError" type="error" :closable="false" />
      <el-descriptions v-else-if="detail" :column="1" border>
        <el-descriptions-item :label="t('payment.wallet.userId')">{{
          detail.userId
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.wallet.username')">{{
          detail.username || '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.wallet.currency')">{{
          detail.currency
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.wallet.balance')">{{
          detail.balance
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.wallet.createdAt')">{{
          formatTime(detail.createdAt)
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.wallet.updatedAt')">{{
          formatTime(detail.updatedAt)
        }}</el-descriptions-item>
      </el-descriptions>
    </AppDialog>
  </AppPage>
</template>

<style scoped lang="scss">
.payment-wallet-page {
  min-width: 0;
}
</style>
