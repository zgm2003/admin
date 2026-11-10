<!-- 页面：官方模型 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'

import * as api from '@/api/ai/officialModel'
import type {
  OfficialModel,
  OfficialModelOptions,
  OfficialModelQuery,
} from '@/api/ai/officialModel'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { usePermissionStore } from '@/store/permission'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import PriceDialog from './components/PriceDialog/index.vue'
import type { PriceForm } from './components/PriceDialog/types'

const { t, locale } = useI18n()
const access = usePermissionStore()
const {
  options,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<OfficialModelOptions>(api.getOfficialModelOptions, () => ({ vendors: [] }))

const rows = ref<OfficialModel[]>([])
const total = ref(0)
const query = ref<OfficialModelQuery>({ page: 1, pageSize: 20 })
const filters = ref<SearchFormModel<{ keyword: string; vendor: number | '' }>>({
  keyword: '',
  vendor: '',
})
const loading = ref(false)
const loadError = ref('')
const syncing = ref(false)
const busyID = ref<number | null>(null)
const dialogVisible = ref(false)
const editing = ref<OfficialModel | null>(null)
const submitting = ref(false)
const formError = ref('')
const narrowMedia = window.matchMedia?.('(max-width: 900px)')
const isNarrow = ref(narrowMedia?.matches ?? false)
const syncNarrow = (event: MediaQueryListEvent): void => {
  isNarrow.value = event.matches
}
narrowMedia?.addEventListener('change', syncNarrow)
let sequence = 0
let alive = true

const canList = computed(() => access.hasPermission('ai:officialModel:list'))
const canUpdate = computed(() => access.hasPermission('ai:officialModel:update'))
const canSync = computed(() => access.hasPermission('ai:officialModel:sync'))
const canReset = computed(() => access.hasPermission('ai:officialModel:reset'))
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const searchFields = computed<SearchField<{ keyword: string; vendor: number | '' }>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('ai.officialModel.keyword'),
    placeholder: t('ai.officialModel.keywordPlaceholder'),
    resetValue: '',
    width: 260,
  },
  {
    key: 'vendor',
    type: 'select-v2',
    label: t('ai.officialModel.vendor'),
    placeholder: t('ai.officialModel.allVendors'),
    resetValue: '',
    options: options.value.vendors,
    width: 180,
  },
])
const columns = computed<TableColumn<OfficialModel>[]>(() => [
  { key: 'model', label: t('ai.officialModel.model'), minWidth: 250 },
  { key: 'officialInput', label: t('ai.officialModel.officialInput'), width: 120 },
  { key: 'officialOutput', label: t('ai.officialModel.officialOutput'), width: 120 },
  { key: 'cnyInput', label: t('ai.officialModel.cnyInput'), width: 130 },
  { key: 'cnyOutput', label: t('ai.officialModel.cnyOutput'), width: 130 },
  { key: 'video', label: t('ai.officialModel.videoPrice'), minWidth: 260 },
  {
    key: 'actions',
    label: t('common.actions'),
    width: 190,
    fixed: isNarrow.value ? false : 'right',
  },
])
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
    const result = await api.getOfficialModels(query.value)
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = t('ai.officialModel.requestFailed')
    }
  } finally {
    if (alive && current === sequence) loading.value = false
  }
}

function search(): void {
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(filters.value.keyword.trim() ? { keyword: filters.value.keyword.trim() } : {}),
    ...(filters.value.vendor !== '' ? { vendor: filters.value.vendor } : {}),
  }
  void load()
}

function reset(): void {
  filters.value = { keyword: '', vendor: '' }
  search()
}

