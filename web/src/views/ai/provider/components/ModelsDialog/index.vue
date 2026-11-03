<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as api from '@/api/ai/provider'
import type {
  CatalogPreview,
  CatalogSelection,
  JsonObject,
  JsonValue,
  ProviderModel,
  OfficialCandidate,
  ProviderOptions,
} from '@/api/ai/provider'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { usePermissionStore } from '@/store/permission'
import PriceDialog from '@/views/ai/provider/components/PriceDialog/index.vue'
const props = defineProps<{ modelValue: boolean; providerId: string; providerName: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})
const { t, locale } = useI18n()
const access = usePermissionStore()
const allowed = (action: string) => access.hasPermission(`ai:provider:${action}`)
const {
  options,
  loading: optionsLoading,
  error: optionsError,
} = useLocalizedOptions<ProviderOptions | null>(api.getProviderOptions, () => null)
const rows = ref<ProviderModel[]>([])
const loading = ref(false)
const failed = ref(false)
const error = ref('')
const busy = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const pagination = computed(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))
const columns = computed<TableColumn<ProviderModel>[]>(() => [
  { prop: 'upstreamModelId', label: t('aiProvider.upstreamModel'), minWidth: 170 },
  { prop: 'name', label: t('aiProvider.name'), minWidth: 140 },
  { prop: 'officialModelId', label: t('aiProvider.officialModel'), minWidth: 110 },
  { key: 'operations', label: t('aiProvider.operations'), minWidth: 170 },
  { key: 'status', label: t('aiProvider.status'), width: 90 },
  { key: 'actions', label: t('common.actions'), minWidth: 250 },
])
interface ModelForm {
  id: string
  upstreamModelId: string
  name: string
  officialModelId: string | null
  operations: string[]
  limits: string
}
interface CatalogForm extends ModelForm {
  selected: boolean
}
const editing = ref<ModelForm | null>(null)
const catalog = ref<CatalogPreview | null>(null)
const selections = ref<CatalogForm[]>([])
const candidates = ref<OfficialCandidate[]>([])
const candidatePage = ref(1)
const candidateTotal = ref(0)
const candidateKeyword = ref('')
const candidateLoading = ref(false)
const candidateAction = ref<'update' | 'sync'>('update')
const candidateOptions = computed(() => {
  const values = new Map(
    candidates.value.map((c) => [
      c.id,
      { value: c.id, label: `${c.name} · ${c.vendorCode}/${c.modelKey}`, disabled: c.status !== 1 },
    ]),
  )
  for (const model of [...(editing.value === null ? [] : [editing.value]), ...selections.value]) {
    if (model.officialModelId !== null && !values.has(model.officialModelId))
      values.set(model.officialModelId, {
        value: model.officialModelId,
        label: model.officialModelId,
        disabled: false,
      })
  }
  return [...values.values()]
})
let sequence = 0
let candidateSequence = 0
let alive = true
async function load(): Promise<void> {
  if (!props.modelValue || !allowed('detail')) return
  const token = ++sequence
  loading.value = true
  failed.value = false
  try {
    const result = await api.getProviderModels(props.providerId, {
      page: page.value,
      pageSize: pageSize.value,
    })
    if (alive && token === sequence && props.modelValue && allowed('detail')) {
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
async function loadCandidates(keyword = '', more = false): Promise<void> {
  if (!allowed(candidateAction.value)) return
  const token = ++candidateSequence
  const id = props.providerId
  const requestedPage = more ? candidatePage.value + 1 : 1
  const requestedKeyword = more ? candidateKeyword.value : keyword
  candidateLoading.value = true
  try {
    const result = await api.getProviderCandidates(
      id,
      { page: requestedPage, pageSize: 20, keyword: requestedKeyword },
      candidateAction.value,
    )
    if (alive && props.modelValue && token === candidateSequence && id === props.providerId) {
      candidatePage.value = requestedPage
      candidateKeyword.value = requestedKeyword
      candidates.value = more
        ? [...new Map([...candidates.value, ...result.list].map((v) => [v.id, v])).values()]
        : result.list
      candidateTotal.value = result.total
    }
  } catch {
    /* Errors are notified by the request layer. */
  } finally {
    if (token === candidateSequence) candidateLoading.value = false
  }
}
function edit(row?: ProviderModel): void {
  if (!allowed('update') || options.value === null) return
  editing.value =
    row === undefined
      ? {
          id: '',
          upstreamModelId: '',
          name: '',
          officialModelId: null,
          operations: [],
          limits: '{}',
        }
      : {
          id: row.id,
          upstreamModelId: row.upstreamModelId,
          name: row.name,
          officialModelId: row.officialModelId === undefined ? null : row.officialModelId,
          operations: [...row.operations],
          limits: JSON.stringify(row.limits, null, 2),
        }
  catalog.value = null
  selections.value = []
  error.value = ''
  candidateAction.value = 'update'
  candidates.value = []
  void loadCandidates()
}
function object(raw: string): JsonObject {
  const value: JsonValue = JSON.parse(raw)
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('object')
  return value
}
function selection(row: ModelForm): CatalogSelection {
  return {
    upstreamModelId: row.upstreamModelId,
    name: row.name,
    officialModelId: row.officialModelId === '' ? null : row.officialModelId,
    operations: [...row.operations],
    limits: object(row.limits),
  }
}
async function save(): Promise<void> {
  if (
    !allowed('update') ||
    editing.value === null ||
    busy.value ||
    options.value === null ||
    optionsLoading.value
  )
    return
  if (
    !editing.value.name.trim() ||
    !editing.value.upstreamModelId.trim() ||
    editing.value.operations.length === 0
  ) {
    error.value = t('aiProvider.required')
    return
  }
  let input: api.SaveProviderModel
  try {
    input = { ...selection(editing.value), id: editing.value.id }
  } catch {
    error.value = t('aiProvider.invalidJson')
    return
  }
  const id = props.providerId
  busy.value = true
  error.value = ''
  try {
    await api.saveProviderModels(id, [input])
    if (alive && props.modelValue && id === props.providerId) {
      editing.value = null
      await load()
    }
  } catch {
    /* Preserve edits. */
  } finally {
    busy.value = false
  }
}
async function previewCatalog(): Promise<void> {
  if (!allowed('sync') || busy.value) return
  const id = props.providerId
  busy.value = true
  error.value = ''
  try {
    const result = await api.previewProviderCatalog(id)
    if (alive && props.modelValue && id === props.providerId && allowed('sync')) {
      catalog.value = result
      editing.value = null
      selections.value = result.added.map((m) => ({
        id: '',
        upstreamModelId: m.id,
        name: m.id,
        officialModelId: null,
        operations: [],
        limits: '{}',
        selected: false,
      }))
      candidates.value = []
      candidateAction.value = 'sync'
      void loadCandidates()
    }
  } catch {
    /* Request layer notifies. */
  } finally {
    busy.value = false
  }
}
async function confirmCatalog(): Promise<void> {
  if (
    !allowed('sync') ||
    catalog.value === null ||
    busy.value ||
    options.value === null ||
    optionsLoading.value
  )
    return
  const selected = selections.value.filter((m) => m.selected)
  if (selected.length === 0 || selected.some((m) => !m.name.trim() || m.operations.length === 0)) {
    error.value = t('aiProvider.required')
    return
  }
  let input: CatalogSelection[]
  try {
    input = selected.map(selection)
  } catch {
    error.value = t('aiProvider.invalidJson')
    return
  }
  const id = props.providerId
  busy.value = true
  error.value = ''
  try {
    await api.confirmProviderCatalog(id, catalog.value.digest, input)
    if (alive && props.modelValue && id === props.providerId) {
      catalog.value = null
      selections.value = []
      await load()
    }
  } catch {
    /* Preserve the preview, never infer replacement rows. */
  } finally {
    busy.value = false
  }
}
const statusVisible = ref(false)
const statusModel = ref<ProviderModel | null>(null)
const nextStatus = ref(0)
function openStatus(row: ProviderModel): void {
  if (!allowed('status')) return
  statusModel.value = row
  nextStatus.value = row.status
  statusVisible.value = true
}
async function saveStatus(): Promise<void> {
  if (!allowed('status') || statusModel.value === null || busy.value) return
  busy.value = true
  try {
    await api.updateProviderModelStatus(props.providerId, statusModel.value.id, nextStatus.value)
    statusVisible.value = false
    await load()
  } catch {
    /* Request layer notifies. */
  } finally {
    busy.value = false
  }
}
const priceVisible = ref(false)
const priceModel = ref<ProviderModel | null>(null)
function openPrice(row: ProviderModel): void {
  if (!allowed('detail')) return
  priceModel.value = row
  priceVisible.value = true
}
watch(
  () => [props.modelValue, props.providerId, locale.value, allowed('detail')],
  () => {
    ++sequence
    ++candidateSequence
    rows.value = []
    total.value = 0
    editing.value = null
    catalog.value = null
    selections.value = []
    candidates.value = []
    page.value = 1
    priceVisible.value = false
    error.value = ''
    if (props.modelValue) void load()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  alive = false
  ++sequence
  ++candidateSequence
})
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="`${t('aiProvider.models')} · ${providerName}`"
    width="1100px"
    :close-on-press-escape="!busy"
  >
    <el-alert
      v-if="optionsError"
      :title="t('aiProvider.optionsFailed')"
      type="error"
      :closable="false"
    />
    <div class="model-toolbar">
      <el-button
        v-if="allowed('sync')"
        data-testid="ai-provider-sync"
        :loading="busy"
        :disabled="options === null"
        @click="previewCatalog"
        >{{ t('aiProvider.sync') }}</el-button
      ><el-button
        v-if="allowed('update')"
        data-testid="ai-provider-add-model"
        :disabled="busy || options === null"
        @click="edit()"
        >{{ t('aiProvider.addModel') }}</el-button
      >
    </div>
    <el-alert
      v-if="!allowed('detail')"
      :title="t('aiProvider.readDenied')"
      type="warning"
      :closable="false"
    />
    <el-alert v-if="failed" :title="t('aiProvider.modelFailed')" type="error" :closable="false"
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
      <template #cell-operations="{ row }: { row: ProviderModel }">{{
        row.operations
          .map((op) => options?.operations.find((v) => v.value === op)?.label ?? op)
          .join(', ')
      }}</template>
      <template #cell-status="{ row }: { row: ProviderModel }">{{
        options?.statuses.find((v) => v.value === row.status)?.label ?? String(row.status)
      }}</template>
      <template #cell-actions="{ row }: { row: ProviderModel }"
        ><el-button v-if="allowed('update')" link type="primary" @click="edit(row)">{{
          t('aiProvider.editModel')
        }}</el-button
        ><el-button
          v-if="allowed('status')"
          link
          :disabled="options === null"
          @click="openStatus(row)"
          >{{ t('aiProvider.changeStatus') }}</el-button
        ><el-button link type="primary" @click="openPrice(row)">{{
          t('aiProvider.prices')
        }}</el-button></template
      >
      <template #empty
        ><el-empty v-if="!failed" :description="t('aiProvider.modelEmpty')"
      /></template>
    </AppTable>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-form v-if="editing" label-position="top" :disabled="busy" class="model-editor">
      <el-form-item :label="t('aiProvider.upstreamModel')" required
        ><el-input
          v-model="editing.upstreamModelId"
          :disabled="editing.id !== ''"
          :placeholder="t('aiProvider.upstreamPlaceholder')"
      /></el-form-item>
      <el-form-item :label="t('aiProvider.name')" required
        ><el-input
          v-model="editing.name"
          :placeholder="t('aiProvider.namePlaceholder')"
          :maxlength="options?.constraints.maxNameLength"
      /></el-form-item>
      <el-form-item :label="t('aiProvider.officialModel')"
        ><el-select-v2
          v-model="editing.officialModelId"
          clearable
          filterable
          remote
          :remote-method="loadCandidates"
          :loading="candidateLoading"
          :options="candidateOptions"
          :placeholder="t('aiProvider.officialPlaceholder')"
      /></el-form-item>
      <el-button
        v-if="candidates.length < candidateTotal"
        :loading="candidateLoading"
        @click="loadCandidates('', true)"
        >{{ t('aiProvider.more') }}</el-button
      >
      <el-form-item :label="t('aiProvider.operations')" required
        ><el-select-v2
          v-model="editing.operations"
          multiple
          :options="options === null ? [] : options.operations"
          :placeholder="t('aiProvider.operationPlaceholder')"
      /></el-form-item>
      <el-form-item :label="t('aiProvider.limits')"
        ><el-input
          v-model="editing.limits"
          type="textarea"
          :placeholder="t('aiProvider.limitsPlaceholder')"
      /></el-form-item>
      <el-button @click="editing = null">{{ t('aiProvider.cancel') }}</el-button
      ><el-button
        type="primary"
        :loading="busy"
        :disabled="options === null || optionsLoading"
        @click="save"
        >{{ t('aiProvider.save') }}</el-button
      >
    </el-form>
    <section v-if="catalog" class="catalog-preview">
      <el-alert :title="t('aiProvider.syncHint')" type="info" :closable="false" />
      <p>
        {{
          t('aiProvider.catalogSummary', {
            added: catalog.added.length,
            existing: catalog.existing.length,
          })
        }}
      </p>
      <p class="catalog-digest">
        {{ t('aiProvider.digest') }}: <code>{{ catalog.digest }}</code>
      </p>
      <el-form
        v-for="model in selections"
        :key="model.upstreamModelId"
        class="catalog-model"
        label-position="top"
        :disabled="busy"
      >
        <el-checkbox v-model="model.selected"
          >{{ t('aiProvider.select') }} · {{ model.upstreamModelId }}</el-checkbox
        >
        <template v-if="model.selected">
          <el-form-item :label="t('aiProvider.name')" required
            ><el-input v-model="model.name" :placeholder="t('aiProvider.namePlaceholder')"
          /></el-form-item>
          <el-form-item :label="t('aiProvider.officialModel')"
            ><el-select-v2
              v-model="model.officialModelId"
              clearable
              filterable
              remote
              :remote-method="loadCandidates"
              :loading="candidateLoading"
              :options="candidateOptions"
              :placeholder="t('aiProvider.officialPlaceholder')"
          /></el-form-item>
          <el-form-item :label="t('aiProvider.operations')" required
            ><el-select-v2
              v-model="model.operations"
              multiple
              :options="options === null ? [] : options.operations"
              :placeholder="t('aiProvider.operationPlaceholder')"
          /></el-form-item>
          <el-form-item :label="t('aiProvider.limits')"
            ><el-input
              v-model="model.limits"
              type="textarea"
              :placeholder="t('aiProvider.limitsPlaceholder')"
          /></el-form-item>
        </template>
      </el-form>
      <el-button
        v-if="candidates.length < candidateTotal"
        :loading="candidateLoading"
        @click="loadCandidates('', true)"
        >{{ t('aiProvider.more') }}</el-button
      >
      <el-button
        type="primary"
        data-testid="ai-provider-confirm-sync"
        :disabled="!selections.some((m) => m.selected) || options === null || optionsLoading"
        :loading="busy"
        @click="confirmCatalog"
        >{{ t('aiProvider.confirmSync') }}</el-button
      >
    </section>
    <AppDialog v-model="statusVisible" :title="t('aiProvider.changeStatus')" width="420px"
      ><el-select-v2
        v-model="nextStatus"
        :options="options === null ? [] : options.statuses"
      /><template #footer
        ><el-button
          type="primary"
          :disabled="options === null"
          :loading="busy"
          @click="saveStatus"
          >{{ t('aiProvider.save') }}</el-button
        ></template
      ></AppDialog
    >
    <PriceDialog
      v-if="priceModel"
      v-model="priceVisible"
      :provider-id="providerId"
      :model-id="priceModel.id"
      :model-name="priceModel.name"
    />
  </AppDialog>
</template>

<style scoped lang="scss">
.model-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}
.model-editor,
.catalog-preview {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--el-border-color);
}
.catalog-model {
  padding: 12px;
  margin: 12px 0;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
}
.catalog-digest {
  overflow-wrap: anywhere;
}
.el-alert {
  margin-bottom: 12px;
}
</style>
