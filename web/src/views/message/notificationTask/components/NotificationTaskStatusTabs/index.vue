<script setup lang="ts">
import { computed } from 'vue'
import type { TabPaneName } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  notificationTaskStatusMetadata,
  type NotificationTaskStatus,
} from '@/api/message/notificationTask'

const model = defineModel<NotificationTaskStatus | ''>({ required: true })
const emit = defineEmits<{ change: [value: NotificationTaskStatus | ''] }>()
const { t } = useI18n()

const tabs = computed(() => [
  { value: '' as const, label: t('notificationTask.statusAll') },
  ...notificationTaskStatusMetadata.map((status) => ({
    value: status.value,
    label: t(status.i18nKey),
  })),
])

function changeStatus(value: TabPaneName): void {
  const status =
    (typeof value === 'number' &&
      notificationTaskStatusMetadata.some((item) => item.value === value)) ||
    (typeof value === 'string' &&
      /^\d+$/.test(value) &&
      notificationTaskStatusMetadata.some((item) => item.value === Number(value)))
      ? (Number(value) as NotificationTaskStatus)
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
