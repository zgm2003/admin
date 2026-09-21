<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  NotificationTaskStatus,
  notificationTaskStatusMetadata,
  type NotificationTaskListItem,
  type NotificationTaskStatus as NotificationTaskStatusValue,
} from '@/api/message/notificationTask'
import type { TableColumn, TablePaginationState } from '@/components/AppTable/types'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'

defineProps<{
  rows: NotificationTaskListItem[]
  loading: boolean
  errorMessage: string
  pagination: TablePaginationState
}>()
const emit = defineEmits<{
  refresh: []
  updatePagination: [value: TablePaginationState]
  create: []
  detail: [row: NotificationTaskListItem]
  edit: [row: NotificationTaskListItem]
  remove: [row: NotificationTaskListItem]
  command: [row: NotificationTaskListItem, action: 'submit' | 'cancel' | 'copy']
}>()
const access = usePermissionStore()
const { t } = useI18n()
const can = (code: string): boolean => access.hasPermission(code)
type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'
const statusTagTypes: Record<NotificationTaskStatusValue, TagType> = Object.fromEntries(
  notificationTaskStatusMetadata.map((status) => [status.value, status.tagType]),
) as Record<NotificationTaskStatusValue, TagType>
const statusTagType = (status: NotificationTaskStatusValue): TagType => statusTagTypes[status]
const statusI18nKey = (status: NotificationTaskStatusValue): string =>
  notificationTaskStatusMetadata.find((item) => item.value === status)?.i18nKey ?? ''
const displayTime = (value: string | null): string => (value === null ? '-' : formatTime(value))
const columns = computed<TableColumn<NotificationTaskListItem>[]>(() => [
  { prop: 'title', label: t('notificationTask.title'), minWidth: 180 },
  { prop: 'platformName', label: t('notificationTask.platform'), minWidth: 140 },
  { prop: 'audienceType', label: t('notificationTask.audienceLabel'), width: 110 },
  { prop: 'status', label: t('notificationTask.statusLabel'), width: 110 },
  { prop: 'generatedCount', label: t('notificationTask.generatedCount'), width: 120 },
  { prop: 'scheduledAt', label: t('notificationTask.scheduledAt'), minWidth: 170 },
  { prop: 'submittedAt', label: t('notificationTask.submittedAt'), minWidth: 170 },
  { prop: 'completedAt', label: t('notificationTask.completedAt'), minWidth: 170 },
  { key: 'actions', label: t('notificationTask.actions'), width: 300 },
])
</script>

<template>
  <AppTable
    :columns="columns"
    :data="rows"
    :loading="loading"
    :pagination="pagination"
    :result-state="errorMessage ? 'error' : rows.length === 0 ? 'empty' : 'success'"
    :status-message="errorMessage"
    @refresh="emit('refresh')"
    @update:pagination="emit('updatePagination', $event)"
  >
    <template #toolbar-right>
      <el-button
        v-if="can('message:notificationTask:create')"
        data-testid="notification-task-create"
        type="primary"
        @click="emit('create')"
        >{{ t('notificationTask.create') }}</el-button
      >
    </template>
    <template #cell-audienceType="{ row }">
      <span v-if="row.id !== undefined" :data-testid="`notification-task-audience-${row.id}`">
        {{ t(`notificationTask.audience.${row.audienceType}`) }}
      </span>
    </template>
    <template #cell-status="{ row }">
      <span v-if="row.id !== undefined" :data-testid="`notification-task-status-${row.id}`">
        <el-tag :type="statusTagType(row.status)" effect="light" size="small">
          {{ t(statusI18nKey(row.status)) }}
        </el-tag>
      </span>
    </template>
    <template #cell-generatedCount="{ row }">
      <span v-if="row.id !== undefined" :data-testid="`notification-task-generated-${row.id}`">
        {{
          row.status === NotificationTaskStatus.Draft
            ? t('notificationTask.notGenerated')
            : t('notificationTask.generatedValue', { count: row.generatedCount })
        }}
      </span>
    </template>
    <template #cell-scheduledAt="{ row }">
      <span v-if="row.id !== undefined" :data-testid="`notification-task-scheduled-${row.id}`">
        {{ displayTime(row.scheduledAt) }}
      </span>
    </template>
    <template #cell-submittedAt="{ row }">
      <span v-if="row.id !== undefined" :data-testid="`notification-task-submitted-${row.id}`">
        {{ displayTime(row.submittedAt) }}
      </span>
    </template>
    <template #cell-completedAt="{ row }">
      <span v-if="row.id !== undefined" :data-testid="`notification-task-completed-${row.id}`">
        {{ displayTime(row.completedAt) }}
      </span>
    </template>
    <template #cell-actions="{ row }">
      <el-button
        v-if="can('message:notificationTask:detail')"
        :data-testid="`notification-task-detail-${row.id}`"
        link
        type="primary"
        @click.stop="emit('detail', row)"
        >{{ t('notificationTask.detail') }}</el-button
      >
      <el-button
        v-if="row.status === NotificationTaskStatus.Draft && can('message:notificationTask:update')"
        :data-testid="`notification-task-edit-${row.id}`"
        link
        type="warning"
        @click.stop="emit('edit', row)"
        >{{ t('notificationTask.edit') }}</el-button
      >
      <el-button
        v-if="row.status === NotificationTaskStatus.Draft && can('message:notificationTask:delete')"
        :data-testid="`notification-task-delete-${row.id}`"
        link
        type="danger"
        @click.stop="emit('remove', row)"
        >{{ t('notificationTask.delete') }}</el-button
      >
      <el-button
        v-if="row.status === NotificationTaskStatus.Draft && can('message:notificationTask:submit')"
        :data-testid="`notification-task-submit-${row.id}`"
        link
        type="success"
        @click.stop="emit('command', row, 'submit')"
        >{{ t('notificationTask.submit') }}</el-button
      >
      <el-button
        v-if="
          [
            NotificationTaskStatus.Scheduled,
            NotificationTaskStatus.Queued,
            NotificationTaskStatus.Processing,
          ].includes(row.status) && can('message:notificationTask:cancel')
        "
        :data-testid="`notification-task-cancel-${row.id}`"
        link
        type="warning"
        @click.stop="emit('command', row, 'cancel')"
        >{{ t('notificationTask.cancel') }}</el-button
      >
      <el-button
        v-if="row.status !== NotificationTaskStatus.Draft && can('message:notificationTask:copy')"
        :data-testid="`notification-task-copy-${row.id}`"
        link
        type="primary"
        @click.stop="emit('command', row, 'copy')"
        >{{ t('notificationTask.copy') }}</el-button
      >
    </template>
  </AppTable>
</template>
