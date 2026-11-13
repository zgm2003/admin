<!-- 页面：AI 智能体配置 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'

import * as api from '@/api/ai/agent'
import type { AgentModelOptions, AgentOptions, AgentQuery, AiAgent } from '@/api/ai/agent'
import { requestObjectURL } from '@/api/storage/upload'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { usePermissionStore } from '@/store/permission'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import AgentDialog from './components/AgentDialog/index.vue'
import type { AgentForm } from './components/AgentDialog/types'

const { t, locale } = useI18n()
const access = usePermissionStore()
const {
  options,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<AgentOptions>(api.getAgentOptions, () => ({
  types: [],
  scenes: [],
  statuses: [],
}))
const {
  options: modelOptions,
  loading: modelsLoading,
  error: modelOptionsError,
  reload: reloadModelOptions,
} = useLocalizedOptions<AgentModelOptions>(api.getAgentModelOptions, () => ({
  providers: [],
}))

const rows = ref<AiAgent[]>([])
const avatarUrls = ref<Record<string, string>>({})
const total = ref(0)
const query = ref<AgentQuery>({ page: 1, pageSize: 20 })
const filters = ref<
  SearchFormModel<{ keyword: string; type: number | ''; scene: number | ''; status: number | '' }>
>({
  keyword: '',
  type: '',
  scene: '',
  status: '',
})
const loading = ref(false)
const loadError = ref('')
const busyID = ref<number | null>(null)
const dialogVisible = ref(false)
const editing = ref<AiAgent | null>(null)
const submitting = ref(false)
const formError = ref('')
let sequence = 0
let alive = true

const canList = computed(() => access.hasPermission('ai:agent:list'))
const canCreate = computed(() => access.hasPermission('ai:agent:create'))
const canUpdate = computed(() => access.hasPermission('ai:agent:update'))
const canStatus = computed(() => access.hasPermission('ai:agent:status'))
const canDelete = computed(() => access.hasPermission('ai:agent:delete'))
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const searchFields = computed<
  SearchField<{ keyword: string; type: number | ''; scene: number | ''; status: number | '' }>[]
>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('ai.agent.keyword'),
    placeholder: t('ai.agent.keywordPlaceholder'),
    resetValue: '',
    width: 240,
  },
  {
    key: 'type',
    type: 'select-v2',
    label: t('ai.agent.type'),
    placeholder: t('ai.agent.allTypes'),
    resetValue: '',
    options: options.value.types,
    width: 130,
  },
  {
    key: 'scene',
    type: 'select-v2',
    label: t('ai.agent.scene'),
    placeholder: t('ai.agent.allScenes'),
    resetValue: '',
    options: options.value.scenes,
    width: 140,
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('ai.agent.status'),
    placeholder: t('ai.agent.allStatuses'),
    resetValue: '',
    options: options.value.statuses,
    width: 130,
  },
])
const columns = computed<TableColumn<AiAgent>[]>(() => [
  { key: 'agent', label: t('ai.agent.agent'), minWidth: 220 },
  { key: 'type', label: t('ai.agent.type'), width: 90 },
  { key: 'scene', label: t('ai.agent.scene'), minWidth: 150 },
  { key: 'model', label: t('ai.agent.model'), minWidth: 240 },
  { key: 'status', label: t('ai.agent.status'), width: 90 },
  { key: 'updatedAt', label: t('ai.agent.updatedAt'), width: 170 },
  { key: 'actions', label: t('common.actions'), width: 260, fixed: 'right' },
])

async function hydrateAvatars(items: AiAgent[]): Promise<void> {
  const keys = [...new Set(items.map((item) => item.avatar).filter((key) => key !== ''))]
  await Promise.all(
    keys.map(async (key) => {
      if (avatarUrls.value[key]) return
      try {
        const result = await requestObjectURL(key)
        if (alive) avatarUrls.value = { ...avatarUrls.value, [key]: result.url }
      } catch {
        // A missing preview must not block the agent list.
      }
    }),
  )
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
    const result = await api.getAgents(query.value)
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
      void hydrateAvatars(result.list)
    }
  } catch {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = t('ai.agent.requestFailed')
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
    ...(filters.value.type !== '' ? { type: filters.value.type } : {}),
    ...(filters.value.scene !== '' ? { scene: filters.value.scene } : {}),
    ...(filters.value.status !== '' ? { status: filters.value.status } : {}),
  }
  void load()
}

function reset(): void {
  filters.value = { keyword: '', type: '', scene: '', status: '' }
  search()
}

function typeTagType(type: number): 'primary' | 'success' | 'warning' | 'info' {
  switch (type) {
    case 2:
      return 'success'
    case 3:
      return 'warning'
    case 4:
      return 'info'
    default:
      return 'primary'
  }
}

