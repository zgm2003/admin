<!-- 组件：供应商模型同步与配置 -->

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElNotification } from 'element-plus/es/components/notification/index'

import * as api from '@/api/ai/providerModel'
import type {
  AiProviderModel,
  ProviderModelOptions,
  ProviderModelQuery,
  UpdateProviderModelInput,
} from '@/api/ai/providerModel'
import type { AiProvider } from '@/api/ai/provider'
import { usePermissionStore } from '@/store/permission'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import AppDialog from '@/components/AppDialog/index.vue'
import ModelEditDialog from './ModelEditDialog/index.vue'

defineOptions({ name: 'AIProviderModelDialog' })

const props = defineProps<{ provider: AiProvider }>()
const visible = defineModel<boolean>({ required: true })
const { t, locale } = useI18n()
const access = usePermissionStore()

const rows = ref<AiProviderModel[]>([])
const total = ref(0)
const query = ref<ProviderModelQuery>({ page: 1, pageSize: 20 })
const keyword = ref('')
const kindFilter = ref<number | ''>('')
const statusFilter = ref<number | ''>('')
const loading = ref(false)
const loadError = ref('')
const options = ref<ProviderModelOptions>({
  modelKinds: [],
  pricingModes: [],
  priceUnits: [],
  officialModels: [],
})
const optionsLoading = ref(false)
const optionsError = ref('')
const syncing = ref(false)
const batchEnabling = ref(false)
const selection = ref<AiProviderModel[]>([])
const tableRef = ref<{ clearSelection: () => void }>()
const editVisible = ref(false)
const editing = ref<AiProviderModel | null>(null)
const submitting = ref(false)
const editError = ref('')
let sequence = 0

const canSync = computed(() => access.hasPermission('ai:provider:model:sync'))
const canUpdate = computed(() => access.hasPermission('ai:provider:model:update'))
const statusOptions = computed(() => [
  { value: 1, label: t('ai.providerModel.enabled') },
  { value: 0, label: t('ai.providerModel.disabled') },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const columns = computed<TableColumn<AiProviderModel>[]>(() => [
  { key: 'model', label: t('ai.providerModel.model'), minWidth: 250 },
  { key: 'kind', label: t('ai.providerModel.modelKind'), width: 90 },
  { key: 'pricing', label: t('ai.providerModel.pricing'), width: 180 },
  { key: 'official', label: t('ai.providerModel.officialModel'), minWidth: 220 },
  { key: 'status', label: t('ai.providerModel.status'), width: 90 },
  { key: 'lastSeenAt', label: t('ai.providerModel.lastSeenAt'), width: 160 },
  { key: 'actions', label: t('common.actions'), width: 90, fixed: 'right' },
])

function resultState(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value || optionsLoading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}

async function loadOptions(): Promise<void> {
  optionsLoading.value = true
  optionsError.value = ''
  try {
    options.value = await api.getProviderModelOptions(props.provider.id)
  } catch {
    optionsError.value = t('ai.providerModel.optionsFailed')
  } finally {
    optionsLoading.value = false
  }
}

async function load(): Promise<void> {
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getProviderModels(props.provider.id, query.value)
    if (current === sequence) {
      rows.value = result.list
      total.value = result.total
      selection.value = []
      tableRef.value?.clearSelection()
    }
  } catch {
    if (current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = t('ai.providerModel.requestFailed')
    }
  } finally {
    if (current === sequence) loading.value = false
  }
}

function applyFilters(): void {
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(keyword.value.trim() ? { keyword: keyword.value.trim() } : {}),
    ...(kindFilter.value !== '' ? { modelKind: kindFilter.value } : {}),
    ...(statusFilter.value !== '' ? { status: statusFilter.value } : {}),
  }
  void load()
}

function resetFilters(): void {
  keyword.value = ''
  kindFilter.value = ''
  statusFilter.value = ''
  query.value = { page: 1, pageSize: query.value.pageSize }
  void load()
}

