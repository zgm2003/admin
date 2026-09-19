<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { NotificationTask, NotificationTaskStatus } from '@/api/message/notificationTask'
import { formatTime } from '@/utils/datetime'

const props = defineProps<{ task: NotificationTask }>()
const { t } = useI18n()

type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'

const statusTagTypes: Record<NotificationTaskStatus, TagType> = {
  draft: 'info',
  scheduled: 'warning',
  queued: 'primary',
  processing: 'primary',
  completed: 'success',
  failed: 'danger',
  canceled: 'info',
}
const generatedText = computed(() =>
  props.task.status === 'draft'
    ? t('notificationTask.notGenerated')
    : t('notificationTask.generatedValue', { count: props.task.generatedCount }),
)
const audienceText = computed(() => {
  const label = t(`notificationTask.audience.${props.task.audienceType}`)
  return props.task.audienceType === 'platform'
    ? label
    : `${label} · ${t('notificationTask.targetCount', { count: props.task.targetIds.length })}`
})

function displayTime(value: string | null): string {
  return value === null ? '-' : formatTime(value)
}
</script>

<template>
  <article class="notification-task-detail" data-testid="notification-task-readonly-detail">
    <header class="notification-task-detail__summary">
      <div class="notification-task-detail__identity">
        <h2 data-testid="notification-task-detail-title">{{ task.title }}</h2>
        <div class="notification-task-detail__tags">
          <el-tag
            :type="statusTagTypes[task.status]"
            effect="light"
            data-testid="notification-task-detail-status"
          >
            {{ t(`notificationTask.status.${task.status}`) }}
          </el-tag>
          <el-tag v-if="task.priority === 'urgent'" type="danger" effect="plain">
            {{ t('notification.urgent') }}
          </el-tag>
        </div>
      </div>
      <div class="notification-task-detail__metric">
        <strong data-testid="notification-task-detail-generated">{{ generatedText }}</strong>
        <span>{{ t('notificationTask.generationResult') }}</span>
      </div>
    </header>

    <el-descriptions class="notification-task-detail__meta" :column="2" border>
      <el-descriptions-item :label="t('notificationTask.platform')">
        <span data-testid="notification-task-detail-platform">{{ task.platformName }}</span>
      </el-descriptions-item>
      <el-descriptions-item :label="t('notificationTask.audienceLabel')">
        {{ audienceText }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('notificationTask.variantLabel')">
        {{ t(`notificationTask.variant.${task.variant}`) }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('notificationTask.priorityLabel')">
        {{ t(`notification.${task.priority}`) }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('notificationTask.scheduledAt')">
        {{
          task.scheduledAt === null
            ? t('notificationTask.sendImmediately')
            : displayTime(task.scheduledAt)
        }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('notificationTask.createdAt')">
        {{ displayTime(task.createdAt) }}
      </el-descriptions-item>
      <el-descriptions-item
        v-if="task.submittedAt !== null"
        :label="t('notificationTask.submittedAt')"
      >
        {{ displayTime(task.submittedAt) }}
      </el-descriptions-item>
      <el-descriptions-item
        v-if="task.completedAt !== null"
        :label="t('notificationTask.completedAt')"
      >
        {{ displayTime(task.completedAt) }}
      </el-descriptions-item>
      <el-descriptions-item
        v-if="task.linkType !== 'none'"
        :label="t('notificationTask.link')"
        :span="2"
      >
        <span class="notification-task-detail__link">{{ task.link }}</span>
      </el-descriptions-item>
    </el-descriptions>

    <el-alert
      v-if="task.failureMessage"
      class="notification-task-detail__failure"
      :title="task.failureMessage"
      type="error"
      :closable="false"
      show-icon
    />

    <section class="notification-task-detail__body">
      <h3>{{ t('notificationTask.content') }}</h3>
      <div
        class="notification-task-detail__content"
        data-testid="notification-task-detail-content"
        v-html="task.contentHtml"
      />
    </section>
  </article>
</template>

<style scoped>
.notification-task-detail {
  min-width: 0;
}

.notification-task-detail__summary {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  padding: 2px 0 18px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.notification-task-detail__identity {
  min-width: 0;
}

.notification-task-detail__identity h2 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 18px;
  font-weight: 600;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.notification-task-detail__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.notification-task-detail__metric {
  display: flex;
  min-width: 132px;
  flex-direction: column;
  align-items: flex-end;
  padding-left: 24px;
  border-left: 1px solid var(--el-border-color-lighter);
}

.notification-task-detail__metric strong {
  color: var(--el-text-color-primary);
  font-size: 22px;
  font-weight: 650;
  line-height: 1.4;
}

.notification-task-detail__metric span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.notification-task-detail__meta {
  margin-top: 18px;
}

.notification-task-detail__meta :deep(.el-descriptions__label) {
  width: 116px;
  color: var(--el-text-color-secondary);
  font-weight: 500;
}

.notification-task-detail__meta :deep(.el-descriptions__content),
.notification-task-detail__link {
  overflow-wrap: anywhere;
}

.notification-task-detail__failure {
  margin-top: 18px;
}

.notification-task-detail__body {
  padding-top: 18px;
}

.notification-task-detail__body h3 {
  margin: 0 0 10px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  font-weight: 500;
}

.notification-task-detail__content {
  min-height: 72px;
  padding: 14px 16px;
  border-left: 3px solid var(--el-color-primary-light-5);
  border-radius: 0 6px 6px 0;
  background: var(--el-fill-color-lighter);
  color: var(--el-text-color-primary);
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.notification-task-detail__content :deep(> :first-child) {
  margin-top: 0;
}

.notification-task-detail__content :deep(> :last-child) {
  margin-bottom: 0;
}

@media (max-width: 640px) {
  .notification-task-detail__summary {
    flex-direction: column;
    gap: 14px;
  }

  .notification-task-detail__metric {
    width: 100%;
    min-width: 0;
    align-items: flex-start;
    padding: 12px 0 0;
    border-top: 1px solid var(--el-border-color-lighter);
    border-left: 0;
  }

  .notification-task-detail__meta :deep(.el-descriptions__label) {
    width: 92px;
  }
}
</style>
