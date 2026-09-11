<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import * as smsApi from '@/api/message/sms'
import { AppDialog } from '@/components/AppDialog'
import { AppSearch, type SearchField, type SearchFormModel } from '@/components/AppSearch'
import { AppTable, type TableColumn, type TablePaginationState } from '@/components/AppTable'
import { formatTime } from '@/utils/datetime'

export interface SmsLogFilter {
  platform: string
  toPhone: string
  scene: smsApi.SmsScene | ''
  status: smsApi.SmsStatus | ''
  timeRange: [string, string] | []
}

const props = defineProps<{
  logs: smsApi.SmsLog[]
  sceneOptions: Array<{ label: string; value: smsApi.SmsScene }>
  total: number
  page: number
  pageSize: number
  loading: boolean
  canDetail: boolean
}>()
const emit = defineEmits<{
  refresh: []
  search: [value: SmsLogFilter]
  pageChange: [value: TablePaginationState]
}>()
const { t } = useI18n()
const filter = ref<SmsLogFilter>(blankFilter())
const detail = ref<smsApi.SmsLogDetail | null>(null)
const detailVisible = ref(false)
const detailLoading = ref(false)
const searchModel = computed<SearchFormModel>({
  get: () => filter.value,
  set: (value) => {
    filter.value = toFilter(value)
  },
})
const statusOptions = computed(() => [
  { label: t('sms.status.pending'), value: 'pending' },
  { label: t('sms.status.sent'), value: 'sent' },
  { label: t('sms.status.failed'), value: 'failed' },
])
const searchFields = computed<SearchField[]>(() => [
  {
    key: 'platform',
    type: 'input',
    label: t('sms.platform'),
    placeholder: t('sms.platformPlaceholder'),
    width: 160,
    testId: 'sms-log-platform',
  },
  {
    key: 'toPhone',
    type: 'input',
    label: t('sms.phone'),
    placeholder: t('sms.logPhonePlaceholder'),
    width: 210,
    testId: 'sms-log-phone',
  },
  {
    key: 'scene',
    type: 'select-v2',
    label: t('sms.sceneLabel'),
    placeholder: t('sms.scenePlaceholder'),
    options: props.sceneOptions,
    width: 170,
    testId: 'sms-log-scene',
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('sms.statusLabel'),
    placeholder: t('sms.statusPlaceholder'),
    options: statusOptions.value,
    width: 140,
    testId: 'sms-log-status',
  },
  {
    key: 'timeRange',
    type: 'date-range',
    label: t('sms.sentAt'),
    placeholder: t('sms.timePlaceholder'),
    valueFormat: 'YYYY-MM-DDTHH:mm:ssZ',
    width: 360,
    testId: 'sms-log-time',
  },
])
const columns = computed<TableColumn<smsApi.SmsLog>[]>(() => [
  { prop: 'platform', label: t('sms.platform'), width: 120 },
  { key: 'scene', prop: 'scene', label: t('sms.sceneLabel'), minWidth: 150 },
  { prop: 'toPhoneHint', label: t('sms.phone'), width: 150 },
  { key: 'status', prop: 'status', label: t('sms.statusLabel'), width: 100 },
  { key: 'sentAt', prop: 'sentAt', label: t('sms.sentAt'), minWidth: 180 },
  { prop: 'errorSummary', label: t('sms.error'), minWidth: 180, overflowTooltip: true },
  {
    key: 'actions',
    prop: 'id',
    label: t('sms.actions'),
    width: 90,
    fixed: 'right',
    hidden: !props.canDetail,
  },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: props.page,
  pageSize: props.pageSize,
  total: props.total,
}))

function blankFilter(): SmsLogFilter {
  return { platform: '', toPhone: '', scene: '', status: '', timeRange: [] }
}