function updatePagination(value: TablePaginationState): void {
  query.value = {
    ...query.value,
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}

function canSelectForBatch(row: AiProviderModel): boolean {
  if (row.status === 1) return false
  if (row.modelKind === 1) {
    return row.pricingMode === 1 && row.officialModel !== null
  }
  if (row.pricingMode === 1) return row.officialModel !== null
  return row.prices.length > 0
}

function onSelectionChange(value: AiProviderModel[]): void {
  selection.value = value
}

const batchSelection = computed(() => selection.value.filter(canSelectForBatch))

async function sync(): Promise<void> {
  if (!canSync.value || syncing.value || batchEnabling.value) return
  syncing.value = true
  try {
    const result = await api.syncProviderModels(props.provider.id)
    ElNotification.success({
      title: t('ai.providerModel.syncDone'),
      message: t('ai.providerModel.syncSummary', {
        total: result.total,
        inserted: result.inserted,
        updated: result.updated,
        deactivated: result.deactivated,
        unmatched: result.unmatched,
      }),
    })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    syncing.value = false
  }
}

async function enableSelected(): Promise<void> {
  if (!canUpdate.value || batchEnabling.value || batchSelection.value.length === 0) return
  const targets = [...batchSelection.value]
  batchEnabling.value = true
  let enabled = 0
  let failed = 0
  for (const row of targets) {
    try {
      await api.updateProviderModel(props.provider.id, row.id, {
        displayName: row.displayName,
        modelKind: row.modelKind,
        pricingMode: row.pricingMode,
        priceUnit: row.priceUnit,
        officialModelId: row.officialModel?.id ?? null,
        status: 1,
        prices: row.prices.map((price) => ({
          tierKey: price.tierKey,
          tierLabel: price.tierLabel,
          priceCny: price.priceCny,
        })),
      })
      enabled++
    } catch {
      failed++
    }
  }
  await load()
  tableRef.value?.clearSelection()
  selection.value = []
  batchEnabling.value = false
  if (failed === 0) {
    ElNotification.success({
      title: t('ai.providerModel.batchEnableDone'),
      message: t('ai.providerModel.batchEnableSummary', { enabled, failed }),
    })
    return
  }
  ElNotification.warning({
    title: t('ai.providerModel.batchEnablePartial'),
    message: t('ai.providerModel.batchEnableSummary', { enabled, failed }),
  })
}

function openEdit(row: AiProviderModel): void {
  if (!canUpdate.value) return
  editing.value = row
  editError.value = ''
  editVisible.value = true
}

async function saveModel(payload: UpdateProviderModelInput): Promise<void> {
  if (!editing.value || submitting.value) return
  submitting.value = true
  editError.value = ''
  try {
    await api.updateProviderModel(props.provider.id, editing.value.id, payload)
    editVisible.value = false
    ElNotification.success({ title: t('ai.providerModel.saved') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    submitting.value = false
  }
}

function formatDate(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

watch(
  () => [visible.value, props.provider.id] as const,
  ([open]) => {
    if (!open) return
    sequence++
    rows.value = []
    total.value = 0
    keyword.value = ''
    kindFilter.value = ''
    statusFilter.value = ''
    query.value = { page: 1, pageSize: 20 }
    void Promise.all([loadOptions(), load()])
  },
  { immediate: true },
)

watch(visible, (open) => {
  if (!open) {
    editing.value = null
    editError.value = ''
  }
})
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="t('ai.providerModel.title', { name: provider.name })"
    width="1040px"
    :close-on-click-modal="false"
    :show-close="!syncing && !submitting"
  >
    <div class="ai-provider-model">
      <p class="ai-provider-model__description">
        {{ t('ai.providerModel.description') }}
      </p>

      <div class="ai-provider-model__toolbar">
        <div class="ai-provider-model__filters">
          <el-input
            v-model="keyword"
            clearable
            :placeholder="t('ai.providerModel.keywordPlaceholder')"
            class="ai-provider-model__keyword"
            @keyup.enter="applyFilters"
          />
          <el-select-v2
            v-model="kindFilter"
            :options="options.modelKinds"
            clearable
            :placeholder="t('ai.providerModel.allKinds')"
            class="ai-provider-model__filter-select"
          />
          <el-select-v2
            v-model="statusFilter"
            :options="statusOptions"
            clearable
            :placeholder="t('ai.providerModel.allStatuses')"
            class="ai-provider-model__filter-select"
          />
          <el-button @click="applyFilters">{{ t('common.search') }}</el-button>
          <el-button text @click="resetFilters">{{ t('common.reset') }}</el-button>
        </div>
        <div class="ai-provider-model__toolbar-actions">
          <el-button
            v-if="canUpdate"
            type="primary"
            plain
            :loading="batchEnabling"
            :disabled="batchSelection.length === 0 || syncing || loading || submitting"
            :title="t('ai.providerModel.batchEnableHint')"
            @click="enableSelected"
          >
            {{
              batchSelection.length > 0
                ? t('ai.providerModel.batchEnableCount', { count: batchSelection.length })
                : t('ai.providerModel.batchEnable')
            }}
          </el-button>
          <el-button
            v-if="canSync"
            type="primary"
            :loading="syncing"
            :disabled="loading || submitting || batchEnabling"
            @click="sync"
          >
            {{ t('ai.providerModel.sync') }}
          </el-button>
        </div>
      </div>

      <el-alert
        v-if="loadError || optionsError"
        :title="loadError || optionsError"
        type="error"
        :closable="false"
        class="ai-provider-model__alert"
      />

      <AppTable
        ref="tableRef"
        :columns="columns"
        :data="rows"
        :loading="loading"
        :pagination="pagination"
        :row-key="(row: AiProviderModel) => String(row.id)"
        :selectable="canUpdate"
        :selection-selectable="canSelectForBatch"
        :result-state="resultState()"
        :status-message="loadError"
        :aria-label="t('ai.providerModel.title', { name: provider.name })"
        @refresh="load"
        @selection-change="onSelectionChange"
        @update:pagination="updatePagination"
      >
        <template #cell-model="{ row }: { row: AiProviderModel }">
          <div class="ai-provider-model__identity">
            <strong>{{ row.displayName }}</strong>
            <span>{{ t('ai.providerModel.modelId') }}: {{ row.upstreamModel }}</span>
          </div>
        </template>
        <template #cell-kind="{ row }: { row: AiProviderModel }">
          <el-tag effect="plain">{{ row.kindName }}</el-tag>
        </template>
        <template #cell-pricing="{ row }: { row: AiProviderModel }">
          <div class="ai-provider-model__pricing">
            <span>{{ row.pricingName }}</span>
            <small>{{ row.unitName }}</small>
          </div>
        </template>
        <template #cell-official="{ row }: { row: AiProviderModel }">
          <div v-if="row.officialModel" class="ai-provider-model__pricing">
            <span>{{ row.officialModel.displayName || row.officialModel.modelKey }}</span>
            <small
              v-if="
                row.officialModel.displayName &&
                row.officialModel.displayName !== row.officialModel.modelKey
              "
            >
              {{ row.officialModel.modelKey }}
            </small>
          </div>
          <el-tag v-else type="warning" effect="plain">
            {{ t('ai.providerModel.unmatched') }}
          </el-tag>
        </template>
        <template #cell-status="{ row }: { row: AiProviderModel }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'" effect="plain">
            {{ row.statusName }}
          </el-tag>
        </template>
        <template #cell-lastSeenAt="{ row }: { row: AiProviderModel }">
          <span class="ai-provider-model__muted">{{ formatDate(row.lastSeenAt) }}</span>
        </template>
        <template #cell-actions="{ row }: { row: AiProviderModel }">
          <el-button
            v-if="canUpdate && row.actions.update"
            link
            type="primary"
            :disabled="submitting || syncing || batchEnabling"
            @click="openEdit(row)"
          >
            {{ t('common.edit') }}
          </el-button>
        </template>
        <template #empty>
          <el-empty :description="t('ai.providerModel.empty')">
            <el-button v-if="canSync" type="primary" :loading="syncing" @click="sync">
              {{ t('ai.providerModel.sync') }}
            </el-button>
          </el-empty>
        </template>
      </AppTable>

      <p class="ai-provider-model__hint">{{ t('ai.providerModel.hint') }}</p>
    </div>

    <template #footer>
      <el-button :disabled="syncing || submitting" @click="visible = false">
        {{ t('common.close') }}
      </el-button>
    </template>
  </AppDialog>

  <ModelEditDialog
    v-if="editing"
    v-model="editVisible"
    :model="editing"
    :options="options"
    :submitting="submitting"
    :error="editError"
    @save="saveModel"
  />
</template>

<style scoped lang="scss">
.ai-provider-model {
  display: flex;
  flex-direction: column;
  gap: 12px;

  &__description {
    margin: -2px 0 0;
    color: var(--admin-text-muted);
    font-size: 12px;
    line-height: 1.5;
  }

  &__toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  &__toolbar-actions {
    display: flex;
    flex-shrink: 0;
    gap: 8px;
  }

  &__filters {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  &__keyword {
    width: 220px;
  }

  &__filter-select {
    width: 150px;
  }

  &__identity,
  &__pricing {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;

    span,
    small {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    span {
      color: var(--admin-text);
      font-size: 13px;
    }

    small {
      color: var(--admin-text-muted);
      font-size: 11px;
    }
  }

  &__muted {
    color: var(--admin-text-muted);
    font-size: 12px;
  }

  &__hint {
    margin: 0;
    color: var(--admin-text-muted);
    font-size: 12px;
  }

  &__alert {
    margin-bottom: 0;
  }
}

@media (max-width: 720px) {
  .ai-provider-model__toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .ai-provider-model__keyword {
    width: 100%;
  }
}
</style>
