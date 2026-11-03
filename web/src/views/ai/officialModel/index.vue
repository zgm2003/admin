<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElNotification } from 'element-plus/es/components/notification/index'
import * as api from '@/api/ai/officialModel'
import type {
  OfficialModel,
  OfficialModelOptions,
  OfficialModelPrice,
  OfficialModelUpdate,
} from '@/api/ai/officialModel'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { usePermissionStore } from '@/store/permission'
const { t, locale } = useI18n()
const access = usePermissionStore()
const allowed = (action: string) => access.hasPermission(`ai:officialModel:${action}`)
const canList = computed(() => allowed('list'))
const {
  options,
  loading: optionsLoading,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<OfficialModelOptions | null>(api.getOfficialModelOptions, () => null)
const rows = ref<OfficialModel[]>([])
const loading = ref(false)
const failed = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const pagination = computed(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))
const columns = computed<TableColumn<OfficialModel>[]>(() => [
  { prop: 'vendorCode', label: t('aiOfficialModel.vendor'), minWidth: 120 },
  { prop: 'modelKey', label: t('aiOfficialModel.modelKey'), minWidth: 170 },
  { prop: 'name', label: t('aiOfficialModel.name'), minWidth: 150 },
  { key: 'operations', label: t('aiOfficialModel.operations'), minWidth: 170 },
  { key: 'status', label: t('aiOfficialModel.status'), width: 100 },
  { key: 'actions', label: t('common.actions'), minWidth: 320, fixed: 'right' },
])
let sequence = 0
let detailSequence = 0
let priceSequence = 0
let alive = true
async function load(): Promise<void> {
  if (!canList.value) return
  const token = ++sequence
  loading.value = true
  failed.value = false
  try {
    const result = await api.getOfficialModels({ page: page.value, pageSize: pageSize.value })
    if (alive && token === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && token === sequence) {
      rows.value = []
      total.value = 0
      failed.value = true
    }
  } finally {
    if (token === sequence) loading.value = false
  }
}
function changePage(value: TablePaginationState): void {
  page.value = value.pageSize === pageSize.value ? value.currentPage : 1
  pageSize.value = value.pageSize
  void load()
}
const visible = ref(false)
const editingId = ref<string | null>(null)
const readonly = ref(false)
const saving = ref(false)
const error = ref('')
const form = ref({
  vendorCode: '',
  modelKey: '',
  name: '',
  operations: [] as string[],
  sourceUrls: '',
  status: 0,
})
function openCreate(): void {
  if (!allowed('create') || options.value === null) return
  ++detailSequence
  editingId.value = null
  readonly.value = false
  error.value = ''
  form.value = {
    vendorCode: '',
    modelKey: '',
    name: '',
    operations: [],
    sourceUrls: '',
    status: options.value.defaults.status,
  }
  visible.value = true
}
async function openDetail(row: OfficialModel, edit: boolean): Promise<void> {
  if (!allowed('detail') || (edit && !allowed('update'))) return
  const token = ++detailSequence
  try {
    const detail = await api.getOfficialModel(row.id)
    if (alive && token === detailSequence && allowed('detail') && (!edit || allowed('update'))) {
      editingId.value = row.id
      readonly.value = !edit
      error.value = ''
      form.value = {
        vendorCode: detail.vendorCode,
        modelKey: detail.modelKey,
        name: detail.name,
        operations: [...detail.operations],
        sourceUrls: detail.sourceUrls.join('\n'),
        status: detail.status,
      }
      visible.value = true
    }
  } catch {
    /* Request layer notifies. */
  }
}
async function save(): Promise<void> {
  if (
    saving.value ||
    readonly.value ||
    options.value === null ||
    optionsLoading.value ||
    optionsError.value !== '' ||
    !allowed(editingId.value === null ? 'create' : 'update')
  )
    return
  error.value = ''
  const urls = form.value.sourceUrls
    .split('\n')
    .map((v) => v.trim())
    .filter((v) => v !== '')
  let valid =
    form.value.vendorCode.trim() !== '' &&
    form.value.modelKey.trim() !== '' &&
    form.value.name.trim() !== '' &&
    form.value.operations.length > 0 &&
    urls.length <= options.value.constraints.maxSourceUrls
  for (const raw of urls) {
    try {
      const url = new URL(raw)
      if (url.protocol !== 'https:' || url.username !== '' || url.password !== '') valid = false
    } catch {
      valid = false
    }
  }
  if (!valid) {
    error.value = t('aiOfficialModel.required')
    return
  }
  const input: OfficialModelUpdate = {
    name: form.value.name,
    operations: [...form.value.operations],
    sourceUrls: urls,
  }
  saving.value = true
  try {
    if (editingId.value === null)
      await api.createOfficialModel({
        ...input,
        vendorCode: form.value.vendorCode,
        modelKey: form.value.modelKey,
        status: form.value.status,
      })
    else await api.updateOfficialModel(editingId.value, input)
    if (alive) {
      visible.value = false
      ElNotification.success({ title: t('aiOfficialModel.saved') })
      await load()
    }
  } catch {
    /* Keep the form available for retry. */
  } finally {
    saving.value = false
  }
}
const statusVisible = ref(false)
const statusId = ref<string | null>(null)
const nextStatus = ref(0)
function openStatus(row: OfficialModel): void {
  if (!allowed('status')) return
  statusId.value = row.id
  nextStatus.value = row.status
  statusVisible.value = true
}
async function saveStatus(): Promise<void> {
  if (!allowed('status') || statusId.value === null || saving.value) return
  saving.value = true
  try {
    await api.updateOfficialModelStatus(statusId.value, nextStatus.value)
    statusVisible.value = false
    await load()
  } catch {
    /* Request layer notifies. */
  } finally {
    saving.value = false
  }
}
const priceVisible = ref(false)
const priceId = ref<string | null>(null)
const prices = ref<OfficialModelPrice[]>([])
const priceLoading = ref(false)
const priceFailed = ref(false)
const pricePage = ref(1)
const pricePageSize = ref(20)
const priceTotal = ref(0)
const pricePagination = computed(() => ({
  currentPage: pricePage.value,
  pageSize: pricePageSize.value,
  total: priceTotal.value,
}))
const priceColumns = computed<TableColumn<OfficialModelPrice>[]>(() => [
  { prop: 'providerName', label: t('aiOfficialModel.provider'), minWidth: 150 },
  { prop: 'upstreamModelId', label: t('aiProvider.upstreamModel'), minWidth: 160 },
  { prop: 'version', label: t('aiProvider.priceVersion'), width: 90 },
  { prop: 'effectiveAt', label: t('aiProvider.effectiveAt'), minWidth: 205 },
  { key: 'rates', label: t('aiOfficialModel.prices'), minWidth: 380 },
])
async function loadPrices(): Promise<void> {
  if (!allowed('detail') || priceId.value === null || !priceVisible.value) return
  const token = ++priceSequence
  priceLoading.value = true
  priceFailed.value = false
  try {
    const result = await api.getOfficialModelPrices(priceId.value, {
      page: pricePage.value,
      pageSize: pricePageSize.value,
    })
    if (alive && token === priceSequence && priceVisible.value && allowed('detail')) {
      prices.value = result.list
      priceTotal.value = result.total
    }
  } catch {
    if (alive && token === priceSequence) {
      prices.value = []
      priceTotal.value = 0
      priceFailed.value = true
    }
  } finally {
    if (token === priceSequence) priceLoading.value = false
  }
}
function openPrices(row: OfficialModel): void {
  if (!allowed('detail')) return
  ++priceSequence
  priceId.value = row.id
  prices.value = []
  pricePage.value = 1
  priceTotal.value = 0
  priceVisible.value = true
  void loadPrices()
}
function changePricePage(value: TablePaginationState): void {
  pricePage.value = value.pageSize === pricePageSize.value ? value.currentPage : 1
  pricePageSize.value = value.pageSize
  void loadPrices()
}
watch(
  [canList, locale],
  () => {
    ++sequence
    rows.value = []
    total.value = 0
    if (canList.value) void load()
    if (priceVisible.value) void loadPrices()
  },
  { immediate: true },
)
watch(
  () => access.permissionCodes,
  () => {
    ++detailSequence
    if (!allowed('detail')) {
      ++priceSequence
      priceVisible.value = false
      prices.value = []
    }
    if (
      visible.value &&
      !allowed(readonly.value ? 'detail' : editingId.value === null ? 'create' : 'update')
    )
      visible.value = false
  },
  { deep: true },
)
watch(priceVisible, (value) => {
  if (!value) {
    ++priceSequence
    prices.value = []
    priceTotal.value = 0
  }
})
onBeforeUnmount(() => {
  alive = false
  ++sequence
  ++detailSequence
  ++priceSequence
})
</script>

