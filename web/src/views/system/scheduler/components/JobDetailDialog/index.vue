<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Job, Run } from '@/api/system/scheduler'
import { listRuns } from '@/api/system/scheduler'
import RunTimeline from '../RunTimeline/index.vue'

const props = defineProps<{ modelValue: boolean; job: Job | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const runs = ref<Run[]>([])
const loading = ref(false)
watch(
  () => [props.modelValue, props.job?.id] as const,
  async ([visible]) => {
    if (!visible || props.job === null) return
    loading.value = true
    try {
      runs.value = await listRuns(props.job.id)
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)
</script>

<template>
  <AppDialog
    :model-value="modelValue"
    :title="$t('scheduler.jobDetail')"
    width="680px"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-descriptions v-if="job" :column="2" border>
      <el-descriptions-item :label="$t('scheduler.id')">{{ job.id }}</el-descriptions-item>
      <el-descriptions-item :label="$t('scheduler.taskType')">{{
        job.taskType
      }}</el-descriptions-item>
      <el-descriptions-item :label="$t('scheduler.status')">{{ job.status }}</el-descriptions-item>
      <el-descriptions-item :label="$t('scheduler.attempt')"
        >{{ job.attemptCount }}/{{ job.maxAttempts }}</el-descriptions-item
      >
      <el-descriptions-item :label="$t('scheduler.createdAt')" :span="2">{{
        job.createdAt
      }}</el-descriptions-item>
      <el-descriptions-item v-if="job.lastError" :label="$t('scheduler.error')" :span="2">{{
        job.lastError
      }}</el-descriptions-item>
    </el-descriptions>
    <el-divider>{{ $t('scheduler.runs') }}</el-divider>
    <div v-loading="loading"><RunTimeline :runs="runs" /></div>
  </AppDialog>
</template>
