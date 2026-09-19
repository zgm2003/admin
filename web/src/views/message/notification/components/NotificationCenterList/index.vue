<script setup lang="ts">
import {
  ArrowUpRight,
  Check,
  CircleAlert,
  CircleCheck,
  CircleX,
  Info,
  Trash2,
} from 'lucide-vue-next'
import type { Component } from 'vue'
import { useI18n } from 'vue-i18n'

import type { NotificationItem, NotificationVariant } from '@/api/message/notification'
import { formatTime } from '@/utils/datetime'

defineProps<{
  items: NotificationItem[]
  loading: boolean
  nextBeforeId: number | null
  canRead: boolean
  canDelete: boolean
}>()
const emit = defineEmits<{
  read: [item: NotificationItem]
  remove: [item: NotificationItem]
  open: [item: NotificationItem]
  loadMore: []
}>()
const { t } = useI18n()

type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'
const variantTagTypes: Record<NotificationVariant, TagType> = {
  info: 'primary',
  success: 'success',
  warning: 'warning',
  error: 'danger',
}
const variantIcons: Record<NotificationVariant, Component> = {
  info: Info,
  success: CircleCheck,
  warning: CircleAlert,
  error: CircleX,
}
</script>

<template>
  <div class="notification-center__list">
    <article
      v-for="item in items"
      :key="item.id"
      :class="['notification-center__item', { 'is-unread': !item.isRead }]"
      :data-testid="`notification-item-${item.id}`"
    >
      <div :class="['notification-center__signal', `notification-center__signal--${item.variant}`]">
        <component :is="variantIcons[item.variant]" :size="19" :stroke-width="1.8" />
      </div>
      <div class="notification-center__main">
        <div class="notification-center__heading">
          <h2>{{ item.title }}</h2>
          <span
            v-if="!item.isRead"
            class="notification-center__unread"
            :title="t('notification.unread')"
            :aria-label="t('notification.unread')"
          />
        </div>
        <div class="notification-center__meta">
          <el-tag
            :data-testid="`notification-variant-${item.id}`"
            :type="variantTagTypes[item.variant]"
            effect="plain"
            size="small"
          >
            {{ t(`notification.variant.${item.variant}`) }}
          </el-tag>
          <el-tag
            :data-testid="`notification-priority-${item.id}`"
            :type="item.priority === 'urgent' ? 'danger' : 'info'"
            effect="plain"
            size="small"
          >
            {{ t(`notification.priority.${item.priority}`) }}
          </el-tag>
          <time :data-testid="`notification-published-${item.id}`" :datetime="item.publishedAt">
            {{ formatTime(item.publishedAt) }}
          </time>
        </div>
        <div class="notification-center__body" v-html="item.contentHtml" />
        <el-button
          v-if="item.linkType !== 'none'"
          class="notification-center__link"
          :icon="ArrowUpRight"
          link
          type="primary"
          @click="emit('open', item)"
        >
          {{ t('notification.openLink') }}
        </el-button>
      </div>
      <div class="notification-center__actions">
        <el-tooltip
          v-if="!item.isRead && canRead"
          :content="t('notification.markRead')"
          placement="top"
        >
          <el-button
            :aria-label="t('notification.markRead')"
            :data-testid="`notification-read-${item.id}`"
            :icon="Check"
            circle
            text
            type="primary"
            @click="emit('read', item)"
          />
        </el-tooltip>
        <el-tooltip v-if="canDelete" :content="t('notification.delete')" placement="top">
          <el-button
            :aria-label="t('notification.delete')"
            :data-testid="`notification-delete-${item.id}`"
            :icon="Trash2"
            circle
            text
            type="danger"
            @click="emit('remove', item)"
          />
        </el-tooltip>
      </div>
    </article>
    <div v-if="nextBeforeId !== null" class="notification-center__load-more">
      <el-button :loading="loading" @click="emit('loadMore')">
        {{ t('notification.loadMore') }}
      </el-button>
    </div>
  </div>
</template>

<style scoped>
.notification-center__list {
  min-height: 0;
}

.notification-center__item {
  position: relative;
  display: grid;
  min-height: 132px;
  grid-template-columns: 38px minmax(0, 1fr) auto;
  gap: 14px;
  padding: 18px 12px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  transition: background-color 160ms ease;
}

.notification-center__item.is-unread {
  background: var(--el-color-primary-light-9);
}

.notification-center__item.is-unread::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: var(--el-color-primary);
  content: '';
}

.notification-center__signal {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 7px;
}

.notification-center__signal--info {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.notification-center__signal--success {
  background: var(--el-color-success-light-9);
  color: var(--el-color-success);
}

.notification-center__signal--warning {
  background: var(--el-color-warning-light-9);
  color: var(--el-color-warning);
}

.notification-center__signal--error {
  background: var(--el-color-danger-light-9);
  color: var(--el-color-danger);
}

.notification-center__main {
  min-width: 0;
}

.notification-center__heading {
  display: flex;
  align-items: center;
  gap: 8px;
}

.notification-center__heading h2 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 15px;
  font-weight: 650;
  line-height: 1.45;
  letter-spacing: 0;
  overflow-wrap: anywhere;
}

.notification-center__unread {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--el-color-primary);
}

.notification-center__meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 7px;
  margin-top: 7px;
}

.notification-center__meta time {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}

.notification-center__body {
  margin-top: 10px;
  color: var(--el-text-color-regular);
  font-size: 14px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.notification-center__body :deep(> :first-child) {
  margin-top: 0;
}

.notification-center__body :deep(> :last-child) {
  margin-bottom: 0;
}

.notification-center__link {
  margin-top: 8px;
}

.notification-center__actions {
  display: flex;
  align-items: flex-start;
  gap: 2px;
}

.notification-center__load-more {
  display: flex;
  justify-content: center;
  padding: 16px 0 4px;
}

@media (max-width: 768px) {
  .notification-center__item {
    grid-template-columns: 32px minmax(0, 1fr);
    gap: 10px;
    padding: 16px 8px;
  }

  .notification-center__signal {
    width: 32px;
    height: 32px;
  }

  .notification-center__actions {
    grid-column: 2;
    justify-content: flex-end;
  }
}
</style>
