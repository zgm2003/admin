<!-- 页面：AI 工具 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'

import * as api from '@/api/ai/tool'
import type { AiTool } from '@/api/ai/tool'
import { usePermissionStore } from '@/store/permission'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import ToolDialog from './components/ToolDialog/index.vue'
import type { ToolFormPayload } from './components/ToolDialog/types'

defineOptions({ name: 'AIToolPage' })

const { t } = useI18n()
const access = usePermissionStore()
const rows = ref<AiTool[]>([])
const loading = ref(false)
const loadError = ref('')
const busyCode = ref('')
const dialogVisible = ref(false)
const editing = ref<AiTool | null>(null)
const submitting = ref(false)
const formError = ref('')
let alive = true

const canList = computed(() => access.hasPermission('ai:tool:list'))
const canCreate = computed(() => access.hasPermission('ai:tool:create'))
const canUpdate = computed(() => access.hasPermission('ai:tool:update'))
const canStatus = computed(() => access.hasPermission('ai:tool:status'))
const canDelete = computed(() => access.hasPermission('ai:tool:delete'))
const pagination = computed<TablePaginationState>(() => ({
  currentPage: 1,
  pageSize: Math.max(rows.value.length, 10),
  total: rows.value.length,
}))
const columns = computed<TableColumn<AiTool>[]>(() => [
  { key: 'tool', label: t('ai.tool.tool'), minWidth: 250 },
  { key: 'kind', label: t('ai.tool.kind'), width: 110 },
  { key: 'description', label: t('ai.tool.description'), minWidth: 300 },
  { key: 'status', label: t('ai.tool.status'), width: 90 },
  { key: 'updatedAt', label: t('ai.tool.updatedAt'), width: 170 },
  { key: 'actions', label: t('common.actions'), width: 230, fixed: 'right' },
])

function state(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}

function formatDate(value: string): string {
  return new Date(value).toLocaleString()
}

async function load(): Promise<void> {
  if (!canList.value) return
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getTools()
    if (alive) rows.value = result
  } catch {
    if (alive) {
      rows.value = []
      loadError.value = t('ai.tool.requestFailed')
    }
  } finally {
    if (alive) loading.value = false
  }
}

function openCreate(): void {
  if (!canCreate.value) return
  editing.value = null
  formError.value = ''
  dialogVisible.value = true
}

function openEdit(row: AiTool): void {
  if (!canUpdate.value) return
  editing.value = row
  formError.value = ''
  dialogVisible.value = true
}

async function save(payload: ToolFormPayload): Promise<void> {
  if (submitting.value) return
  submitting.value = true
  formError.value = ''
  const input = {
    name: payload.name,
    description: payload.description,
    category: payload.category,
    method: payload.method,
    url: payload.url,
    timeoutMs: payload.timeoutMs,
    status: payload.status,
    sortOrder: payload.sortOrder,
    parameters: payload.parameters,
  }
  try {
    if (editing.value === null) {
      if (!canCreate.value) return
      await api.createTool({ code: payload.code, ...input })
      ElNotification.success({ title: t('ai.tool.created') })
    } else {
      if (!canUpdate.value) return
      await api.updateTool(editing.value.code, input)
      ElNotification.success({ title: t('ai.tool.saved') })
    }
    dialogVisible.value = false
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(row: AiTool): Promise<void> {
  const next = row.status === 1 ? 0 : 1
  try {
    await ElMessageBox.confirm(
      t(next === 1 ? 'ai.tool.enableConfirm' : 'ai.tool.disableConfirm', { name: row.name }),
      t('confirm.title'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  busyCode.value = row.code
  try {
    await api.updateToolStatus(row.code, next)
    ElNotification.success({ title: t('ai.tool.statusSaved') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyCode.value = ''
  }
}

async function remove(row: AiTool): Promise<void> {
  if (!canDelete.value || row.kind === 1) return
  try {
    await ElMessageBox.confirm(t('ai.tool.deleteConfirm', { name: row.name }), t('confirm.title'), {
      type: 'warning',
    })
  } catch {
    return
  }
  busyCode.value = row.code
  try {
    await api.deleteTool(row.code)
    ElNotification.success({ title: t('ai.tool.deleted') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyCode.value = ''
  }
}

onMounted(load)
onBeforeUnmount(() => {
  alive = false
})
</script>

<template>
  <AppPage class="ai-tool-page">
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: AiTool) => String(row.id)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('ai.tool.title')"
      @refresh="load"
    >
      <template #toolbar-left>
        <el-button
          v-if="canCreate"
          type="primary"
          :icon="Plus"
          :disabled="submitting || busyCode !== ''"
          data-testid="ai-tool-create"
          @click="openCreate"
        >
          {{ t('ai.tool.create') }}
        </el-button>
      </template>
      <template #cell-tool="{ row }: { row: AiTool }">
        <div class="ai-tool-page__tool">
          <strong>{{ row.name }}</strong>
          <code>{{ row.code }}</code>
        </div>
      </template>
      <template #cell-kind="{ row }: { row: AiTool }">
        <el-tag :type="row.kind === 1 ? 'info' : 'primary'" effect="plain">
          {{ row.kindName }}
        </el-tag>
      </template>
      <template #cell-description="{ row }: { row: AiTool }">
        <span class="ai-tool-page__muted">{{ row.description }}</span>
      </template>
      <template #cell-status="{ row }: { row: AiTool }">
        <el-tag :type="row.status === 1 ? 'success' : 'info'" effect="plain">
          {{ row.statusName }}
        </el-tag>
      </template>
      <template #cell-updatedAt="{ row }: { row: AiTool }">
        <span class="ai-tool-page__muted">{{ formatDate(row.updatedAt) }}</span>
      </template>
      <template #cell-actions="{ row }: { row: AiTool }">
        <div class="ai-tool-page__actions">
          <el-button v-if="canUpdate" link type="primary" @click="openEdit(row)">
            {{ t('common.edit') }}
          </el-button>
          <el-button
            v-if="canStatus"
            link
            :type="row.status === 1 ? 'warning' : 'success'"
            :disabled="busyCode !== ''"
            @click="toggleStatus(row)"
          >
            {{ row.status === 1 ? t('ai.tool.disable') : t('ai.tool.enable') }}
          </el-button>
          <el-button
            v-if="canDelete && row.kind === 2"
            link
            type="danger"
            :disabled="busyCode !== ''"
            @click="remove(row)"
          >
            {{ t('common.delete') }}
          </el-button>
        </div>
      </template>
      <template #empty>
        <el-empty :description="t('ai.tool.empty')">
          <el-button v-if="canCreate" type="primary" :icon="Plus" @click="openCreate">
            {{ t('ai.tool.create') }}
          </el-button>
        </el-empty>
      </template>
    </AppTable>

    <ToolDialog
      v-model="dialogVisible"
      :model="editing"
      :submitting="submitting"
      :error="formError"
      @save="save"
    />
  </AppPage>
</template>

<style scoped lang="scss">
.ai-tool-page {
  &__tool {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 3px;
  }

  &__tool strong {
    color: var(--admin-text);
    font-size: 13px;
    font-weight: 600;
  }

  &__tool code {
    overflow: hidden;
    color: #94a3b8;
    font-family: 'JetBrains Mono', monospace;
    font-size: 11px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__muted {
    color: var(--admin-text-muted);
    font-size: 12px;
    line-height: 1.6;
  }

  &__actions {
    display: flex;
    flex-wrap: nowrap;
    align-items: center;
    gap: 4px;
  }

  &__actions :deep(.el-button + .el-button) {
    margin-left: 0;
  }
}
</style>
