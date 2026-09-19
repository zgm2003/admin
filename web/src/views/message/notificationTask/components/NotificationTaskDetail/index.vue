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

    <dl class="notification-task-detail__meta">
      <div class="notification-task-detail__meta-item">
        <dt>{{ t('notificationTask.platform') }}</dt>
        <dd data-testid="notification-task-detail-platform">{{ task.platformName }}</dd>
      </div>
      <div class="notification-task-detail__meta-item">
        <dt>{{ t('notificationTask.audienceLabel') }}</dt>
        <dd>{{ audienceText }}</dd>
      </div>
      <div class="notification-task-detail__meta-item">
        <dt>{{ t('notificationTask.variantLabel') }}</dt>
        <dd>{{ t(`notificationTask.variant.${task.variant}`) }}</dd>
      </div>
      <div class="notification-task-detail__meta-item">
        <dt>{{ t('notificationTask.priorityLabel') }}</dt>
        <dd>{{ t(`notification.${task.priority}`) }}</dd>
      </div>
      <div class="notification-task-detail__meta-item">
        <dt>{{ t('notificationTask.scheduledAt') }}</dt>
        <dd>
          {{
            task.scheduledAt === null
              ? t('notificationTask.sendImmediately')
              : displayTime(task.scheduledAt)
          }}
        </dd>
      </div>
      <div class="notification-task-detail__meta-item">
        <dt>{{ t('notificationTask.createdAt') }}</dt>
        <dd>{{ displayTime(task.createdAt) }}</dd>
      </div>
      <div v-if="task.submittedAt !== null" class="notification-task-detail__meta-item">
        <dt>{{ t('notificationTask.submittedAt') }}</dt>
        <dd>{{ displayTime(task.submittedAt) }}</dd>
      </div>
      <div v-if="task.completedAt !== null" class="notification-task-detail__meta-item">
        <dt>{{ t('notificationTask.completedAt') }}</dt>
        <dd>{{ displayTime(task.completedAt) }}</dd>
      </div>
      <div
        v-if="task.linkType !== 'none'"
        class="notification-task-detail__meta-item notification-task-detail__meta-item--wide"
      >
        <dt>{{ t('notificationTask.link') }}</dt>
        <dd>{{ task.link }}</dd>
      </div>
    </dl>

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

.notification-task-detail__metric span,
.notification-task-detail__meta dt {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.notification-task-detail__meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px 28px;
  padding: 20px 0;
  margin: 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.notification-task-detail__meta-item {
  min-width: 0;
}

.notification-task-detail__meta-item--wide {
  grid-column: 1 / -1;
}

.notification-task-detail__meta dt {
  margin-bottom: 5px;
  line-height: 1.4;
}

.notification-task-detail__meta dd {
  margin: 0;
  color: var(--el-text-color-regular);
  font-size: 14px;
  line-height: 1.5;
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

  .notification-task-detail__meta {
    grid-template-columns: 1fr;
    gap: 14px;
  }
}
</style>
