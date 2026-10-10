<script setup lang="ts">
import { Document } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import type { MediaFileItem } from '@/components/UpMedia/types'

defineProps<{
  items: MediaFileItem[]
  label: string
  disabled: boolean
  uploadDisabled: boolean
  loading: boolean
  clearable: boolean
  multiple: boolean
}>()
const emit = defineEmits<{
  select: []
  clear: [index: number]
  download: [key: string]
  retry: [key: string]
}>()
const { t } = useI18n()
</script>

<template>
  <div class="media-files">
    <div
      v-for="(item, index) in items"
      :key="item.objectKey"
      class="media-file"
      data-testid="up-media-file"
    >
      <el-icon class="media-file__icon"><Document /></el-icon>
      <div class="media-file__info">
        <strong>{{ label || item.objectKey.split('/').at(-1) }}</strong>
        <span v-if="item.failed" role="alert">{{ t('components.upMedia.resolveFailed') }}</span>
        <span v-else-if="item.pending">{{ t('components.upMedia.resolving') }}</span>
      </div>
      <div class="media-file__actions">
        <el-button v-if="item.failed" link type="primary" @click="emit('retry', item.objectKey)">{{
          t('components.upMedia.retry')
        }}</el-button>
        <el-button
          v-else
          data-testid="up-media-download"
          link
          type="primary"
          :disabled="item.pending || loading || !item.previewUrl"
          @click="emit('download', item.objectKey)"
          >{{ t('components.upMedia.download') }}</el-button
        >
        <el-button
          v-if="!multiple"
          link
          type="primary"
          :disabled="disabled || uploadDisabled || loading"
          @click="emit('select')"
          >{{ t('components.upMedia.replace') }}</el-button
        >
        <el-button
          v-if="clearable"
          data-testid="up-media-file-clear"
          link
          type="danger"
          :disabled="disabled || loading"
          @click="emit('clear', index)"
          >{{ t('components.upMedia.clearFile') }}</el-button
        >
      </div>
    </div>
    <el-button
      v-if="multiple || items.length === 0"
      data-testid="up-media-file-select"
      :disabled="disabled || uploadDisabled || loading"
      @click="emit('select')"
      >{{ t('components.upMedia.select') }}</el-button
    >
  </div>
</template>

<style scoped lang="scss">
.media-files {
  width: 100%;
  min-width: 0;
}
.media-file {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  &__icon {
    color: var(--el-color-primary);
    font-size: 24px;
  }
  &__info {
    display: flex;
    flex: 1;
    min-width: 0;
    flex-direction: column;
    gap: 4px;
  }
  &__info strong {
    overflow: hidden;
    font-size: 14px;
    font-weight: 500;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  &__info span {
    color: var(--el-text-color-secondary);
    font-size: 12px;
  }
  &__actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
  }
  &__actions .el-button {
    margin-left: 0;
  }
}
@media (max-width: 640px) {
  .media-file {
    flex-wrap: wrap;
    &__actions {
      width: 100%;
      padding-left: 36px;
    }
  }
}
</style>
