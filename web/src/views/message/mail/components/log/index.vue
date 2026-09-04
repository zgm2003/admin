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
} from '@/api/message/mail'
import { AppTable, type TableColumn, type TablePaginationState } from '@/components/AppTable'
import { formatTime } from '@/utils/datetime'

const props = defineProps<{
  logs: MailLog[]
  total: number
  page: number
  pageSize: number
  loading: boolean
  canDelete: boolean
}>()

const emit = defineEmits<{ refresh: []; pageChange: [value: TablePaginationState] }>()
const { t } = useI18n()
const selected = ref<MailLog[]>([])
const detail = ref<MailLogDetail | null>(null)
const detailVisible = ref(false)
const selectedCount = computed(() => selected.value.length)
const columns = computed<TableColumn<MailLog>[]>(() => [
  { prop: 'toEmail', label: t('mail.recipient'), minWidth: 220, overflowTooltip: true },
  { prop: 'scene', label: t('mail.scene'), width: 150 },
  { key: 'status', prop: 'status', label: t('mail.status'), width: 110 },
  { key: 'latency', prop: 'latencyMs', label: t('mail.latency'), width: 120 },
  { key: 'sentAt', prop: 'sentAt', label: t('mail.sentAt'), minWidth: 190 },
  { key: 'actions', prop: 'id', label: t('mail.actions'), width: 200, fixed: 'right' },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: props.page,
  pageSize: props.pageSize,
  total: props.total,
}))

function select(rows: MailLog[]): void {
  selected.value = rows
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
      <template #cell-status="{ row }: { row: MailLog }">
        <el-tag
          :type="row.status === 'sent' ? 'success' : row.status === 'failed' ? 'danger' : 'warning'"
          effect="plain"
          >{{ row.status }}</el-tag
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
    <el-drawer v-model="detailVisible" :title="t('mail.logDetail')" size="min(480px, 94vw)">
      <el-descriptions v-if="detail" :column="1" border>
        <el-descriptions-item :label="t('mail.recipient')">{{
          detail.log.toEmail
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('mail.scene')">{{ detail.log.scene }}</el-descriptions-item>
        <el-descriptions-item :label="t('mail.status')">{{
          detail.log.status
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
    </el-drawer>
  </div>
</template>

<style scoped>
.table-tab {
  min-width: 0;
}
</style>
