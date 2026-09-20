<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Run } from '@/api/system/scheduler'
import { formatTime } from '@/utils/datetime'
import { getRunStatusKey } from '@/views/system/scheduler/presentation'

const { t } = useI18n()
defineProps<{ runs: Run[] }>()
</script>

<template>
  <el-timeline v-if="runs.length > 0">
    <el-timeline-item
      v-for="run in runs"
      :key="run.id"
      :timestamp="formatTime(run.startedAt)"
      placement="top"
    >
      <el-space direction="vertical" alignment="start" size="small">
        <el-tag
          :type="
            run.status === 'succeeded' ? 'success' : run.status === 'failed' ? 'danger' : 'warning'
          "
          >{{ t(getRunStatusKey(run.status)) }}</el-tag
        >
        <span>{{ run.workerId }} · #{{ run.attemptNo }}</span>
        <span v-if="run.errorMessage" class="run-error">{{ run.errorMessage }}</span>
      </el-space>
    </el-timeline-item>
  </el-timeline>
  <el-empty v-else :description="$t('scheduler.noRuns')" />
</template>

<style scoped>
.run-error {
  color: var(--el-color-danger);
}
</style>
