<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElNotification } from 'element-plus/es/components/notification/index'
import * as api from '@/api/ai/provider'
import type { PriceVersion, PriceRate } from '@/api/ai/provider'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { usePermissionStore } from '@/store/permission'
const props = defineProps<{
  modelValue: boolean
  providerId: string
  modelId: string
  modelName: string
}>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})
const { t, locale } = useI18n()
const access = usePermissionStore()
const allowed = (action: string) => access.hasPermission(`ai:provider:${action}`)
const rows = ref<PriceVersion[]>([])
const loading = ref(false)
const failed = ref(false)
const busy = ref(false)
const error = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const pagination = computed(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))
const selected = ref<PriceVersion | null>(null)
interface EditableRate extends PriceRate {
  billingInput: string
}
const rates = ref<EditableRate[]>([])
const effectiveAt = ref('')
const previewFingerprint = ref<string | null>(null)
const fingerprint = computed(() =>
  JSON.stringify({
    effectiveAt: effectiveAt.value,
    rates: rates.value.map((r) => ({ rateId: r.id, billingUnitPrice: r.billingInput })),
  }),
)
const publishable = computed(
  () =>
    selected.value !== null &&
    selected.value.status === 0 &&
    previewFingerprint.value === fingerprint.value &&
    rates.value.length > 0 &&
    rates.value.every((r) => /^(?:0|[1-9]\d*)(?:\.\d{1,8})?$/.test(r.billingInput)),
)
const columns = computed<TableColumn<PriceVersion>[]>(() => [
  { prop: 'version', label: t('aiProvider.priceVersion'), width: 100 },
  { key: 'status', label: t('aiProvider.priceStatus'), width: 120 },
  { prop: 'effectiveAt', label: t('aiProvider.effectiveAt'), minWidth: 210 },
  { key: 'actions', label: t('common.actions'), width: 130 },
])
let sequence = 0
let alive = true
async function load(): Promise<void> {
  if (!props.modelValue || !allowed('detail')) return
  const token = ++sequence
  loading.value = true
  failed.value = false
  try {
    const result = await api.getProviderPrices(props.providerId, props.modelId, {
      page: page.value,
      pageSize: pageSize.value,
    })
    if (alive && token === sequence && props.modelValue) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (token === sequence && alive) {
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
function open(version: PriceVersion, confirmed = false): void {
  selected.value = version
  rates.value = version.rates.map((r) => ({
    ...r,
    billingInput: r.billingUnitPrice === null ? '' : r.billingUnitPrice,
  }))
  effectiveAt.value = version.effectiveAt === null ? '' : version.effectiveAt
  previewFingerprint.value = confirmed ? fingerprint.value : null
  error.value = ''
}
async function sync(): Promise<void> {
  if (!allowed('syncPrice') || busy.value) return
  busy.value = true
  error.value = ''
  const id = props.providerId
  const model = props.modelId
  try {
    const result = await api.syncProviderPrice(id, model)
    if (
      alive &&
      props.modelValue &&
      id === props.providerId &&
      model === props.modelId &&
      allowed('syncPrice')
    ) {
      open(result, true)
      await load()
    }
  } catch {
    /* Request layer owns notifications. */
  } finally {
    busy.value = false
  }
}
async function preview(): Promise<void> {
  if (!allowed('updatePrice') || selected.value === null || busy.value) return
  if (
    rates.value.length === 0 ||
    rates.value.some((r) => !/^(?:0|[1-9]\d*)(?:\.\d{1,8})?$/.test(r.billingInput)) ||
    (effectiveAt.value !== '' &&
      (!/^\d{4}-\d{2}-\d{2}T/.test(effectiveAt.value) ||
        !Number.isFinite(Date.parse(effectiveAt.value))))
  ) {
    error.value = t('aiProvider.invalidPrice')
    return
  }
  busy.value = true
  error.value = ''
  const id = props.providerId
  const model = props.modelId
  try {
    const result = await api.previewProviderPrice(id, model, {
      versionId: selected.value.id,
      ...(effectiveAt.value === ''
        ? {}
        : { effectiveAt: new Date(effectiveAt.value).toISOString() }),
      rates: rates.value.map((r) => ({ rateId: r.id, billingUnitPrice: r.billingInput })),
    })
    if (
      alive &&
      props.modelValue &&
      id === props.providerId &&
      model === props.modelId &&
      allowed('updatePrice')
    ) {
      open(result, true)
      await load()
    }
  } catch {
    /* Keep the edit for retry. */
  } finally {
    busy.value = false
  }
}
async function publish(): Promise<void> {
  if (!allowed('updatePrice') || selected.value === null || !publishable.value || busy.value) return
  const snapshot = selected.value
  busy.value = true
  error.value = ''
  try {
    await api.publishProviderPrice(
      props.providerId,
      props.modelId,
      snapshot.id,
      snapshot.snapshotDigest,
    )
    if (alive && props.modelValue) {
      selected.value = null
      previewFingerprint.value = null
      ElNotification.success({ title: t('aiProvider.saved') })
      await load()
    }
  } catch {
    /* The exact preview remains available for retry. */
  } finally {
    busy.value = false
  }
}
watch(
  () => [props.modelValue, props.providerId, props.modelId, locale.value, allowed('detail')],
  () => {
    ++sequence
    rows.value = []
    total.value = 0
    selected.value = null
    rates.value = []
    previewFingerprint.value = null
    error.value = ''
    page.value = 1
    if (props.modelValue) void load()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  alive = false
  ++sequence
})
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="`${t('aiProvider.prices')} · ${modelName}`"
    width="1100px"
    :close-on-press-escape="!busy"
  >
    <el-alert :title="t('aiProvider.priceSourceHint')" type="info" :closable="false" />
    <el-alert
      v-if="!allowed('detail')"
      :title="t('aiProvider.readDenied')"
      type="warning"
      :closable="false"
    />
    <el-alert v-if="failed" :title="t('aiProvider.priceFailed')" type="error" :closable="false"
      ><el-button link @click="load">{{ t('aiProvider.retry') }}</el-button></el-alert
    >
    <AppTable
      v-if="allowed('detail')"
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :result-state="failed ? 'error' : loading ? 'loading' : rows.length ? 'success' : 'empty'"
      @refresh="load"
      @update:pagination="changePage"
    >
      <template #toolbar-left
        ><el-button
          v-if="allowed('syncPrice')"
          data-testid="ai-price-sync"
          :loading="busy"
          @click="sync"
          >{{ t('aiProvider.syncPrice') }}</el-button
        ></template
      >
      <template #cell-status="{ row }: { row: PriceVersion }">{{
        row.status === 0
          ? t('aiProvider.draft')
          : row.status === 1
            ? t('aiProvider.published')
            : String(row.status)
      }}</template>
      <template #cell-actions="{ row }: { row: PriceVersion }"
        ><el-button
          data-testid="ai-price-open"
          link
          type="primary"
          :disabled="busy"
          @click="open(row)"
          >{{ t('aiProvider.openPrice') }}</el-button
        ></template
      >
      <template #empty
        ><el-empty v-if="!failed" :description="t('aiProvider.priceEmpty')"
      /></template>
    </AppTable>
    <el-button
      v-else-if="allowed('syncPrice')"
      data-testid="ai-price-sync"
      :loading="busy"
      @click="sync"
      >{{ t('aiProvider.syncPrice') }}</el-button
    >
    <section v-if="selected" class="price-editor">
      <el-alert :title="t('aiProvider.priceHint')" type="info" :closable="false" />
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
      <el-form label-position="top" :disabled="busy || !allowed('updatePrice')">
        <el-form-item :label="t('aiProvider.effectiveAt')"
          ><el-input v-model="effectiveAt" :placeholder="t('aiProvider.effectivePlaceholder')"
        /></el-form-item>
        <div v-for="rate in rates" :key="rate.id" class="price-rate">
          <div class="price-rate__identity">
            <strong>{{ rate.operation }} / {{ rate.usageKey }}</strong
            ><code>{{ JSON.stringify(rate.selector) }}</code
            ><span>{{ rate.unitQuantity }} {{ rate.unit }}</span>
          </div>
          <el-form-item :label="t('aiProvider.sourcePrice')"
            ><output>{{ rate.sourceUnitPrice }}</output></el-form-item
          >
          <el-form-item :label="t('aiProvider.billingPrice')"
            ><el-input
              v-model="rate.billingInput"
              data-testid="ai-price-rmb"
              :placeholder="t('aiProvider.billingPlaceholder')"
              inputmode="decimal"
          /></el-form-item>
        </div>
      </el-form>
      <p class="price-digest">
        <strong>{{ t('aiProvider.digest') }}</strong
        ><code>{{ selected.snapshotDigest }}</code>
      </p>
      <div v-if="allowed('updatePrice')" class="price-actions">
        <el-button data-testid="ai-price-preview" :loading="busy" @click="preview">{{
          t('aiProvider.previewPrice')
        }}</el-button
        ><el-button
          data-testid="ai-price-publish"
          type="primary"
          :disabled="!publishable || busy"
          @click="publish"
          >{{ t('aiProvider.publish') }}</el-button
        >
      </div>
    </section>
  </AppDialog>
</template>

<style scoped lang="scss">
.el-alert {
  margin-bottom: 12px;
}
.price-editor {
  border-top: 1px solid var(--el-border-color);
  margin-top: 20px;
  padding-top: 20px;
}
.price-rate {
  display: grid;
  grid-template-columns: minmax(200px, 1fr) 150px minmax(200px, 1fr);
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
  &__identity {
    display: flex;
    flex-direction: column;
    gap: 8px;
    overflow-wrap: anywhere;
  }
}
.price-digest {
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow-wrap: anywhere;
  code {
    font-family: var(--el-font-family);
  }
}
.price-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
@media (max-width: 768px) {
  .price-rate {
    grid-template-columns: 1fr;
  }
}
</style>
