<!-- 页面：定时任务 -->
<script setup lang="ts">
import { YesNo } from '@/enums/yesNo'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { CirclePlus } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { useI18n } from 'vue-i18n'
import { usePermissionStore } from '@/store/permission'
import { formatTime } from '@/utils/datetime'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import {
  createSchedule,
  deleteSchedule,
  executeSchedule,
  getSchedulerOptions,
  listJobs,
  listSchedules,
  retryJob,
  setScheduleStatus,
  updateSchedule,
} from '@/api/system/scheduler'
import type { Job, Schedule, JobStatus, SchedulerOptions } from '@/api/system/scheduler'
import JobDetailDialog from './components/JobDetailDialog/index.vue'
import {
  CUSTOM_CRON_VALUE,
  getCronPresetValue,
  resolveOptionLabel,
  resolveTaskDisplayName,
} from './presentation'

const { t, locale } = useI18n()
const access = usePermissionStore()
const schedules = ref<Schedule[]>([])
const jobs = ref<Job[]>([])
const options = ref<SchedulerOptions | null>(null)
let loadSequence = 0
const loading = ref(false)
const loadError = ref('')
const selectedPlanID = ref<number | ''>('')
const visibleSchedules = computed(() =>
  selectedPlanID.value === ''
    ? schedules.value
    : schedules.value.filter((row) => row.id === selectedPlanID.value),
)
const planOptions = computed(() =>
  schedules.value.map((row) => ({ value: row.id, label: row.name })),
)
const schedulesHaveMore = ref(false)
const jobsVisible = ref(false)
const selectedSchedule = ref<Schedule | null>(null)
const jobsLoading = ref(false)
const jobsError = ref('')
const jobsPage = ref(1)
const jobsPageSize = ref(20)
const jobsTotal = ref(0)
const jobsPagination = computed<TablePaginationState>(() => ({
  currentPage: jobsPage.value,
  pageSize: jobsPageSize.value,
  total: jobsTotal.value,
}))
let jobsSequence = 0
const jobStatus = ref<'' | JobStatus>('')
const dialogVisible = ref(false)
const editing = ref<Schedule | null>(null)
const submitting = ref(false)
type ScheduleForm = Parameters<typeof createSchedule>[0] & { cronPreset: string }
const form = ref<ScheduleForm>({
  name: '',
  description: '',
  taskType: '',
  cronExpression: '',
  cronPreset: CUSTOM_CRON_VALUE,
  timezone: '',
  params: {} as Record<string, unknown>,
  status: YesNo.Yes,
})
const detailVisible = ref(false)
const selectedJob = ref<Job | null>(null)
const canList = computed(() => access.hasPermission('system:scheduler:list'))
const canDetail = computed(() => access.hasPermission('system:scheduler:detail'))
const canCreate = computed(() => access.hasPermission('system:scheduler:create'))
const canUpdate = computed(() => access.hasPermission('system:scheduler:update'))
const canStatus = computed(() => access.hasPermission('system:scheduler:status'))
const canDelete = computed(() => access.hasPermission('system:scheduler:delete'))
const canExecute = computed(() => access.hasPermission('system:scheduler:execute'))
const canRetry = computed(() => access.hasPermission('system:scheduler:retry'))
const cronPresetOptions = computed(() => [
  ...(options.value?.cronPresets ?? []),
  { value: CUSTOM_CRON_VALUE, label: t('scheduler.cronPreset.custom') },
])
const jobStatusOptions = computed(() => options.value?.jobStatuses ?? [])
const jobStatusPresentation = (value: number) =>
  jobStatusOptions.value.find((option) => option.value === value)
const scheduleStatuses = computed(() => options.value?.scheduleStatuses ?? [])
const scheduleStatusPresentation = (value: number) =>
  scheduleStatuses.value.find((option) => option.value === value)
