<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CirclePlus, Delete, Edit, VideoPlay } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { useI18n } from 'vue-i18n'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import type { TableColumn } from '@/components/AppTable'
import {
  createSchedule,
  deleteSchedule,
  executeSchedule,
  getTaskOptions,
  listJobs,
  listSchedules,
  retryJob,
  setScheduleStatus,
  updateSchedule,
} from '@/api/system/scheduler'
import type { Job, JobStatus, Schedule, TaskOption } from '@/api/system/scheduler'
import JobDetailDialog from './components/JobDetailDialog/index.vue'
import {
  CRON_PRESETS,
  CUSTOM_CRON_VALUE,
  getCronLabelKey,
  getCronPresetValue,
  getJobStatusKey,
  getTriggerSourceKey,
  resolveTaskDisplayName,
} from './presentation'

const { t } = useI18n()
const access = usePermissionStore()
const schedules = ref<Schedule[]>([])
const jobs = ref<Job[]>([])
const options = ref<TaskOption[]>([])
const loading = ref(false)
const loadError = ref('')
const activeTab = ref<'schedules' | 'jobs'>('schedules')
const jobStatus = ref<'' | JobStatus>('')
const dialogVisible = ref(false)
const editing = ref<Schedule | null>(null)
const submitting = ref(false)
const form = ref({
  name: '',
  description: '',
  taskType: '',
  cronExpression: '*/5 * * * *',
  cronPreset: '*/5 * * * *',
  timezone: 'Asia/Shanghai',
  params: {} as Record<string, unknown>,
  isEnabled: true,
})
const detailVisible = ref(false)
const selectedJob = ref<Job | null>(null)
const canList = computed(() => access.hasPermission('system:scheduler:list'))
const canCreate = computed(() => access.hasPermission('system:scheduler:create'))
const canUpdate = computed(() => access.hasPermission('system:scheduler:update'))
const canStatus = computed(() => access.hasPermission('system:scheduler:status'))
const canDelete = computed(() => access.hasPermission('system:scheduler:delete'))
const canExecute = computed(() => access.hasPermission('system:scheduler:execute'))
const canRetry = computed(() => access.hasPermission('system:scheduler:retry'))
const cronPresetOptions = computed(() =>
  CRON_PRESETS.map((preset) => ({ value: preset.value, label: t(preset.labelKey) })),
)
const jobStatusOptions = computed(() => [
  { label: t('scheduler.all'), value: '' },
  { label: t('scheduler.completed'), value: 'completed' },
  { label: t('scheduler.failed'), value: 'failed' },
  { label: t('scheduler.running'), value: 'running' },
])
const scheduleColumns = computed<TableColumn<Schedule>[]>(() => [
  { prop: 'name', label: t('scheduler.name'), minWidth: 180 },
  { prop: 'taskType', label: t('scheduler.taskType'), minWidth: 220 },
  { prop: 'cronExpression', label: t('scheduler.cron'), width: 180 },
  { prop: 'timezone', label: t('scheduler.timezone'), width: 150 },
  { key: 'status', prop: 'id', label: t('scheduler.status'), width: 100 },
  { key: 'actions', prop: 'id', label: t('scheduler.actions'), width: 330 },
])
const jobColumns = computed<TableColumn<Job>[]>(() => [
  { prop: 'id', label: t('scheduler.id'), width: 90 },
  { prop: 'taskType', label: t('scheduler.taskType'), minWidth: 220 },
  { key: 'trigger', prop: 'triggerSource', label: t('scheduler.trigger'), width: 120 },
  { key: 'status', prop: 'status', label: t('scheduler.status'), width: 120 },
  { prop: 'attemptCount', label: t('scheduler.attempt'), width: 100 },
  { prop: 'createdAt', label: t('scheduler.createdAt'), width: 180 },
  { key: 'actions', prop: 'id', label: t('scheduler.actions'), width: 160 },
])
const state = computed<'loading' | 'error' | 'empty' | 'success'>(() =>
  loading.value
    ? 'loading'
    : loadError.value
      ? 'error'
      : (activeTab.value === 'schedules' ? schedules.value.length : jobs.value.length) === 0
        ? 'empty'
        : 'success',
)
const taskOptions = computed(() => options.value.filter((item) => item.adminCreatable))
function taskDisplayName(type: string): string {
  return resolveTaskDisplayName(type, options.value) || t('scheduler.taskTypeUnknown')
}
async function load(): Promise<void> {
  if (!canList.value) return
  loading.value = true
  loadError.value = ''
  try {
    if (options.value.length === 0) options.value = await getTaskOptions()
    if (activeTab.value === 'schedules') schedules.value = await listSchedules()
    else jobs.value = await listJobs(0, jobStatus.value || undefined)
  } catch {
    loadError.value = t('scheduler.loadFailed')
  } finally {
    loading.value = false
  }
}
function openCreate(): void {
  editing.value = null
  const first = taskOptions.value[0]
  form.value = {
    name: '',
    description: '',
    taskType: first?.type ?? '',
    cronExpression: '*/5 * * * *',
    cronPreset: '*/5 * * * *',
    timezone: 'Asia/Shanghai',
    params: first?.defaultParams ?? {},
    isEnabled: true,
  }
  dialogVisible.value = true
}
function openEdit(row: Schedule): void {
  editing.value = row
  form.value = {
    name: row.name,
    description: row.description,
    taskType: row.taskType,
    cronExpression: row.cronExpression,
    cronPreset: getCronPresetValue(row.cronExpression),
    timezone: row.timezone,
    params: { ...row.params },
    isEnabled: row.isEnabled,
  }
  dialogVisible.value = true
}
function changeCronPreset(value: string | number | boolean): void {
  const preset = String(value)
  form.value.cronPreset = preset
  if (preset !== CUSTOM_CRON_VALUE) form.value.cronExpression = preset
}
async function save(): Promise<void> {
  if (
    submitting.value ||
    form.value.name.trim() === '' ||
    form.value.cronExpression.trim() === '' ||
    form.value.timezone.trim() === ''
  )
    return
  submitting.value = true
  try {
    if (editing.value === null)
      await createSchedule({
        name: form.value.name,
        description: form.value.description,
        taskType: form.value.taskType,
        cronExpression: form.value.cronExpression,
        timezone: form.value.timezone,
        params: form.value.params,
        isEnabled: form.value.isEnabled,
      })
    else
      await updateSchedule(editing.value.id, {
        name: form.value.name,
        description: form.value.description,
        cronExpression: form.value.cronExpression,
        timezone: form.value.timezone,
        params: form.value.params,
      })
    dialogVisible.value = false
    await load()
    ElNotification.success({ title: t('scheduler.saved') })
  } finally {
    submitting.value = false
  }
}
async function toggle(row: Schedule): Promise<void> {
  if (!canStatus.value) return
  await setScheduleStatus(row.id, !row.isEnabled)
  await load()
}
async function remove(row: Schedule): Promise<void> {
  if (row.builtinKey || !canDelete.value) return
  await ElMessageBox.confirm(t('scheduler.deleteConfirm'), t('scheduler.deleteTitle'), {
    type: 'warning',
  })
  await deleteSchedule(row.id)
  await load()
  ElNotification.success({ title: t('scheduler.deleted') })
}
async function execute(row: Schedule): Promise<void> {
  if (!canExecute.value) return
  await executeSchedule(row.id)
  await load()
  ElNotification.success({ title: t('scheduler.executed') })
}
async function retry(row: Job): Promise<void> {
  if (!canRetry.value) return
  await retryJob(row.id)
  await load()
  ElNotification.success({ title: t('scheduler.retried') })
}
function openJob(row: Job): void {
  selectedJob.value = row
  detailVisible.value = true
}
function changeTab(value: string): void {
  activeTab.value = value === 'jobs' ? 'jobs' : 'schedules'
  void load()
}
function changeJobStatus(value: string | number | boolean): void {
  jobStatus.value = value === '' ? '' : (String(value) as JobStatus)
  void load()
}
onMounted(() => {
  void load()
})
</script>

