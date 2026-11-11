<!-- 页面：AI 供应商配置 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'

import * as api from '@/api/ai/provider'
import type { AiProvider, ProviderOptions, ProviderQuery } from '@/api/ai/provider'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { usePermissionStore } from '@/store/permission'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import ProviderDialog from './components/ProviderDialog/index.vue'
import type { ProviderForm } from './components/ProviderDialog/types'
import ProviderModelDialog from './components/ProviderModelDialog/index.vue'

const { t, locale } = useI18n()
const access = usePermissionStore()
const {
  options,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<ProviderOptions>(api.getProviderOptions, () => ({
  vendors: [],
  protocols: [],
  statuses: [],
}))

const rows = ref<AiProvider[]>([])
const total = ref(0)
const query = ref<ProviderQuery>({ page: 1, pageSize: 20 })
const filters = ref<SearchFormModel<{ keyword: string; vendor: number | ''; status: number | '' }>>(
  {
    keyword: '',
    vendor: '',
    status: '',
  },
)
const loading = ref(false)
const loadError = ref('')
const busyID = ref<number | null>(null)
const dialogVisible = ref(false)
const editing = ref<AiProvider | null>(null)
const modelDialogVisible = ref(false)
const modelProvider = ref<AiProvider | null>(null)
const submitting = ref(false)
const formError = ref('')
let sequence = 0
let alive = true

const canList = computed(() => access.hasPermission('ai:provider:list'))
const canCreate = computed(() => access.hasPermission('ai:provider:create'))
const canUpdate = computed(() => access.hasPermission('ai:provider:update'))
const canStatus = computed(() => access.hasPermission('ai:provider:status'))
const canDelete = computed(() => access.hasPermission('ai:provider:delete'))
const canModels = computed(() => access.hasPermission('ai:provider:model:list'))
const canTest = computed(() => access.hasPermission('ai:provider:test'))
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const searchFields = computed<
  SearchField<{ keyword: string; vendor: number | ''; status: number | '' }>[]
>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('ai.provider.keyword'),
    placeholder: t('ai.provider.keywordPlaceholder'),
    resetValue: '',
    width: 240,
  },
  {
    key: 'vendor',
    type: 'select-v2',
    label: t('ai.provider.vendor'),
    placeholder: t('ai.provider.allVendors'),
    resetValue: '',
    options: options.value.vendors,
    width: 160,
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('ai.provider.status'),
    placeholder: t('ai.provider.allStatuses'),
    resetValue: '',
    options: options.value.statuses,
    width: 140,
  },
])
const columns = computed<TableColumn<AiProvider>[]>(() => [
  { key: 'provider', label: t('ai.provider.provider'), minWidth: 290 },
  { key: 'vendor', label: t('ai.provider.vendor'), width: 100 },
  { key: 'protocol', label: t('ai.provider.protocol'), width: 140 },
  { key: 'tokenRate', label: t('ai.provider.tokenRateMultiplier'), width: 110 },
  { key: 'apiKey', label: t('ai.provider.apiKey'), width: 110 },
  { key: 'status', label: t('ai.provider.status'), width: 90 },
  { key: 'updatedAt', label: t('ai.provider.updatedAt'), width: 170 },
  { key: 'actions', label: t('common.actions'), width: 280, fixed: 'right' },
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
    const result = await api.getProviders(query.value)
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = t('ai.provider.requestFailed')
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
    ...(filters.value.status !== '' ? { status: filters.value.status } : {}),
  }
  void load()
}