<template>
  <AppPage class="ai-official-model-page">
    <el-alert
      v-if="optionsError"
      :title="t('aiOfficialModel.optionsFailed')"
      type="error"
      :closable="false"
      ><el-button link @click="reloadOptions">{{ t('aiOfficialModel.retry') }}</el-button></el-alert
    >
    <el-alert v-if="failed" :title="t('aiOfficialModel.loadFailed')" type="error" :closable="false"
      ><el-button link @click="load">{{ t('aiOfficialModel.retry') }}</el-button></el-alert
    >
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :result-state="failed ? 'error' : loading ? 'loading' : rows.length ? 'success' : 'empty'"
      :aria-label="t('aiOfficialModel.title')"
      @refresh="load"
      @update:pagination="changePage"
    >
      <template #toolbar-left
        ><el-button
          v-if="allowed('create')"
          data-testid="ai-official-model-create"
          type="primary"
          :disabled="options === null || optionsLoading"
          @click="openCreate"
          >{{ t('aiOfficialModel.create') }}</el-button
        ></template
      >
      <template #cell-status="{ row }: { row: OfficialModel }">{{
        options?.statuses.find((v) => v.value === row.status)?.label ?? String(row.status)
      }}</template>
      <template #cell-operations="{ row }: { row: OfficialModel }">{{
        row.operations
          .map((op) => options?.operations.find((v) => v.value === op)?.label ?? op)
          .join(', ')
      }}</template>
      <template #cell-actions="{ row }: { row: OfficialModel }"
        ><el-button v-if="allowed('detail')" link @click="openDetail(row, false)">{{
          t('aiOfficialModel.detail')
        }}</el-button
        ><el-button
          v-if="allowed('detail') && allowed('update')"
          data-testid="ai-official-model-edit"
          link
          type="primary"
          :disabled="options === null"
          @click="openDetail(row, true)"
          >{{ t('aiOfficialModel.edit') }}</el-button
        ><el-button
          v-if="allowed('detail')"
          link
          type="primary"
          data-testid="ai-official-model-prices"
          @click="openPrices(row)"
          >{{ t('aiOfficialModel.prices') }}</el-button
        ><el-button
          v-if="allowed('status')"
          link
          :disabled="options === null"
          @click="openStatus(row)"
          >{{ t('aiOfficialModel.changeStatus') }}</el-button
        ></template
      >
      <template #empty
        ><el-empty v-if="!failed" :description="t('aiOfficialModel.empty')"
      /></template>
    </AppTable>
    <AppDialog
      v-model="visible"
      :title="
        t(
          readonly
            ? 'aiOfficialModel.detail'
            : editingId === null
              ? 'aiOfficialModel.create'
              : 'aiOfficialModel.edit',
        )
      "
      width="700px"
      :close-on-press-escape="!saving"
    >
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
      <el-form label-position="top" :disabled="readonly || saving">
        <el-form-item :label="t('aiOfficialModel.vendor')" required
          ><el-input
            v-model="form.vendorCode"
            :disabled="editingId !== null"
            :placeholder="t('aiOfficialModel.vendorPlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiOfficialModel.modelKey')" required
          ><el-input
            v-model="form.modelKey"
            :disabled="editingId !== null"
            :maxlength="options?.constraints.maxModelKeyLength"
            :placeholder="t('aiOfficialModel.modelKeyPlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiOfficialModel.name')" required
          ><el-input
            v-model="form.name"
            :maxlength="options?.constraints.maxNameLength"
            :placeholder="t('aiOfficialModel.namePlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiOfficialModel.operations')" required
          ><el-select-v2
            v-model="form.operations"
            multiple
            :options="options === null ? [] : options.operations"
            :placeholder="t('aiOfficialModel.operationsPlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiOfficialModel.sources')"
          ><el-input
            v-model="form.sourceUrls"
            type="textarea"
            :rows="4"
            :placeholder="t('aiOfficialModel.sourcesPlaceholder')"
        /></el-form-item>
        <el-form-item v-if="editingId === null" :label="t('aiOfficialModel.status')"
          ><el-select-v2 v-model="form.status" :options="options === null ? [] : options.statuses"
        /></el-form-item>
      </el-form>
      <template #footer
        ><el-button :disabled="saving" @click="visible = false">{{
          t('aiOfficialModel.cancel')
        }}</el-button
        ><el-button
          v-if="!readonly"
          data-testid="ai-official-model-save"
          type="primary"
          :loading="saving"
          :disabled="options === null || optionsLoading"
          @click="save"
          >{{ t('aiOfficialModel.save') }}</el-button
        ></template
      >
    </AppDialog>
    <AppDialog v-model="statusVisible" :title="t('aiOfficialModel.changeStatus')" width="420px"
      ><el-select-v2
        v-model="nextStatus"
        :options="options === null ? [] : options.statuses"
      /><template #footer
        ><el-button :disabled="saving" @click="statusVisible = false">{{
          t('aiOfficialModel.cancel')
        }}</el-button
        ><el-button
          type="primary"
          :loading="saving"
          :disabled="options === null"
          @click="saveStatus"
          >{{ t('aiOfficialModel.save') }}</el-button
        ></template
      ></AppDialog
    >
    <AppDialog v-model="priceVisible" :title="t('aiOfficialModel.prices')" width="1100px">
      <el-alert :title="t('aiOfficialModel.priceHint')" type="info" :closable="false" />
      <el-alert
        v-if="priceFailed"
        :title="t('aiOfficialModel.priceFailed')"
        type="error"
        :closable="false"
        ><el-button link @click="loadPrices">{{ t('aiOfficialModel.retry') }}</el-button></el-alert
      >
      <AppTable
        :columns="priceColumns"
        :data="prices"
        row-key="providerModelId"
        :loading="priceLoading"
        :pagination="pricePagination"
        :result-state="
          priceFailed ? 'error' : priceLoading ? 'loading' : prices.length ? 'success' : 'empty'
        "
        @refresh="loadPrices"
        @update:pagination="changePricePage"
      >
        <template #cell-rates="{ row }: { row: OfficialModelPrice }"
          ><div v-for="rate in row.rates" :key="rate.id" class="official-price-rate">
            <strong>{{ rate.operation }} · {{ rate.usageKey }}</strong
            ><code>{{ JSON.stringify(rate.selector) }}</code
            ><span
              >{{ rate.unitQuantity }} {{ rate.unit }} · USD {{ rate.sourceUnitPrice }} · CNY
              {{ rate.billingUnitPrice }}</span
            >
          </div></template
        >
        <template #empty
          ><el-empty v-if="!priceFailed" :description="t('aiProvider.priceEmpty')"
        /></template>
      </AppTable>
    </AppDialog>
  </AppPage>
</template>

<style scoped lang="scss">
.ai-official-model-page {
  min-width: 0;
}
.el-alert {
  margin-bottom: 12px;
}
.official-price-rate {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 10px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
  overflow-wrap: anywhere;
}
</style>
