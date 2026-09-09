<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  deleteMailLog,
  deleteMailLogs,
  getMailLogDetail,
  type MailLog,
  type MailLogDetail,
  type MailTemplate,
} from '@/api/message/mail'
import { AppDialog } from '@/components/AppDialog'
import { AppTable, type TableColumn, type TablePaginationState } from '@/components/AppTable'
import { AppSearch, type SearchField, type SearchFormModel } from '@/components/AppSearch'
import { formatTime } from '@/utils/datetime'

export interface MailLogFilter {
  platform: string
  toEmail: string
  scene: string
  status: string
  timeRange: [string, string] | []
}

const props = defineProps<{
  logs: MailLog[]
  scenes: MailTemplate[]
  total: number
  page: number
  pageSize: number
  loading: boolean
  canDelete: boolean
}>()

const emit = defineEmits<{
  refresh: []
  pageChange: [value: TablePaginationState]
  search: [value: MailLogFilter]
}>()
const { t } = useI18n()
const selected = ref<MailLog[]>([])
const detail = ref<MailLogDetail | null>(null)
const detailVisible = ref(false)
const filter = ref<MailLogFilter>(blankFilter())
const selectedCount = computed(() => selected.value.length)
const searchModel = computed<SearchFormModel>({
  get: () => filter.value,
  set: (value) => {
    filter.value = toFilter(value)
  },
})
const sceneOptions = computed(() =>
  props.scenes.map((scene) => ({ label: scene.name, value: scene.scene })),
)
const statusLabels: Record<string, string> = {
  pending: 'mail.statusPending',
  sent: 'mail.statusSent',
  failed: 'mail.statusFailed',
}
const searchFields = computed<SearchField[]>(() => [
  {
    key: 'platform',
    type: 'input',
    label: t('mail.platform'),
    placeholder: t('mail.platformFilterPlaceholder'),
    width: 160,
    testId: 'mail-log-platform',
  },
  {
    key: 'toEmail',
    type: 'input',
    label: t('mail.recipient'),
    placeholder: t('mail.recipient'),
    width: 220,
    testId: 'mail-log-email',
  },
  {
    key: 'scene',
    type: 'select-v2',
    label: t('mail.scene'),
    options: sceneOptions.value,
    width: 170,
    testId: 'mail-log-scene',
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('mail.status'),
    options: [
      { label: t('mail.statusPending'), value: 'pending' },
      { label: t('mail.statusSent'), value: 'sent' },
      { label: t('mail.statusFailed'), value: 'failed' },
    ],
    width: 130,
    testId: 'mail-log-status',
  },
  {
    key: 'timeRange',
    type: 'date-range',
    label: t('mail.timeRange'),
    placeholder: t('mail.timeRange'),
    valueFormat: 'YYYY-MM-DDTHH:mm:ssZ',
    rangeSeparator: '-',
    width: 420,
  },
])
const columns = computed<TableColumn<MailLog>[]>(() => [
  { key: 'platform', prop: 'platform', label: t('mail.platform'), width: 110 },
  { key: 'username', prop: 'username', label: t('mail.associatedUser'), width: 130 },
  { prop: 'toEmail', label: t('mail.recipient'), minWidth: 210, overflowTooltip: true },
  { key: 'scene', prop: 'scene', label: t('mail.scene'), width: 150 },
  { key: 'status', prop: 'status', label: t('mail.status'), width: 110 },
  { key: 'latency', prop: 'latencyMs', label: t('mail.latency'), width: 110 },
  { key: 'sentAt', prop: 'sentAt', label: t('mail.sentAt'), minWidth: 180 },
  { key: 'actions', prop: 'id', label: t('mail.actions'), width: 200, fixed: 'right' },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: props.page,
  pageSize: props.pageSize,
  total: props.total,
}))

function blankFilter(): MailLogFilter {
  return { platform: '', toEmail: '', scene: '', status: '', timeRange: [] }
}

function toFilter(value: SearchFormModel): MailLogFilter {
  return {
    platform: typeof value.platform === 'string' ? value.platform : '',
    toEmail: typeof value.toEmail === 'string' ? value.toEmail : '',
    scene: typeof value.scene === 'string' ? value.scene : '',
    status: typeof value.status === 'string' ? value.status : '',
    timeRange: Array.isArray(value.timeRange) ? (value.timeRange as [string, string] | []) : [],
  }
}

function select(rows: MailLog[]): void {
  selected.value = rows
}

function platformText(value: string): string {
  return value === '' ? '-' : value
}

function usernameText(value: string): string {
  return value === '' ? '-' : value
}

function statusText(value: string): string {
  const key = statusLabels[value]
  return key === undefined ? value : t(key)
}

const sceneNames = computed(() =>
  Object.fromEntries(props.scenes.map((scene) => [scene.scene, scene.name])),
)

