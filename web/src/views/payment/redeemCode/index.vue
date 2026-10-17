<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CirclePlus, Download, Search } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'

import * as api from '@/api/payment/redeemCode'
import type {
  GenerateRedeemCodeInput,
  GenerateRedeemCodeResult,
  RedeemCode,
  RedeemCodeOptions,
  RedeemCodeQuery,
} from '@/api/payment/redeemCode'
import AppDialog from '@/components/AppDialog/index.vue'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import GenerateDialog from './components/GenerateDialog/index.vue'

const { t, locale } = useI18n()
const access = usePermissionStore()
const canList = computed(() => access.hasPermission('payment:redeemCode:list'))
const canDetail = computed(() => access.hasPermission('payment:redeemCode:detail'))
const canGenerate = computed(() => access.hasPermission('payment:redeemCode:generate'))
const canVoid = computed(() => access.hasPermission('payment:redeemCode:void'))
const canExport = computed(() => access.hasPermission('payment:redeemCode:export'))
const {
  options,
  loading: optionsLoading,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<RedeemCodeOptions | null>(api.getRedeemCodeOptions, () => null)

const filters = ref<SearchFormModel<{ batchId: string; status: number | '' }>>({
  batchId: '',
  status: '',
})
const query = ref<RedeemCodeQuery>({ page: 1, pageSize: 20 })
const rows = ref<RedeemCode[]>([])
const selected = ref<RedeemCode[]>([])
const total = ref(0)
const loading = ref(false)
const loadError = ref('')
const lookupValue = ref('')
const lookupLoading = ref(false)
const detail = ref<RedeemCode | null>(null)
const detailVisible = ref(false)
const generateVisible = ref(false)
const generateSubmitting = ref(false)
const generateResult = ref<GenerateRedeemCodeResult | null>(null)
let sequence = 0
let alive = true

const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const searchFields = computed<SearchField<{ batchId: string; status: number | '' }>[]>(() => [
  {
    key: 'batchId',
    type: 'input',
    label: t('payment.redeemCode.batchId'),
    placeholder: t('payment.redeemCode.batchIdPlaceholder'),
    resetValue: '',
    width: 180,
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('payment.redeemCode.status'),
    placeholder: t('payment.redeemCode.allStatuses'),
    resetValue: '',
    options: options.value?.statuses ?? [],
    width: 160,
  },
])
const columns = computed<TableColumn<RedeemCode>[]>(() => [
  { prop: 'batchNo', label: t('payment.redeemCode.batchNo'), width: 170 },
  { prop: 'codeMask', label: t('payment.redeemCode.code'), minWidth: 220 },
  { prop: 'amount', label: t('payment.redeemCode.amount'), width: 120 },
  { key: 'status', label: t('payment.redeemCode.status'), width: 110 },
  { key: 'expiresAt', label: t('payment.redeemCode.expiresAt'), width: 180 },
  { key: 'usedBy', label: t('payment.redeemCode.usedBy'), width: 100 },
  { key: 'actions', label: t('payment.redeemCode.actions'), width: 250, fixed: 'right' },
])
function statusLabel(value: number): string {
  return options.value?.statuses.find((item) => item.value === value)?.label ?? String(value)
}
function statusType(value: number): 'success' | 'info' | 'warning' | 'danger' {
  if (value === 1) return 'success'
  if (value === 2) return 'info'
  if (value === 3) return 'danger'
  return 'warning'
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
function loadQuery(): RedeemCodeQuery {
  const next: RedeemCodeQuery = { page: query.value.page, pageSize: query.value.pageSize }
  const batchId = positive(filters.value.batchId)
  if (batchId !== undefined) next.batchId = batchId
  if (filters.value.status !== '') next.status = filters.value.status
  return next
}
async function load(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  selected.value = []
  try {
    const result = await api.getRedeemCodes(loadQuery())
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = t('payment.redeemCode.requestFailed')
    }
  } finally {
    if (alive && current === sequence) loading.value = false
  }
}
function searchList(): void {
  query.value = { page: 1, pageSize: query.value.pageSize }
  void load()
}
function reset(): void {
  filters.value = { batchId: '', status: '' }
  searchList()
}
function updatePagination(value: TablePaginationState): void {
  query.value = {
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}
function openGenerate(): void {
  if (!canGenerate.value || options.value === null) return
  generateResult.value = null
  generateVisible.value = true
}
async function generate(input: GenerateRedeemCodeInput): Promise<void> {
  if (generateSubmitting.value) return
  generateSubmitting.value = true
  try {
    generateResult.value = await api.generateRedeemCodeBatch(input)
    ElNotification.success({ title: t('payment.redeemCode.generated') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    generateSubmitting.value = false
  }
}
async function openDetail(row: RedeemCode): Promise<void> {
  if (!canDetail.value) return
  detailVisible.value = true
  detail.value = null
  try {
    detail.value = await api.getRedeemCode(row.id)
  } catch {
    detailVisible.value = false
  }
}
async function lookup(): Promise<void> {
  const code = lookupValue.value.trim()
  if (code === '' || lookupLoading.value || !canDetail.value) return
  lookupLoading.value = true
  detail.value = null
  detailVisible.value = true
  try {
    detail.value = await api.lookupRedeemCode(code)
  } catch {
    detailVisible.value = false
  } finally {
    lookupLoading.value = false
  }
}
function selectRows(value: RedeemCode[]): void {
  selected.value = value
}
function csvCell(value: string | number | null): string {
  const text = value === null ? '' : String(value)
  return `"${text.replaceAll('"', '""')}"`
}
function downloadCsv(values: api.RedeemCodeExport[]): void {
  const header = ['batchNo', 'code', 'amount', 'status', 'expiresAt', 'usedBy', 'usedAt']
  const lines = values.map((value) =>
    [
      value.batchNo,
      value.code,
      value.amount,
      statusLabel(value.status),
      value.expiresAt,
      value.usedBy,
      value.usedAt,
    ]
      .map(csvCell)
      .join(','),
  )
  const blob = new Blob(
    [
      `\uFEFF${[header, ...lines].map((line) => (Array.isArray(line) ? line.map(csvCell).join(',') : line)).join('\r\n')}`,
    ],
    { type: 'text/csv;charset=utf-8' },
  )
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = 'payment-redeem-codes.csv'
  link.click()
  URL.revokeObjectURL(link.href)
}
async function exportRows(rowsToExport: RedeemCode[]): Promise<void> {
  if (!canExport.value || rowsToExport.length === 0) return
  try {
    const values = await api.exportRedeemCodes(rowsToExport.map((row) => row.id))
    downloadCsv(values)
    ElNotification.success({ title: t('payment.redeemCode.exported') })
  } catch {
    // request.ts owns the API error notification.
  }
}
async function voidRows(rowsToVoid: RedeemCode[]): Promise<void> {
  const ids = rowsToVoid.filter((row) => row.actions.void).map((row) => row.id)
  if (!canVoid.value || ids.length === 0) return
  try {
    await ElMessageBox.confirm(
      t('payment.redeemCode.voidConfirm', { count: ids.length }),
      t('confirm.title'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  try {
    await api.voidRedeemCodes(ids)
    ElNotification.success({ title: t('payment.redeemCode.voided') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  }
}
watch(
  [canList, locale],
  () => {
    sequence++
    rows.value = []
    total.value = 0
    selected.value = []
    if (canList.value) void load()
  },
  { immediate: true },
)
watch(generateVisible, (visible) => {
  if (!visible) generateResult.value = null
})
onBeforeUnmount(() => {
  alive = false
  sequence++
})
</script>

<template>
  <AppPage class="payment-redeem-code-page">
    <AppSearch
      v-model="filters"
      class="management-page__filters"
      :fields="searchFields"
      :query-label="t('search.query')"
      :reset-label="t('search.reset')"
      @query="searchList"
      @reset="reset"
    />
    <div v-if="optionsError"><el-button text @click="reloadOptions">{{ t('payment.redeemCode.retry') }}</el-button></div>
    
    
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: RedeemCode) => String(row.id)"
      :selectable="canVoid || canExport"
      :selection-selectable="(row: RedeemCode) => row.actions.void || row.actions.export"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('payment.redeemCode.title')"
      @refresh="load"
      @update:pagination="updatePagination"
      @selection-change="selectRows"
    >
      <template #toolbar-left>
        <el-button
          v-if="canGenerate"
          type="primary"
          :icon="CirclePlus"
          :disabled="optionsLoading || options === null"
          data-testid="payment-redeem-code-generate"
          @click="openGenerate"
        >
          {{ t('payment.redeemCode.generate') }}
        </el-button>
        <el-button
          v-if="canVoid && selected.some((row) => row.actions.void)"
          type="danger"
          @click="voidRows(selected)"
        >
          {{ t('payment.redeemCode.voidSelected') }}
        </el-button>
        <el-button
          v-if="canExport && selected.some((row) => row.actions.export)"
          :icon="Download"
          @click="exportRows(selected)"
        >
          {{ t('payment.redeemCode.exportSelected') }}
        </el-button>
      </template>
      <template #toolbar-right>
        <el-input
          v-model="lookupValue"
          :placeholder="t('payment.redeemCode.lookupPlaceholder')"
          clearable
          data-testid="payment-redeem-code-lookup"
          @keyup.enter="lookup"
        />
        <el-button
          v-if="canDetail"
          :icon="Search"
          :loading="lookupLoading"
          data-testid="payment-redeem-code-lookup-submit"
          @click="lookup"
          >{{ t('payment.redeemCode.lookup') }}</el-button
        >
      </template>
      <template #cell-status="{ row }: { row: RedeemCode }"
        ><el-tag size="small" :type="statusType(row.status)">{{
          statusLabel(row.status)
        }}</el-tag></template
      >
      <template #cell-expiresAt="{ row }: { row: RedeemCode }">{{
        row.expiresAt ? formatTime(row.expiresAt) : '-'
      }}</template>
      <template #cell-usedBy="{ row }: { row: RedeemCode }">{{ row.usedBy ?? '-' }}</template>
      <template #cell-actions="{ row }: { row: RedeemCode }">
        <el-button v-if="canDetail" link type="primary" @click="openDetail(row)">{{
          t('payment.redeemCode.detail')
        }}</el-button>
        <el-button
          v-if="canExport && row.actions.export"
          link
          type="primary"
          @click="exportRows([row])"
          >{{ t('payment.redeemCode.export') }}</el-button
        >
        <el-button v-if="canVoid && row.actions.void" link type="danger" @click="voidRows([row])">{{
          t('payment.redeemCode.void')
        }}</el-button>
      </template>
      <template #empty><el-empty :description="t('payment.redeemCode.empty')" /></template>
    </AppTable>

    <GenerateDialog
      v-model="generateVisible"
      :options="options"
      :submitting="generateSubmitting"
      :result="generateResult"
      @submit="generate"
    />
    <AppDialog v-model="detailVisible" :title="t('payment.redeemCode.detail')" width="680px">
      
      <el-skeleton v-if="detail === null" :rows="3" animated />
      <el-descriptions v-else :column="2" border>
        <el-descriptions-item :label="t('payment.redeemCode.code')">{{
          detail.codeMask
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.redeemCode.batchNo')">{{
          detail.batchNo
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.redeemCode.amount')">{{
          detail.amount
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.redeemCode.status')"
          ><el-tag :type="statusType(detail.status)">{{
            statusLabel(detail.status)
          }}</el-tag></el-descriptions-item
        >
        <el-descriptions-item :label="t('payment.redeemCode.expiresAt')">{{
          detail.expiresAt ? formatTime(detail.expiresAt) : '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.redeemCode.usedBy')">{{
          detail.usedBy ?? '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.redeemCode.usedAt')">{{
          detail.usedAt ? formatTime(detail.usedAt) : '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('payment.redeemCode.createdAt')">{{
          formatTime(detail.createdAt)
        }}</el-descriptions-item>
      </el-descriptions>
    </AppDialog>
  </AppPage>
</template>

<style scoped lang="scss">
.payment-redeem-code-page {
  min-width: 0;
}
.payment-redeem-code-page :deep(.el-input) {
  width: 230px;
}
@media (max-width: 768px) {
  .payment-redeem-code-page :deep(.el-input) {
    width: 100%;
  }
}
</style>
