<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { NotificationTask } from '@/api/message/notificationTask'

defineProps<{ task: NotificationTask }>()
const { t } = useI18n()
</script>

<template>
  <div class="notification-task-detail" data-testid="notification-task-readonly-detail">
    <dl>
      <dt>{{ t('notificationTask.title') }}</dt>
      <dd>{{ task.title }}</dd>
      <dt>{{ t('notificationTask.platform') }}</dt>
      <dd>{{ task.platformId }}</dd>
      <dt>{{ t('notificationTask.audienceLabel') }}</dt>
      <dd>{{ t(`notificationTask.audience.${task.audienceType}`) }}</dd>
      <dt>{{ t('notificationTask.statusLabel') }}</dt>
      <dd>{{ t(`notificationTask.status.${task.status}`) }}</dd>
      <dt>{{ t('notificationTask.generatedCount') }}</dt>
      <dd>{{ task.generatedCount }}</dd>
      <dt>{{ t('notificationTask.completedAt') }}</dt>
      <dd>{{ task.completedAt ?? '-' }}</dd>
    </dl>
    <div class="notification-task-detail__content" v-html="task.contentHtml" />
  </div>
</template>

<style scoped>
.notification-task-detail dl {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: 8px 16px;
  margin: 0 0 16px;
}
.notification-task-detail dt {
  color: var(--el-text-color-secondary);
}
.notification-task-detail dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.notification-task-detail__content {
  padding-top: 16px;
  border-top: 1px solid var(--el-border-color-lighter);
  line-height: 1.6;
  overflow-wrap: anywhere;
}
</style>