function updatePagination(value: TablePaginationState): void {
  query.value = {
    ...query.value,
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}

function retryOptions(): void {
  void reloadOptions()
  void reloadModelOptions()
}

function openCreate(): void {
  if (!canCreate.value) return
  editing.value = null
  formError.value = ''
  dialogVisible.value = true
}

function openEdit(row: AiAgent): void {
  if (!canUpdate.value || !row.actions.update) return
  editing.value = row
  formError.value = ''
  dialogVisible.value = true
}

async function save(payload: AgentForm): Promise<void> {
  if (submitting.value || payload.providerModelId === null) return
  const input = { ...payload, providerModelId: payload.providerModelId }
  submitting.value = true
  formError.value = ''
  try {
    if (editing.value === null) {
      if (!canCreate.value) return
      await api.createAgent(input)
    } else {
      if (!canUpdate.value) return
      await api.updateAgent(editing.value.id, input)
    }
    dialogVisible.value = false
    ElNotification.success({
      title: editing.value === null ? t('ai.agent.created') : t('ai.agent.saved'),
    })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(row: AiAgent): Promise<void> {
  if (!canStatus.value || !row.actions.status || busyID.value !== null) return
  const nextStatus = row.status === 1 ? 0 : 1
  const action = nextStatus === 1 ? t('ai.agent.enable') : t('ai.agent.disable')
  try {
    await ElMessageBox.confirm(
      t('ai.agent.statusConfirm', { action, name: row.name }),
      t('confirm.title'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  busyID.value = row.id
  try {
    await api.updateAgentStatus(row.id, nextStatus)
    ElNotification.success({ title: t('ai.agent.statusSaved') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyID.value = null
  }
}

async function remove(row: AiAgent): Promise<void> {
  if (!canDelete.value || !row.actions.delete || busyID.value !== null) return
  try {
    await ElMessageBox.confirm(
      t('ai.agent.deleteConfirm', { name: row.name }),
      t('confirm.title'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  busyID.value = row.id
  try {
    await api.deleteAgent(row.id)
    ElNotification.success({ title: t('ai.agent.deleted') })
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
  if (visible) {
    if (modelOptionsError.value && !modelsLoading.value) void reloadModelOptions()
    return
  }
  editing.value = null
  formError.value = ''
})
onBeforeUnmount(() => {
  alive = false
  sequence++
})
</script>

<template>
  <AppPage class="ai-agent-page">
    <AppSearch
      v-model="filters"
      class="management-page__filters"
      :fields="searchFields"
      @query="search"
      @reset="reset"
    />
    <div v-if="optionsError || modelOptionsError" class="ai-agent-page__retry">
      <span>{{ t('ai.agent.optionsFailed') }}</span>
      <el-button text :loading="modelsLoading" @click="retryOptions">
        {{ t('common.retry') }}
      </el-button>
    </div>

    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: AiAgent) => String(row.id)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('ai.agent.title')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #toolbar-left>
        <el-button
          v-if="canCreate"
          type="primary"
          :icon="Plus"
          :disabled="submitting || busyID !== null"
          data-testid="ai-agent-create"
          @click="openCreate"
        >
          {{ t('ai.agent.create') }}
        </el-button>
      </template>
      <template #toolbar-right>
        <span class="ai-agent-page__hint">{{ t('ai.agent.hint') }}</span>
      </template>
      <template #cell-agent="{ row }: { row: AiAgent }">
        <div class="ai-agent-page__agent-cell">
          <el-avatar :size="38" :src="avatarUrls[row.avatar]">
            {{ row.name.slice(0, 1) }}
          </el-avatar>
          <div class="ai-agent-page__identity">
            <strong>{{ row.name }}</strong>
            <span>{{ row.description || t('ai.agent.noDescription') }}</span>
          </div>
        </div>
      </template>
      <template #cell-type="{ row }: { row: AiAgent }">
        <el-tag :type="typeTagType(row.type)" effect="plain">{{ row.typeName }}</el-tag>
      </template>
      <template #cell-scene="{ row }: { row: AiAgent }">
        <span class="ai-agent-page__scene">{{ row.sceneName }}</span>
      </template>
      <template #cell-model="{ row }: { row: AiAgent }">
        <div class="ai-agent-page__identity">
          <strong>{{ row.modelName || t('ai.agent.unknownModel') }}</strong>
          <span>
            {{ row.providerName || t('ai.agent.unknownProvider') }} · {{ t('ai.agent.modelId') }}:
            {{ row.providerModelId }}
          </span>
          <el-tag v-if="!row.modelAvailable" type="warning" size="small" effect="plain">
            {{ t('ai.agent.modelUnavailableShort') }}
          </el-tag>
        </div>
      </template>
      <template #cell-status="{ row }: { row: AiAgent }">
        <el-tag :type="row.status === 1 ? 'success' : 'info'" effect="plain">
          {{ row.statusName }}
        </el-tag>
      </template>
      <template #cell-updatedAt="{ row }: { row: AiAgent }">
        <span class="ai-agent-page__muted">{{ formatDate(row.updatedAt) }}</span>
      </template>
      <template #cell-actions="{ row }: { row: AiAgent }">
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
          {{ row.status === 1 ? t('ai.agent.disable') : t('ai.agent.enable') }}
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
        <el-empty :description="t('ai.agent.empty')">
          <el-button v-if="canCreate" type="primary" :icon="Plus" @click="openCreate">
            {{ t('ai.agent.create') }}
          </el-button>
        </el-empty>
      </template>
    </AppTable>

    <AgentDialog
      v-model="dialogVisible"
      :model="editing"
      :options="options"
      :model-options="modelOptions.providers"
      :submitting="submitting"
      :error="formError"
      @save="save"
    />
  </AppPage>
</template>

<style scoped lang="scss">
.ai-agent-page {
  min-width: 0;

  &__agent-cell {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 10px;
  }

  &__retry {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    margin: -8px 0 8px;
    color: var(--admin-text-muted);
    font-size: 12px;
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
    text-align: left;

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
      line-height: 1.4;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .el-tag {
      align-self: flex-start;
    }
  }

  &__muted {
    color: var(--admin-text-muted);
    font-size: 12px;
  }

  &__scene {
    display: block;
    overflow: hidden;
    color: var(--admin-text);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}
</style>
