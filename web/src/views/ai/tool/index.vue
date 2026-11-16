<!-- 页面：AI 内置工具 -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'

import * as api from '@/api/ai/tool'
import type { AiTool } from '@/api/ai/tool'
import { usePermissionStore } from '@/store/permission'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'

defineOptions({ name: 'AIToolPage' })

const { t } = useI18n()
const access = usePermissionStore()
const rows = ref<AiTool[]>([])
const loading = ref(false)
const loadError = ref('')
const busyCode = ref('')
let alive = true

const canList = computed(() => access.hasPermission('ai:tool:list'))
const canStatus = computed(() => access.hasPermission('ai:tool:status'))
const pagination = computed<TablePaginationState>(() => ({
  currentPage: 1,
  pageSize: Math.max(rows.value.length, 10),
  total: rows.value.length,
}))
const columns = computed<TableColumn<AiTool>[]>(() => [
  { key: 'tool', label: t('ai.tool.tool'), minWidth: 230 },
  { key: 'description', label: t('ai.tool.description'), minWidth: 360 },
  { key: 'status', label: t('ai.tool.status'), width: 100 },
  { key: 'updatedAt', label: t('ai.tool.updatedAt'), width: 170 },
  { key: 'actions', label: t('common.actions'), width: 100, fixed: 'right' },
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

onMounted(load)
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
      <template #cell-tool="{ row }: { row: AiTool }">
        <div class="ai-tool-page__tool">
          <strong>{{ row.name }}</strong>
          <code>{{ row.code }}</code>
        </div>
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
        <el-button
          v-if="canStatus"
          link
          :type="row.status === 1 ? 'warning' : 'success'"
          :disabled="busyCode !== ''"
          @click="toggleStatus(row)"
        >
          {{ row.status === 1 ? t('ai.tool.disable') : t('ai.tool.enable') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('ai.tool.empty')" />
      </template>
    </AppTable>
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
}
</style>