function updatePagination(value: TablePaginationState): void {
  query.value = {
    ...query.value,
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}

function price(value: string, currency: 'USD' | 'CNY'): string {
  return `${currency === 'USD' ? '$' : '¥'}${value}/M`
}

function modelKindLabel(kind: number): string {
  if (kind === 2) return t('ai.officialModel.kindImage')
  if (kind === 3) return t('ai.officialModel.kindVideo')
  if (kind === 4) return t('ai.officialModel.kindAudio')
  return t('ai.officialModel.kindChat')
}

function videoPriceSummary(row: OfficialModel): string {
  const items = (row.videoPrices ?? []).filter(
    (item) =>
      item.unit === 'second' && item.priceCnyPerSecond && item.tierKey.endsWith(':no_input'),
  )
  if (items.length === 0) return '—'
  return items
    .map((item) => `${item.tierKey.split(':')[0]} ¥${item.priceCnyPerSecond}/s`)
    .join(' · ')
}

function openEdit(row: OfficialModel): void {
  if (!canUpdate.value || !row.actions.update) return
  editing.value = row
  formError.value = ''
  dialogVisible.value = true
}

async function save(payload: PriceForm): Promise<void> {
  if (submitting.value || editing.value === null || !canUpdate.value) return
  submitting.value = true
  formError.value = ''
  try {
    await api.updateOfficialModelPrice(editing.value.id, {
      expectedVersion: editing.value.version,
      inputCnyPerMillion: payload.inputCny,
      outputCnyPerMillion: payload.outputCny,
      ...(payload.cacheReadCny ? { cacheReadCnyPerMillion: payload.cacheReadCny } : {}),
      ...(payload.cacheWriteCny ? { cacheWriteCnyPerMillion: payload.cacheWriteCny } : {}),
    })
    dialogVisible.value = false
    ElNotification.success({ title: t('ai.officialModel.saved') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    submitting.value = false
  }
}

async function resetPrice(row: OfficialModel): Promise<void> {
  if (!canReset.value || !row.actions.reset || busyID.value !== null) return
  try {
    await ElMessageBox.confirm(
      t('ai.officialModel.resetConfirm', {
        model: row.displayName,
        input: row.inputUsdPerMillion,
        output: row.outputUsdPerMillion,
      }),
      t('confirm.title'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  busyID.value = row.id
  try {
    await api.resetOfficialModelPrice(row.id, row.version)
    ElNotification.success({ title: t('ai.officialModel.resetDone') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyID.value = null
  }
}

async function syncCatalog(): Promise<void> {
  if (!canSync.value || syncing.value) return
  try {
    await ElMessageBox.confirm(t('ai.officialModel.syncConfirm'), t('confirm.title'), {
      type: 'warning',
    })
  } catch {
    return
  }
  syncing.value = true
  try {
    const result = await api.syncOfficialModelCatalog()
    ElNotification.success({
      title: t('ai.officialModel.synced'),
      message: t('ai.officialModel.syncSummary', {
        inserted: result.inserted,
        updated: result.updated,
        unchanged: result.unchanged,
      }),
    })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    syncing.value = false
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
watch(dialogVisible, (visible) => {
  if (!visible) {
    editing.value = null
    formError.value = ''
  }
})
onBeforeUnmount(() => {
  alive = false
  sequence++
  narrowMedia?.removeEventListener('change', syncNarrow)
})
</script>

<template>
  <AppPage class="ai-official-model-page">
    <AppSearch
      v-model="filters"
      class="management-page__filters"
      :fields="searchFields"
      @query="search"
      @reset="reset"
    />
    <div v-if="optionsError" class="ai-official-model-page__retry">
      <el-button text @click="reloadOptions">{{ t('common.retry') }}</el-button>
    </div>

    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: OfficialModel) => String(row.id)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('ai.officialModel.title')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #toolbar-left>
        <el-button
          v-if="canSync"
          type="primary"
          :icon="Refresh"
          :loading="syncing"
          :disabled="submitting || busyID !== null"
          data-testid="ai-official-model-sync"
          @click="syncCatalog"
        >
          {{ t('ai.officialModel.sync') }}
        </el-button>
      </template>
      <template #toolbar-right>
        <span class="ai-official-model-page__hint">{{ t('ai.officialModel.syncHint') }}</span>
      </template>
      <template #cell-model="{ row }: { row: OfficialModel }">
        <div class="ai-official-model-page__model">
          <span class="ai-official-model-page__model-vendor">{{ row.vendorName }}</span>
          <strong>{{ row.displayName }}</strong>
          <el-tag v-if="row.modelKind !== 1" size="small" type="warning" effect="plain">
            {{ modelKindLabel(row.modelKind) }}
          </el-tag>
          <code v-if="row.displayName !== row.modelKey">{{ row.modelKey }}</code>
        </div>
      </template>
      <template #cell-officialInput="{ row }: { row: OfficialModel }">
        <span
          v-if="row.modelKind === 1"
          class="ai-official-model-page__money ai-official-model-page__money--official"
        >
          {{ price(row.inputUsdPerMillion, 'USD') }}
        </span>
        <span v-else class="ai-official-model-page__money">—</span>
      </template>
      <template #cell-officialOutput="{ row }: { row: OfficialModel }">
        <span
          v-if="row.modelKind === 1"
          class="ai-official-model-page__money ai-official-model-page__money--official"
        >
          {{ price(row.outputUsdPerMillion, 'USD') }}
        </span>
        <span v-else class="ai-official-model-page__money">—</span>
      </template>
      <template #cell-cnyInput="{ row }: { row: OfficialModel }">
        <span v-if="row.modelKind === 1" class="ai-official-model-page__money">
          {{ price(row.inputCnyPerMillion, 'CNY') }}
        </span>
        <span v-else class="ai-official-model-page__money">—</span>
      </template>
      <template #cell-cnyOutput="{ row }: { row: OfficialModel }">
        <span v-if="row.modelKind === 1" class="ai-official-model-page__money">
          {{ price(row.outputCnyPerMillion, 'CNY') }}
        </span>
        <span v-else class="ai-official-model-page__money">—</span>
      </template>
      <template #cell-video="{ row }: { row: OfficialModel }">
        <span v-if="row.modelKind === 3" class="ai-official-model-page__video-prices">
          {{ videoPriceSummary(row) }}
        </span>
        <span v-else class="ai-official-model-page__money">—</span>
      </template>
      <template #cell-actions="{ row }: { row: OfficialModel }">
        <el-button
          v-if="canUpdate && row.actions.update"
          link
          type="primary"
          :disabled="busyID !== null || syncing"
          @click="openEdit(row)"
        >
          {{ row.modelKind === 1 ? t('ai.officialModel.edit') : t('ai.officialModel.viewPrice') }}
        </el-button>
        <el-button
          v-if="canReset && row.modelKind === 1 && row.actions.reset"
          link
          type="warning"
          :disabled="busyID !== null || syncing"
          @click="resetPrice(row)"
        >
          {{ t('ai.officialModel.reset') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('ai.officialModel.empty')">
          <el-button
            v-if="canSync"
            type="primary"
            :icon="Refresh"
            :loading="syncing"
            @click="syncCatalog"
          >
            {{ t('ai.officialModel.sync') }}
          </el-button>
        </el-empty>
      </template>
    </AppTable>

    <PriceDialog
      v-model="dialogVisible"
      :model="editing"
      :submitting="submitting"
      :error="formError"
      @save="save"
    />
  </AppPage>
</template>

<style scoped lang="scss">
.ai-official-model-page {
  min-width: 0;

  &__retry {
    display: flex;
    justify-content: flex-end;
    margin: -8px 0 8px;
  }

  &__hint {
    color: var(--admin-text-muted);
    font-size: 12px;
  }

  &__model {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 3px;

    &-vendor {
      color: var(--admin-text-muted);
      font-size: 10px;
      font-weight: 650;
      letter-spacing: 0.04em;
      text-transform: uppercase;
    }

    strong,
    code {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    strong {
      color: var(--admin-text);
      font-size: 13px;
      font-weight: 650;
    }

    code {
      color: var(--admin-text-muted);
      font-family: var(--admin-font-data, ui-monospace, monospace);
      font-size: 11px;
    }
  }

  &__money {
    color: var(--el-color-primary);
    font-family: var(--admin-font-data, ui-monospace, monospace);
    font-size: 12px;
    font-weight: 650;
    white-space: nowrap;

    &--official {
      color: var(--admin-text-soft);
      font-weight: 500;
    }
  }

  &__video-prices {
    color: var(--admin-text-soft);
    font-family: var(--admin-font-data, ui-monospace, monospace);
    font-size: 11px;
    line-height: 1.6;
    white-space: normal;
  }
}
</style>
