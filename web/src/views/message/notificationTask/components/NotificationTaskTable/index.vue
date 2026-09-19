<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { NotificationTaskListItem } from '@/api/message/notificationTask'
import type { TableColumn, TablePaginationState } from '@/components/AppTable/types'
import { usePermissionStore } from '@/store/permission'

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
const columns = computed<TableColumn<NotificationTaskListItem>[]>(() => [
  { prop: 'title', label: t('notificationTask.title'), minWidth: 180 },
  { prop: 'platformId', label: t('notificationTask.platform'), width: 100 },
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
    <template #cell-actions="{ row }">
      <el-button
        v-if="can('message:notificationTask:detail')"
        :data-testid="`notification-task-detail-${row.id}`"
        link
        @click.stop="emit('detail', row)"
        >{{ t('notificationTask.detail') }}</el-button
      >
      <el-button
        v-if="row.status === 'draft' && can('message:notificationTask:update')"
        :data-testid="`notification-task-edit-${row.id}`"
        link
        @click.stop="emit('edit', row)"
        >{{ t('notificationTask.edit') }}</el-button
      >
      <el-button
        v-if="row.status === 'draft' && can('message:notificationTask:delete')"
        :data-testid="`notification-task-delete-${row.id}`"
        link
        type="danger"
        @click.stop="emit('remove', row)"
        >{{ t('notificationTask.delete') }}</el-button
      >
      <el-button
        v-if="row.status === 'draft' && can('message:notificationTask:submit')"
        :data-testid="`notification-task-submit-${row.id}`"
        link
        @click.stop="emit('command', row, 'submit')"
        >{{ t('notificationTask.submit') }}</el-button
      >
      <el-button
        v-if="
          ['scheduled', 'queued', 'processing'].includes(row.status) &&
          can('message:notificationTask:cancel')
        "
        :data-testid="`notification-task-cancel-${row.id}`"
        link
        @click.stop="emit('command', row, 'cancel')"
        >{{ t('notificationTask.cancel') }}</el-button
      >
      <el-button
        v-if="row.status !== 'draft' && can('message:notificationTask:copy')"
        :data-testid="`notification-task-copy-${row.id}`"
        link
        @click.stop="emit('command', row, 'copy')"
        >{{ t('notificationTask.copy') }}</el-button
      >
    </template>
  </AppTable>
</template>
