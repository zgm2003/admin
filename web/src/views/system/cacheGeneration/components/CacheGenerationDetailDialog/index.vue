<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { CacheGeneration } from '@/api/system/cacheGeneration'
import { formatTime } from '@/utils/datetime'

defineOptions({ name: 'CacheGenerationDetailDialog' })

const props = defineProps<{ row: CacheGeneration | null }>()
const visible = defineModel<boolean>({ required: true })
const { t } = useI18n()

function displayTime(value: string | null): string {
  return value === null ? '-' : formatTime(value)
}
</script>

<template>
  <AppDialog v-model="visible" :title="t('cacheGeneration.details')" width="640px">
    <el-descriptions v-if="props.row !== null" :column="1" border>
      <el-descriptions-item :label="t('cacheGeneration.detailRawNamespace')">
        {{ props.row.namespace }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('cacheGeneration.detailRawScope')">
        {{ props.row.scopeKey }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('cacheGeneration.detailGeneration')">
        {{ props.row.generation }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('cacheGeneration.detailPendingCount')">
        {{ props.row.pendingCount }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('cacheGeneration.detailLatestAttempts')">
        {{ props.row.latestAttempts }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('cacheGeneration.detailLatestPublishedGeneration')">
        {{ props.row.latestPublishedGeneration ?? '-' }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('cacheGeneration.detailLatestPublishedAt')">
        {{ displayTime(props.row.latestPublishedAt) }}
      </el-descriptions-item>
      <el-descriptions-item :label="t('cacheGeneration.detailUpdatedAt')">
        {{ displayTime(props.row.updatedAt) }}
      </el-descriptions-item>
    </el-descriptions>
  </AppDialog>
</template>