function toFilter(value: SearchFormModel): SmsLogFilter {
  return {
    platform: typeof value.platform === 'string' ? value.platform : '',
    toPhone: typeof value.toPhone === 'string' ? value.toPhone : '',
    scene:
      value.scene === 'login' ||
      value.scene === 'forget' ||
      value.scene === 'bind_phone' ||
      value.scene === 'change_password'
        ? value.scene
        : '',
    status:
      value.status === 'pending' || value.status === 'sent' || value.status === 'failed'
        ? value.status
        : '',
    timeRange:
      Array.isArray(value.timeRange) &&
      value.timeRange.length === 2 &&
      value.timeRange.every((item) => typeof item === 'string')
        ? [value.timeRange[0], value.timeRange[1]]
        : [],
  }
}

function search(value: SearchFormModel): void {
  const next = toFilter(value)
  filter.value = next
  emit('search', next)
}

function reset(value: SearchFormModel): void {
  const next = toFilter(value)
  filter.value = next
  emit('search', next)
}

async function showDetail(row: smsApi.SmsLog): Promise<void> {
  detailLoading.value = true
  try {
    detail.value = await smsApi.getSmsLogDetail(row.id)
    detailVisible.value = true
  } catch {
    // request.ts owns the API error notification.
  } finally {
    detailLoading.value = false
  }
}
</script>

<template>
  <div class="sms-log">
    <AppSearch
      v-model="searchModel"
      :fields="searchFields"
      :collapse-count="4"
      query-test-id="sms-log-query"
      reset-test-id="sms-log-reset"
      @query="search"
      @reset="reset"
    />
    <AppTable
      :columns="columns"
      :data="logs"
      :loading="loading"
      :pagination="pagination"
      :aria-label="t('sms.tab.logs')"
      :refresh-label="t('sms.refresh')"
      @refresh="emit('refresh')"
      @update:pagination="emit('pageChange', $event)"
    >
      <template #cell-scene="{ row }: { row: smsApi.SmsLog }">
        {{ sceneOptions.find((option) => option.value === row.scene)?.label ?? row.scene }}
      </template>
      <template #cell-status="{ row }: { row: smsApi.SmsLog }">
        <el-tag
          :type="row.status === 'failed' ? 'danger' : row.status === 'sent' ? 'success' : 'info'"
        >
          {{ t(`sms.status.${row.status}`) }}
        </el-tag>
      </template>
      <template #cell-sentAt="{ row }: { row: smsApi.SmsLog }">
        {{ row.sentAt === null ? '-' : formatTime(row.sentAt) }}
      </template>
      <template #cell-actions="{ row }: { row: smsApi.SmsLog }">
        <el-button
          :data-testid="`sms-log-detail-${row.id}`"
          text
          type="primary"
          :loading="detailLoading"
          @click="showDetail(row)"
        >
          {{ t('sms.detail') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('sms.noLogs')" />
      </template>
    </AppTable>

    <AppDialog
      v-model="detailVisible"
      :title="t('sms.logDetail')"
      width="min(620px, 94vw)"
      append-to-body
    >
      <el-descriptions v-if="detail" :column="1" border>
        <el-descriptions-item :label="t('sms.platform')">{{
          detail.log.platform
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('sms.sceneLabel')">{{
          detail.log.scene
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('sms.phone')">{{ detail.toPhone }}</el-descriptions-item>
        <el-descriptions-item :label="t('sms.verificationCode')">
          {{ detail.verificationCode || '-' }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('sms.verificationExpiresAt')">
          {{ detail.verificationExpiresAt ? formatTime(detail.verificationExpiresAt) : '-' }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('sms.requestId')">{{
          detail.log.requestId || '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('sms.serialNo')">{{
          detail.log.serialNo || '-'
        }}</el-descriptions-item>
        <el-descriptions-item :label="t('sms.error')">{{
          detail.log.errorSummary || '-'
        }}</el-descriptions-item>
      </el-descriptions>
    </AppDialog>
  </div>
</template>

<style scoped>
.sms-log {
  min-width: 0;
}
</style>
