<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Job, Run, TaskOption } from '@/api/system/scheduler'
import { JobStatus } from '@/enums/scheduler'
import { listRuns } from '@/api/system/scheduler'
import { formatTime } from '@/utils/datetime'
import RunTimeline from '@/views/system/scheduler/components/RunTimeline/index.vue'
import {
  getJobStatusKey,
  getTriggerSourceKey,
  resolveTaskDisplayName,
} from '@/views/system/scheduler/presentation'

const props = defineProps<{
  modelValue: boolean
  job: Job | null
  taskOptions: TaskOption[]
}>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const { t } = useI18n()
const runs = ref<Run[]>([])
const loading = ref(false)
let loadSequence = 0
const taskDisplayName = computed(() => {
  if (props.job === null) return ''
  return (
    resolveTaskDisplayName(props.job.taskType, props.taskOptions) || t('scheduler.taskTypeUnknown')
  )
})
watch(
  () => [props.modelValue, props.job?.id] as const,
  async ([visible]) => {
    const sequence = ++loadSequence
    runs.value = []
    if (!visible || props.job === null) {
      loading.value = false
      return
    }
    const jobID = props.job.id
    loading.value = true
    try {
      const result = await listRuns(jobID)
      if (sequence === loadSequence && props.modelValue && props.job?.id === jobID) {
        runs.value = result
      }
    } catch {
      // request.ts owns the API error notification.
    } finally {
      if (sequence === loadSequence) loading.value = false
    }
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  loadSequence += 1
})
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
      <el-descriptions-item :label="$t('scheduler.taskType')">
        <el-tooltip :content="job.taskType" placement="top">
          <span>{{ taskDisplayName }}</span>
        </el-tooltip>
      </el-descriptions-item>
      <el-descriptions-item :label="$t('scheduler.trigger')">
        <el-tooltip :content="job.triggerSource" placement="top">
          <span>{{ $t(getTriggerSourceKey(job.triggerSource)) }}</span>
        </el-tooltip>
      </el-descriptions-item>
      <el-descriptions-item :label="$t('scheduler.status')">
        <el-tag
          :type="
            job.status === JobStatus.failed
              ? 'danger'
              : job.status === JobStatus.completed
                ? 'success'
                : 'info'
          "
          >{{ $t(getJobStatusKey(job.status)) }}</el-tag
        >
      </el-descriptions-item>
      <el-descriptions-item :label="$t('scheduler.attempt')"
        >{{ job.attemptCount }}/{{ job.maxAttempts }}</el-descriptions-item
      >
      <el-descriptions-item :label="$t('scheduler.createdAt')">{{
        formatTime(job.createdAt)
      }}</el-descriptions-item>
      <el-descriptions-item :label="$t('scheduler.scheduledAt')">{{
        formatTime(job.scheduledAt)
      }}</el-descriptions-item>
      <el-descriptions-item :label="$t('scheduler.availableAt')">{{
        formatTime(job.availableAt)
      }}</el-descriptions-item>
      <el-descriptions-item v-if="job.completedAt" :label="$t('scheduler.completedAt')">{{
        formatTime(job.completedAt)
      }}</el-descriptions-item>
      <el-descriptions-item v-if="job.lastError" :label="$t('scheduler.error')" :span="2">{{
        job.lastError
      }}</el-descriptions-item>
    </el-descriptions>
    <el-divider>{{ $t('scheduler.runs') }}</el-divider>
    <div v-loading="loading"><RunTimeline :runs="runs" /></div>
  </AppDialog>
</template>
