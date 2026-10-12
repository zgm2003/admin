<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { ConfigCacheStatus } from '@/api/system/configCacheStatus'
import { formatTime } from '@/utils/datetime'

defineOptions({ name: 'ConfigCacheStatusDetailDialog' })

const props = defineProps<{ row: ConfigCacheStatus | null }>()
const visible = defineModel<boolean>({ required: true })
const { t } = useI18n()

function displayTime(value: string | null): string {
  return value === null ? '-' : formatTime(value)
}
</script>

<template>
  <AppDialog v-model="visible" :title="t('configCacheStatus.details')" width="640px">
    <el-descriptions v-if="props.row !== null" :column="1" border>
      <el-descriptions-item :label="t('configCacheStatus.detailRawNamespace')">
        {{ props.row.namespace }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('configCacheStatus.detailRawScope')">
        {{ props.row.scopeKey }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('configCacheStatus.detailGeneration')">
        {{ props.row.generation }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('configCacheStatus.detailPendingCount')">
        {{ props.row.pendingCount }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('configCacheStatus.detailLatestAttempts')">
        {{ props.row.latestAttempts }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('configCacheStatus.detailLatestPublishedGeneration')">
        {{ props.row.latestPublishedGeneration ?? '-' }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('configCacheStatus.detailLatestPublishedAt')">
        {{ displayTime(props.row.latestPublishedAt) }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('configCacheStatus.detailUpdatedAt')">
        {{ displayTime(props.row.updatedAt) }}
      </el-descriptions-item>
    </el-descriptions>
  </AppDialog>
</template>
