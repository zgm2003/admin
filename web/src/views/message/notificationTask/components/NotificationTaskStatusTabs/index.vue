<script setup lang="ts">
import { computed } from 'vue'
import type { TabPaneName } from 'element-plus'
import { useI18n } from 'vue-i18n'

import type { NotificationTaskStatus } from '@/api/message/notificationTask'
import type { NotificationTaskAdminOptions } from '@/api/message/notificationTaskOptions'

const model = defineModel<NotificationTaskStatus | ''>({ required: true })
const emit = defineEmits<{ change: [value: NotificationTaskStatus | ''] }>()
const { t } = useI18n()
const props = defineProps<{ statuses: NotificationTaskAdminOptions['statuses'] }>()

const tabs = computed(() => [
  { value: '' as const, label: t('notificationTask.statusAll') },
  ...props.statuses,
])

function changeStatus(value: TabPaneName): void {
  const parsed = value === '' ? '' : Number(value)
  const status = parsed === '' || Number.isFinite(parsed) ? parsed : ''
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
