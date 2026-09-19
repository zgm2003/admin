<script setup lang="ts">
import { computed } from 'vue'
import type { TabPaneName } from 'element-plus'
import { useI18n } from 'vue-i18n'

import type { NotificationTaskStatus } from '@/api/message/notificationTask'

const model = defineModel<NotificationTaskStatus | ''>({ required: true })
const emit = defineEmits<{ change: [value: NotificationTaskStatus | ''] }>()
const { t } = useI18n()

const taskStatuses = [
  'draft',
  'scheduled',
  'queued',
  'processing',
  'completed',
  'failed',
  'canceled',
] as const
const tabs = computed(() => [
  { value: '' as const, label: t('notificationTask.statusAll') },
  ...taskStatuses.map((value) => ({
    value,
    label: t(`notificationTask.status.${value}`),
  })),
])

function changeStatus(value: TabPaneName): void {
  const status =
    typeof value === 'string' && taskStatuses.some((item) => item === value)
      ? (value as NotificationTaskStatus)
      : ''
  emit('change', status)
}
</script>

<template>
  <el-tabs
    v-model="model"
    class="notification-task-status-tabs"
    data-testid="notification-task-status-tabs"
    @tab-change="changeStatus"
  >
    <el-tab-pane
      v-for="tab in tabs"
      :key="tab.value || 'all'"
      :name="tab.value"
      :label="tab.label"
      :data-testid="`notification-task-status-${tab.value || 'all'}`"
    />
  </el-tabs>
</template>