const triggerSources = computed(() => options.value?.triggerSources ?? [])
const cronPresets = computed(() => options.value?.cronPresets ?? [])
const scheduleColumns = computed<TableColumn<Schedule>[]>(() => [
  { prop: 'name', label: t('scheduler.name'), minWidth: 180 },
  { prop: 'taskType', label: t('scheduler.taskType'), minWidth: 220 },
  { prop: 'cronExpression', label: t('scheduler.cron'), width: 180 },
  { prop: 'timezone', label: t('scheduler.timezone'), width: 150 },
  { key: 'status', prop: 'id', label: t('common.status'), width: 100 },
  { key: 'actions', prop: 'id', label: t('common.actions'), width: 300 },
])
const jobColumns = computed<TableColumn<Job>[]>(() => [
  { prop: 'id', label: t('scheduler.id'), width: 90 },
  { prop: 'taskType', label: t('scheduler.taskType'), minWidth: 220 },
  { key: 'trigger', prop: 'triggerSource', label: t('scheduler.trigger'), width: 120 },
  { key: 'status', prop: 'status', label: t('common.status'), width: 120 },
  { prop: 'attemptCount', label: t('scheduler.attempt'), width: 100 },
  { prop: 'createdAt', label: t('common.createdAt'), width: 240 },
  { key: 'actions', prop: 'id', label: t('common.actions'), width: 160 },
])
const state = computed<'loading' | 'error' | 'empty' | 'success'>(() =>
  loading.value
    ? 'loading'
    : loadError.value
      ? 'error'
      : visibleSchedules.value.length === 0
        ? 'empty'
        : 'success',
)
const jobsState = computed<'loading' | 'error' | 'empty' | 'success'>(() =>
  jobsLoading.value
    ? 'loading'
    : jobsError.value
      ? 'error'
      : jobs.value.length === 0
        ? 'empty'
        : 'success',
)
const tasks = computed(() => options.value?.tasks ?? [])
const taskOptions = computed(() => tasks.value.filter((item) => item.adminCreatable))
function taskDisplayName(type: string): string {
  return resolveTaskDisplayName(type, tasks.value)
}
async function load(append = false): Promise<void> {
  const sequence = ++loadSequence
  if (!canList.value) {
    options.value = null
    schedules.value = []
    jobsVisible.value = false
    jobs.value = []
    loading.value = false
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    if (options.value === null) {
      const result = await getSchedulerOptions()
      if (sequence !== loadSequence) return
      options.value = result
    }
    const beforeID = append ? (schedules.value.at(-1)?.id ?? 0) : 0
    const result = await listSchedules(beforeID)
    if (sequence !== loadSequence) return
    schedules.value = append ? [...schedules.value, ...result] : result
    schedulesHaveMore.value = result.length === 100
    if (jobsVisible.value) await loadJobs()
  } catch {
    if (sequence === loadSequence) {
      if (!append) schedules.value = []
      loadError.value = t('scheduler.loadFailed')
    }
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}
async function loadJobs(): Promise<void> {
  const sequence = ++jobsSequence
  const scheduleID = selectedSchedule.value?.id
  if (!jobsVisible.value || scheduleID === undefined || !canList.value) return
  jobs.value = []
  jobsLoading.value = true
  jobsError.value = ''
  try {
    const result = await listJobs({
      scheduleId: scheduleID,
      page: jobsPage.value,
      pageSize: jobsPageSize.value,
      ...(jobStatus.value === '' ? {} : { status: jobStatus.value }),
    })
    if (
      sequence !== jobsSequence ||
      !jobsVisible.value ||
      selectedSchedule.value?.id !== scheduleID
    )
      return
    jobs.value = result.list
    jobsTotal.value = result.total
  } catch {
    if (sequence === jobsSequence) jobsError.value = t('scheduler.loadFailed')
  } finally {
    if (sequence === jobsSequence) jobsLoading.value = false
  }
}
function openJobs(row: Schedule): void {
  if (!canList.value) return
  selectedSchedule.value = row
  jobStatus.value = ''
  jobsPage.value = 1
  jobsTotal.value = 0
  jobsVisible.value = true
  void loadJobs()
}
watch(jobsVisible, (visible) => {
  if (!visible) {
    jobsSequence++
    jobsLoading.value = false
    jobs.value = []
    detailVisible.value = false
    selectedJob.value = null
  }
})

function openCreate(): void {
  if (options.value === null) return
  editing.value = null
  const first = taskOptions.value[0]
  form.value = {
    name: '',
    description: '',
    taskType: first?.type ?? '',
    cronExpression: options.value.defaults.cronExpression,
    cronPreset: getCronPresetValue(
      options.value.defaults.cronExpression,
      options.value.cronPresets,
    ),
    timezone: options.value.defaults.timezone,
    params: first?.defaultParams ?? {},
    status: YesNo.Yes,
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
    cronPreset: getCronPresetValue(row.cronExpression, cronPresets.value),
    timezone: row.timezone,
    params: { ...row.params },
    status: row.status,
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
    options.value === null ||
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
        status: form.value.status,
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
  const next = row.status === YesNo.Yes ? YesNo.No : YesNo.Yes
  try {
    await ElMessageBox.confirm(
      t('confirm.status', {
        name: row.name,
        status: t(next === YesNo.Yes ? 'common.enable' : 'common.disable'),
      }),
      t('confirm.title'),
      { type: 'warning' },
    )
    if (!canStatus.value) return
    await setScheduleStatus(row.id, next)
    await load()
  } catch {
    // Cancellation sends no request; request.ts owns API error notifications.
  }
}
async function remove(row: Schedule): Promise<void> {
  if (!row.actions.delete || !canDelete.value) return
  try {
    await ElMessageBox.confirm(t('scheduler.deleteConfirm'), t('scheduler.deleteTitle'), {
      type: 'warning',
    })
    await deleteSchedule(row.id)
    await load()
    ElNotification.success({ title: t('scheduler.deleted') })
  } catch {
    // Cancellation sends no request; request.ts owns API error notifications.
  }
}
async function execute(row: Schedule): Promise<void> {
  if (!canExecute.value) return
  try {
    await ElMessageBox.confirm(t('confirm.execute', { name: row.name }), t('confirm.title'), {
      type: 'warning',
    })
    await executeSchedule(row.id)
    await load()
    ElNotification.success({ title: t('scheduler.executed') })
  } catch {
    // Cancellation sends no request; request.ts owns API error notifications.
  }
}
async function retry(row: Job): Promise<void> {
  if (!canRetry.value || !row.actions.retry) return
  try {
    await ElMessageBox.confirm(t('confirm.retry', { id: row.id }), t('confirm.title'), {
      type: 'warning',
    })
    await retryJob(row.id)
    await load()
    ElNotification.success({ title: t('scheduler.retried') })
  } catch {
    // Cancellation sends no request; request.ts owns API error notifications.
  }
}
function openJob(row: Job): void {
  selectedJob.value = row
  detailVisible.value = true
}
function changeJobStatus(value: string | number | boolean | null | undefined): void {
  jobStatus.value = typeof value === 'number' && Number.isFinite(value) ? value : ''
  jobsPage.value = 1
  void loadJobs()
}
function updateJobsPagination(next: TablePaginationState): void {
  jobsPage.value = next.pageSize === jobsPageSize.value ? next.currentPage : 1
  jobsPageSize.value = next.pageSize
  void loadJobs()
}

watch(
  [locale, canList],
  () => {
    options.value = null
    void load()
  },
  { immediate: true, flush: 'sync' },
)
onBeforeUnmount(() => {
  loadSequence++
  jobsSequence++
})
</script>

<template>
  <AppPage class="scheduler-page">
    <AppTable
      :columns="scheduleColumns"
      :data="visibleSchedules"
      :loading="loading"
      :result-state="state"
      :status-message="loadError"
      row-key="id"
      @refresh="load()"
    >
      <template #toolbar-left>
        <el-select-v2
          v-model="selectedPlanID"
          data-testid="schedule-plan-filter"
          class="scheduler-plan-filter"
          :options="planOptions"
          :placeholder="t('scheduler.allPlans')"
          :value-on-clear="''"
          clearable
          filterable
        />
        <el-button
          v-if="canCreate && taskOptions.length > 0"
          type="primary"
          :icon="CirclePlus"
          @click="openCreate"
          >{{ t('scheduler.create') }}</el-button
        ></template
      >
      <template #cell-status="{ row }: { row: Schedule }"
        ><el-tag :type="scheduleStatusPresentation(row.status)?.tone ?? 'info'">{{
          resolveOptionLabel(row.status, scheduleStatuses)
        }}</el-tag></template
      >
      <template #cell-taskType="{ row }: { row: Schedule }">
        <el-tooltip :content="row.taskType" placement="top">
          <span>{{ taskDisplayName(row.taskType) }}</span>
        </el-tooltip>
      </template>
      <template #cell-cronExpression="{ row }: { row: Schedule }">
        <el-tooltip :content="row.cronExpression" placement="top">
          <span>{{ resolveOptionLabel(row.cronExpression, cronPresets) }}</span>
        </el-tooltip>
      </template>
      <template #cell-actions="{ row }: { row: Schedule }"
        ><el-space
          ><el-button
            :data-testid="`schedule-jobs-${row.id}`"
            type="primary"
            link
            @click="openJobs(row)"
            >{{ t('scheduler.jobs') }}</el-button
          ><el-button v-if="canUpdate" type="primary" link @click="openEdit(row)">{{
            t('common.edit')
          }}</el-button
          ><el-button
            v-if="canStatus"
            :type="row.status === YesNo.Yes ? 'warning' : 'success'"
            link
            @click="toggle(row)"
            >{{ row.status === YesNo.Yes ? t('common.disable') : t('common.enable') }}</el-button
          ><el-button v-if="canExecute" type="success" link @click="execute(row)">{{
            t('scheduler.execute')
          }}</el-button
          ><el-button
            v-if="canDelete && row.actions.delete"
            type="danger"
            link
            @click="remove(row)"
            >{{ t('common.delete') }}</el-button
          ></el-space
        ></template
      >
    </AppTable>
    <el-button v-if="schedulesHaveMore" type="primary" :loading="loading" @click="load(true)">{{
      t('scheduler.loadMorePlans')
    }}</el-button>
    <AppDialog
      v-model="jobsVisible"
      :title="t('scheduler.planJobs', { name: selectedSchedule?.name ?? '' })"
      width="min(1400px, 94vw)"
      body-padding="0"
    >
      <AppTable
        class="scheduler-job-history"
        :columns="jobColumns"
        :data="jobs"
        :loading="jobsLoading"
        :result-state="jobsState"
        :status-message="jobsError"
        :pagination="jobsPagination"
        @update:pagination="updateJobsPagination"
        row-key="id"
        @refresh="loadJobs()"
      >
        <template #toolbar-left
          ><el-select-v2
            data-testid="schedule-job-status"
            class="scheduler-job-status-filter"
            :model-value="jobStatus"
            :options="jobStatusOptions"
            :placeholder="t('common.allStatuses')"
            :value-on-clear="''"
            clearable
            @update:model-value="changeJobStatus"
        /></template>
        <template #cell-taskType="{ row }: { row: Job }">
          <el-tooltip :content="row.taskType" placement="top">
            <span>{{ taskDisplayName(row.taskType) }}</span>
          </el-tooltip>
        </template>
        <template #cell-trigger="{ row }: { row: Job }">
          <el-tooltip :content="row.triggerSource" placement="top">
            <span>{{ resolveOptionLabel(row.triggerSource, triggerSources) }}</span>
          </el-tooltip>
        </template>
        <template #cell-status="{ row }: { row: Job }"
          ><el-tag :type="jobStatusPresentation(row.status)?.tone ?? 'info'">{{
            resolveOptionLabel(row.status, jobStatusOptions)
          }}</el-tag></template
        >
        <template #cell-createdAt="{ row }: { row: Job }">
          <span>{{ formatTime(row.createdAt) }}</span>
        </template>
        <template #cell-actions="{ row }: { row: Job }"
          ><el-space
            ><el-button v-if="canDetail" type="primary" link @click="openJob(row)">{{
              t('common.detail')
            }}</el-button
            ><el-button
              v-if="canRetry && row.actions.retry"
              type="warning"
              link
              @click="retry(row)"
              >{{ t('common.retry') }}</el-button
            ></el-space
          ></template
        >
      </AppTable>
    </AppDialog>
    <AppDialog
      v-model="dialogVisible"
      :title="editing === null ? t('scheduler.create') : t('common.edit')"
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
            t('common.save')
          }}</el-button></template
        ></el-form
      >
    </AppDialog>
    <JobDetailDialog
      v-model="detailVisible"
      :job="selectedJob"
      :task-options="tasks"
      :options="options"
    />
  </AppPage>
</template>

<style scoped lang="scss">
.scheduler-job-history {
  padding: 20px 20px 0;

  :deep(.app-table__pagination) {
    position: sticky;
    z-index: 2;
    bottom: 0;
    padding: 12px 0 20px;
    border-top: 1px solid var(--el-border-color-lighter);
    background: var(--el-dialog-bg-color, var(--el-bg-color));
  }

  @media (max-width: 768px) {
    padding: 12px 16px 0;

    :deep(.app-table__pagination) {
      padding-bottom: 12px;
    }
  }
}

.scheduler-job-status-filter,
.scheduler-plan-filter {
  width: 200px;
}

.scheduler-cron-custom-input {
  margin-top: 8px;
}

.scheduler-job-status-filter {
  @media (max-width: 900px) {
    width: 100%;
  }
}
</style>