function sceneText(value: string): string {
  return sceneNames.value[value] ?? value
}

function search(value: SearchFormModel): void {
  emit('search', toFilter(value))
}

function reset(value: SearchFormModel): void {
  emit('search', toFilter(value))
}

async function inspect(row: MailLog): Promise<void> {
  try {
    detail.value = await getMailLogDetail(row.id)
    detailVisible.value = true
  } catch {
    // request.ts owns API error notifications.
  }
}

async function remove(row: MailLog): Promise<void> {
  try {
    await ElMessageBox.confirm(t('mail.deleteLogConfirm'))
    await deleteMailLog(row.id)
    ElMessage.success(t('mail.deleted'))
    emit('refresh')
  } catch {
    // ElMessageBox cancellation and request errors are handled by their respective layers.
  }
}

async function removeSelected(): Promise<void> {
  if (!selected.value.length) return
  try {
    await ElMessageBox.confirm(t('mail.deleteLogsConfirm'))
    await deleteMailLogs(selected.value.map((item) => item.id))
    selected.value = []
    ElMessage.success(t('mail.deleted'))
    emit('refresh')
  } catch {
    // ElMessageBox cancellation and request errors are handled by their respective layers.
  }
}
</script>

<template>
  <div class="table-tab">
    <AppSearch
      v-model="searchModel"
      :fields="searchFields"
      :collapse-count="3"
      query-test-id="mail-log-search"
      reset-test-id="mail-log-reset"
      @query="search"
      @reset="reset"
    />
    <AppTable
      :columns="columns"
      :data="logs"
      :loading="loading"
      :selectable="canDelete"
      :pagination="pagination"
      :aria-label="t('mail.logsTab')"
      :refresh-label="t('mail.refresh')"
      @refresh="emit('refresh')"
      @selection-change="select"
      @update:pagination="(next: TablePaginationState) => emit('pageChange', next)"
    >
      <template #toolbar-left>
        <el-button
          v-if="canDelete"
          data-testid="mail-log-batch-delete"
          type="danger"
          :disabled="selectedCount === 0"
          @click="removeSelected"
        >
          {{ t('mail.batchDelete') }}
        </el-button>
      </template>
      <template #cell-platform="{ row }: { row: MailLog }">
        {{ platformText(row.platform) }}
      </template>
      <template #cell-username="{ row }: { row: MailLog }">
        {{ usernameText(row.username) }}
      </template>
      <template #cell-scene="{ row }: { row: MailLog }">
        {{ sceneText(row.scene) }}
      </template>
      <template #cell-status="{ row }: { row: MailLog }">
        <el-tag
          :type="row.status === 'sent' ? 'success' : row.status === 'failed' ? 'danger' : 'warning'"
          effect="plain"
          >{{ statusText(row.status) }}</el-tag
        >
      </template>
      <template #cell-latency="{ row }: { row: MailLog }">{{ row.latencyMs }} ms</template>
      <template #cell-sentAt="{ row }: { row: MailLog }">{{
        row.sentAt === null ? '-' : formatTime(row.sentAt)
      }}</template>
      <template #cell-actions="{ row }: { row: MailLog }">
        <el-button text type="primary" @click="inspect(row)">
          {{ t('mail.detail') }}
        </el-button>
        <el-button v-if="canDelete" text type="danger" @click="remove(row)">
          {{ t('mail.delete') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('mail.noLogs')" />
      </template>
    </AppTable>
    <AppDialog v-model="detailVisible" :title="t('mail.logDetail')" width="520px">
      <el-descriptions v-if="detail" :column="1" border>
        <el-descriptions-item :label="t('mail.platform')">{{
          platformText(detail.log.platform)
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('mail.associatedUser')">{{
          usernameText(detail.log.username)
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('mail.recipient')">{{
          detail.log.toEmail
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('mail.scene')">{{
          sceneText(detail.log.scene)
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('mail.status')">{{
          statusText(detail.log.status)
        }}</el-descriptions-item>
        <el-descriptions-item label="Request ID"
          ><code>{{ detail.log.requestId || '-' }}</code></el-descriptions-item
        >
        <el-descriptions-item label="Message ID"
          ><code>{{ detail.log.messageId || '-' }}</code></el-descriptions-item
        >
        <el-descriptions-item :label="t('mail.verificationCode')"
          ><strong>{{ detail.verificationCode || '-' }}</strong></el-descriptions-item
        >
        <el-descriptions-item :label="t('mail.expiresAt')">{{
          detail.verificationExpiresAt === null ? '-' : formatTime(detail.verificationExpiresAt)
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('mail.error')">{{
          detail.log.errorSummary || '-'
        }}</el-descriptions-item>
      </el-descriptions>
    </AppDialog>
  </div>
</template>

<style scoped>
.table-tab {
  min-width: 0;
}
</style>