<template>
  <AppPage class="scheduler-page">
    <el-tabs :model-value="activeTab" @update:model-value="changeTab">
      <el-tab-pane name="schedules" :label="t('scheduler.schedules')" />
      <el-tab-pane name="jobs" :label="t('scheduler.jobs')" />
    </el-tabs>
    <AppTable
      v-if="activeTab === 'schedules'"
      :columns="scheduleColumns"
      :data="schedules"
      :loading="loading"
      :result-state="state"
      :status-message="loadError"
      row-key="id"
      @refresh="load"
    >
      <template #toolbar-left
        ><el-button
          v-if="canCreate && taskOptions.length > 0"
          type="primary"
          :icon="CirclePlus"
          @click="openCreate"
          >{{ t('scheduler.create') }}</el-button
        ></template
      >
      <template #cell-status="{ row }: { row: Schedule }"
        ><el-tag :type="row.isEnabled ? 'success' : 'info'">{{
          row.isEnabled ? t('scheduler.enabled') : t('scheduler.disabled')
        }}</el-tag></template
      >
      <template #cell-taskType="{ row }: { row: Schedule }">
        <el-tooltip :content="row.taskType" placement="top">
          <span>{{ taskDisplayName(row.taskType) }}</span>
        </el-tooltip>
      </template>
      <template #cell-cronExpression="{ row }: { row: Schedule }">
        <el-tooltip :content="row.cronExpression" placement="top">
          <span>{{ t(getCronLabelKey(row.cronExpression)) }}</span>
        </el-tooltip>
      </template>
      <template #cell-actions="{ row }: { row: Schedule }"
        ><el-space
          ><el-button v-if="canUpdate" type="primary" link :icon="Edit" @click="openEdit(row)">{{
            t('scheduler.edit')
          }}</el-button
          ><el-button
            v-if="canStatus"
            :type="row.isEnabled ? 'warning' : 'success'"
            link
            @click="toggle(row)"
            >{{ row.isEnabled ? t('scheduler.disable') : t('scheduler.enable') }}</el-button
          ><el-button
            v-if="canExecute"
            type="success"
            link
            :icon="VideoPlay"
            @click="execute(row)"
            >{{ t('scheduler.execute') }}</el-button
          ><el-button
            v-if="canDelete && !row.builtinKey"
            type="danger"
            link
            :icon="Delete"
            @click="remove(row)"
            >{{ t('scheduler.delete') }}</el-button
          ></el-space
        ></template
      >
    </AppTable>
    <AppTable
      v-else
      :columns="jobColumns"
      :data="jobs"
      :loading="loading"
      :result-state="state"
      :status-message="loadError"
      row-key="id"
      @refresh="load"
    >
      <template #toolbar-left
        ><el-select-v2
          class="scheduler-job-status-filter"
          :model-value="jobStatus"
          :options="jobStatusOptions"
          @update:model-value="changeJobStatus"
      /></template>
      <template #cell-taskType="{ row }: { row: Job }">
        <el-tooltip :content="row.taskType" placement="top">
          <span>{{ taskDisplayName(row.taskType) }}</span>
        </el-tooltip>
      </template>
      <template #cell-trigger="{ row }: { row: Job }">
        <el-tooltip :content="row.triggerSource" placement="top">
          <span>{{ t(getTriggerSourceKey(row.triggerSource)) }}</span>
        </el-tooltip>
      </template>
      <template #cell-status="{ row }: { row: Job }"
        ><el-tag
          :type="
            row.status === 'completed'
              ? 'success'
              : row.status === 'failed'
                ? 'danger'
                : row.status === 'running'
                  ? 'warning'
                  : 'info'
          "
          >{{ t(getJobStatusKey(row.status)) }}</el-tag
        ></template
      >
      <template #cell-createdAt="{ row }: { row: Job }">
        <span>{{ formatTime(row.createdAt) }}</span>
      </template>
      <template #cell-actions="{ row }: { row: Job }"
        ><el-space
          ><el-button type="primary" link @click="openJob(row)">{{
            t('scheduler.detail')
          }}</el-button
          ><el-button
            v-if="canRetry && row.status === 'failed'"
            type="warning"
            link
            @click="retry(row)"
            >{{ t('scheduler.retry') }}</el-button
          ></el-space
        ></template
      >
    </AppTable>
    <AppDialog
      v-model="dialogVisible"
      :title="editing === null ? t('scheduler.create') : t('scheduler.edit')"
      width="600px"
      @close="dialogVisible = false"
    >
      <el-form label-position="top" @submit.prevent="save"
        ><el-form-item :label="t('scheduler.name')" required
          ><el-input v-model="form.name" /></el-form-item
        ><el-form-item :label="t('scheduler.description')"
          ><el-input v-model="form.description" /></el-form-item
        ><el-form-item :label="t('scheduler.taskType')" required
          ><el-select-v2
            v-model="form.taskType"
            :options="taskOptions.map((item) => ({ label: item.displayName, value: item.type }))"
            disabled /></el-form-item
        ><el-form-item :label="t('scheduler.cron')" required
          ><el-select-v2
            v-model="form.cronPreset"
            :options="cronPresetOptions"
            @update:model-value="changeCronPreset" />
          <el-input
            v-if="form.cronPreset === CUSTOM_CRON_VALUE"
            v-model="form.cronExpression"
            class="scheduler-cron-custom-input"
            :placeholder="t('scheduler.cronCustomPlaceholder')" /></el-form-item
        ><el-form-item :label="t('scheduler.timezone')" required
          ><el-input v-model="form.timezone" /></el-form-item
        ><template #footer
          ><el-button type="primary" :loading="submitting" @click="save">{{
            t('scheduler.save')
          }}</el-button></template
        ></el-form
      >
    </AppDialog>
    <JobDetailDialog v-model="detailVisible" :job="selectedJob" :task-options="options" />
  </AppPage>
</template>

<style scoped>
.scheduler-job-status-filter {
  width: 160px;
}

.scheduler-cron-custom-input {
  margin-top: 8px;
}

@media (max-width: 900px) {
  .scheduler-job-status-filter {
    width: 100%;
  }
}
</style>
