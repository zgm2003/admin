<script setup lang="ts">
import type { Run } from '@/api/system/scheduler'
import type { StatusOption } from '@/types/option'
import { formatTime } from '@/utils/datetime'
import { resolveOptionLabel } from '@/views/system/scheduler/presentation'

defineProps<{ runs: Run[]; statuses: StatusOption[] }>()
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
        <el-tag :type="statuses.find((option) => option.value === run.status)?.tone ?? 'info'">{{
          resolveOptionLabel(run.status, statuses)
        }}</el-tag>
        <span>{{ run.workerId }} · #{{ run.attemptNo }}</span>
        <span v-if="run.errorMessage" class="run-error">{{ run.errorMessage }}</span>
      </el-space>
    </el-timeline-item>
  </el-timeline>
  <el-empty v-else :description="$t('scheduler.noRuns')" />
</template>

<style scoped lang="scss">
.run-error {
  color: var(--el-color-danger);
}
</style>