function reset(): void {
  filters.value = { keyword: '', vendor: '', status: '' }
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

function openCreate(): void {
  if (!canCreate.value) return
  editing.value = null
  formError.value = ''
  dialogVisible.value = true
}

function openEdit(row: AiProvider): void {
  if (!canUpdate.value || !row.actions.update) return
  editing.value = row
  formError.value = ''
  dialogVisible.value = true
}

function openModels(row: AiProvider): void {
  if (!canModels.value) return
  modelProvider.value = row
  modelDialogVisible.value = true
}

async function save(payload: ProviderForm): Promise<void> {
  if (submitting.value) return
  submitting.value = true
  formError.value = ''
  try {
    if (editing.value === null) {
      if (!canCreate.value) return
      await api.createProvider({
        name: payload.name,
        vendor: payload.vendor,
        protocol: payload.protocol,
        baseUrl: payload.baseUrl,
        apiKey: payload.apiKey,
        timeoutMs: payload.timeoutMs,
        sortOrder: payload.sortOrder,
        tokenRateMultiplier: payload.tokenRateMultiplier,
        status: payload.status,
      })
    } else {
      if (!canUpdate.value) return
      await api.updateProvider(editing.value.id, {
        name: payload.name,
        vendor: payload.vendor,
        protocol: payload.protocol,
        baseUrl: payload.baseUrl,
        ...(payload.apiKey ? { apiKey: payload.apiKey } : {}),
        timeoutMs: payload.timeoutMs,
        sortOrder: payload.sortOrder,
        tokenRateMultiplier: payload.tokenRateMultiplier,
        status: payload.status,
      })
    }
    dialogVisible.value = false
    ElNotification.success({
      title: editing.value === null ? t('ai.provider.created') : t('ai.provider.saved'),
    })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(row: AiProvider): Promise<void> {
  if (!canStatus.value || !row.actions.status || busyID.value !== null) return
  const nextStatus = row.status === 1 ? 0 : 1
  const action = nextStatus === 1 ? t('ai.provider.enable') : t('ai.provider.disable')
  try {
    await ElMessageBox.confirm(
      t('ai.provider.statusConfirm', { action, name: row.name }),
      t('confirm.title'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  busyID.value = row.id
  try {
    await api.updateProviderStatus(row.id, nextStatus)
    ElNotification.success({ title: t('ai.provider.statusSaved') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyID.value = null
  }
}

async function remove(row: AiProvider): Promise<void> {
  if (!canDelete.value || !row.actions.delete || busyID.value !== null) return
  try {
    await ElMessageBox.confirm(
      t('ai.provider.deleteConfirm', { name: row.name }),
      t('confirm.title'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  busyID.value = row.id
  try {
    await api.deleteProvider(row.id)
    ElNotification.success({ title: t('ai.provider.deleted') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyID.value = null
  }
}

function formatDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
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
})
</script>

<template>
  <AppPage class="ai-provider-page">
    <AppSearch
      v-model="filters"
      class="management-page__filters"
      :fields="searchFields"
      @query="search"
      @reset="reset"
    />
    <div v-if="optionsError" class="ai-provider-page__retry">
      <el-button text @click="reloadOptions">{{ t('common.retry') }}</el-button>
    </div>

    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: AiProvider) => String(row.id)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('ai.provider.title')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #toolbar-left>
        <el-button
          v-if="canCreate"
          type="primary"
          :icon="Plus"
          :disabled="submitting || busyID !== null"
          data-testid="ai-provider-create"
          @click="openCreate"
        >
          {{ t('ai.provider.create') }}
        </el-button>
      </template>
      <template #toolbar-right>
        <span class="ai-provider-page__hint">{{ t('ai.provider.hint') }}</span>
      </template>
      <template #cell-provider="{ row }: { row: AiProvider }">
        <div class="ai-provider-page__identity">
          <strong>{{ row.name }}</strong>
          <span>{{ row.baseUrl }}</span>
        </div>
      </template>
      <template #cell-vendor="{ row }: { row: AiProvider }">
        <span>{{ row.vendorName }}</span>
      </template>
      <template #cell-protocol="{ row }: { row: AiProvider }">
        <span>{{ row.protocolName }}</span>
      </template>
      <template #cell-tokenRate="{ row }: { row: AiProvider }">
        <el-tag type="warning" effect="plain">{{ row.tokenRateMultiplier }}x</el-tag>
      </template>
      <template #cell-apiKey="{ row }: { row: AiProvider }">
        <el-tag :type="row.apiKeyConfigured ? 'success' : 'info'" effect="plain">
          {{ row.apiKeyConfigured ? t('ai.provider.keyConfigured') : t('ai.provider.keyMissing') }}
        </el-tag>
      </template>
      <template #cell-timeout="{ row }: { row: AiProvider }">
        <span class="ai-provider-page__muted">{{ row.timeoutMs }} ms</span>
      </template>
      <template #cell-status="{ row }: { row: AiProvider }">
        <el-tag :type="row.status === 1 ? 'success' : 'info'" effect="plain">
          {{ row.statusName }}
        </el-tag>
      </template>
      <template #cell-updatedAt="{ row }: { row: AiProvider }">
        <span class="ai-provider-page__muted">{{ formatDate(row.updatedAt) }}</span>
      </template>
      <template #cell-actions="{ row }: { row: AiProvider }">
        <el-button
          v-if="canModels"
          link
          type="primary"
          :disabled="busyID !== null"
          :title="t('ai.provider.modelsHint')"
          @click="openModels(row)"
        >
          {{ t('ai.provider.models') }}
        </el-button>
        <el-button
          v-if="canUpdate && row.actions.update"
          link
          type="primary"
          :disabled="busyID !== null"
          @click="openEdit(row)"
        >
          {{ t('common.edit') }}
        </el-button>
        <el-button
          v-if="canStatus && row.actions.status"
          link
          :type="row.status === 1 ? 'warning' : 'success'"
          :disabled="busyID !== null"
          @click="toggleStatus(row)"
        >
          {{ row.status === 1 ? t('ai.provider.disable') : t('ai.provider.enable') }}
        </el-button>
        <el-button
          v-if="canDelete && row.actions.delete"
          link
          type="danger"
          :disabled="busyID !== null"
          @click="remove(row)"
        >
          {{ t('common.delete') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('ai.provider.empty')">
          <el-button v-if="canCreate" type="primary" :icon="Plus" @click="openCreate">
            {{ t('ai.provider.create') }}
          </el-button>
        </el-empty>
      </template>
    </AppTable>

    <ProviderDialog
      v-model="dialogVisible"
      :model="editing"
      :options="options"
      :submitting="submitting"
      :can-test="canTest"
      :error="formError"
      @save="save"
    />

    <ProviderModelDialog
      v-if="modelProvider"
      v-model="modelDialogVisible"
      :provider="modelProvider"
    />
  </AppPage>
</template>

<style scoped lang="scss">
.ai-provider-page {
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

  &__identity {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 3px;

    strong {
      overflow: hidden;
      color: var(--admin-text);
      font-size: 13px;
      font-weight: 650;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    span {
      overflow: hidden;
      color: var(--admin-text-muted);
      font-size: 11px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  &__muted {
    color: var(--admin-text-muted);
    font-size: 12px;
  }
}
</style>
