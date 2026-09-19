<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import * as taskApi from '@/api/message/notificationTask'
import type { SearchFormModel } from '@/components/AppSearch'
import type { TablePaginationState } from '@/components/AppTable/types'
import { usePermissionStore } from '@/store/permission'
import { ProtocolError } from '@/types/http'
import NotificationEditor from './components/NotificationEditor/index.vue'
import NotificationTaskSearch from './components/NotificationTaskSearch/index.vue'
import NotificationTaskTable from './components/NotificationTaskTable/index.vue'
import NotificationTaskDetail from './components/NotificationTaskDetail/index.vue'
import { useNotificationTaskOptions } from './useNotificationTaskOptions'

const access = usePermissionStore()
const { t } = useI18n()
const rows = ref<taskApi.NotificationTaskListItem[]>([])
const loading = ref(false)
const errorMessage = ref('')
const platformIDFilter = ref('')
const statusFilter = ref<taskApi.NotificationTaskStatus | ''>('')
const audienceFilter = ref<taskApi.NotificationAudience | ''>('')
const keywordFilter = ref('')
const timeRange = ref<[] | [string, string]>([])
const pagination = reactive<TablePaginationState>({ currentPage: 1, pageSize: 20, total: 0 })
const dialogOpen = ref(false)
const dialogMode = ref<'create' | 'edit' | 'detail'>('create')
const dialogLoading = ref(false)
const dialogError = ref('')
const detailTask = ref<taskApi.NotificationTask | null>(null)
const saving = ref(false)
const editingID = ref<number | null>(null)
let listSequence = 0
let dialogSequence = 0

type NotificationTaskFormModel = Omit<taskApi.NotificationTaskInput, 'platformId'> & {
  platformId: number | null
}

const emptyForm = (): NotificationTaskFormModel => ({
  platformId: null,
  title: '',
  contentHtml: '',
  variant: 'info',
  priority: 'normal',
  linkType: 'none',
  link: '',
  audienceType: 'platform',
  targetIds: [],
  scheduledAt: null,
})
const form = reactive<NotificationTaskFormModel>(emptyForm())
const {
  ensureSelectedTargets,
  loadMoreOptions,
  loadOptions,
  optionStates,
  platformOptions,
  remotePlatformOptions,
  remoteTargetOptions,
  resetOptions,
  targetKind,
  targetState,
} = useNotificationTaskOptions(
  () => form.audienceType,
  () => (dialogMode.value === 'create' ? 'create' : 'update'),
  () => t('notificationTask.optionFailed'),
)
const can = (code: string): boolean => access.hasPermission(code)
const readonly = computed(() => dialogMode.value === 'detail')
const dialogTitle = computed(() =>
  dialogMode.value === 'detail'
    ? t('notificationTask.detailTitle')
    : t('notificationTask.dialogTitle'),
)
const audienceOptions = computed(() =>
  (['platform', 'user', 'role'] as const).map((value) => ({
    value,
    label: t(`notificationTask.audience.${value}`),
  })),
)
const searchModel = computed<SearchFormModel>({
  get: () => ({
    platformId: platformIDFilter.value,
    status: statusFilter.value,
    audienceType: audienceFilter.value,
    keyword: keywordFilter.value,
    timeRange: timeRange.value,
  }),
  set: (value) => {
    platformIDFilter.value =
      typeof value.platformId === 'string' || typeof value.platformId === 'number'
        ? String(value.platformId)
        : ''
    statusFilter.value =
      typeof value.status === 'string' &&
      ['draft', 'scheduled', 'queued', 'processing', 'completed', 'failed', 'canceled'].includes(
        value.status,
      )
        ? (value.status as taskApi.NotificationTaskStatus)
        : ''
    audienceFilter.value =
      value.audienceType === 'user' ||
      value.audienceType === 'role' ||
      value.audienceType === 'platform'
        ? value.audienceType
        : ''
    keywordFilter.value = typeof value.keyword === 'string' ? value.keyword : ''
    timeRange.value =
      Array.isArray(value.timeRange) &&
      value.timeRange.length === 2 &&
      value.timeRange.every((item) => typeof item === 'string')
        ? [value.timeRange[0], value.timeRange[1]]
        : []
  },
})
const variantOptions = computed(() =>
  (['info', 'success', 'warning', 'error'] as const).map((value) => ({
    value,
    label: t(`notificationTask.variant.${value}`),
  })),
)
const priorityOptions = computed(() => [
  { value: 'normal', label: t('notification.normal') },
  { value: 'urgent', label: t('notification.urgent') },
])
const linkTypeOptions = computed(() =>
  (['none', 'internal', 'external'] as const).map((value) => ({
    value,
    label: t(`notificationTask.linkType.${value}`),
  })),
)

async function load(): Promise<void> {
  if (!can('message:notificationTask:list')) return
  const current = ++listSequence
  loading.value = true
  errorMessage.value = ''
  try {
    const platformID = platformIDFilter.value.trim()
    const result = await taskApi.listNotificationTasks({
      page: pagination.currentPage,
      pageSize: pagination.pageSize,
      ...(platformID === '' ? {} : { platformId: Number(platformID) }),
      ...(statusFilter.value === '' ? {} : { status: statusFilter.value }),
      ...(audienceFilter.value === '' ? {} : { audienceType: audienceFilter.value }),
      ...(keywordFilter.value.trim() === '' ? {} : { keyword: keywordFilter.value.trim() }),
      ...(timeRange.value.length === 0 ? {} : { from: timeRange.value[0], to: timeRange.value[1] }),
    })
    if (current !== listSequence) return
    rows.value = result.list
    pagination.total = result.total
  } catch (error) {
    if (current === listSequence)
      errorMessage.value = error instanceof Error ? error.message : t('notificationTask.loadFailed')
  } finally {
    if (current === listSequence) loading.value = false
  }
}

function resetForm(): void {
  dialogSequence += 1
  dialogLoading.value = false
  Object.assign(form, emptyForm())
  editingID.value = null
  detailTask.value = null
  dialogError.value = ''
  resetOptions()
}

function openCreate(): void {
  resetForm()
  dialogMode.value = 'create'
  dialogOpen.value = true
  void loadOptions('platform')
}

function applyDetail(row: taskApi.NotificationTask): void {
  detailTask.value = row
  editingID.value = row.id
  Object.assign(form, {
    platformId: row.platformId,
    title: row.title,
    contentHtml: row.contentHtml,
    variant: row.variant,
    priority: row.priority,
    linkType: row.linkType,
    link: row.link,
    audienceType: row.audienceType,
    targetIds: [...row.targetIds],
    scheduledAt: row.scheduledAt,
  })
}

async function loadDetail(id: number, mode: 'edit' | 'detail'): Promise<void> {
  resetForm()
  const current = dialogSequence
  editingID.value = id
  dialogMode.value = mode
  dialogOpen.value = true
  dialogLoading.value = true
  try {
    const row =
      mode === 'edit'
        ? await taskApi.getNotificationTaskForUpdate(id)
        : await taskApi.getNotificationTask(id)
    if (!isCurrentDialogRequest(current, id, mode)) return
    applyDetail(row)
    if (mode === 'edit') {
      await loadOptions('platform')
      if (!isCurrentDialogRequest(current, id, mode)) return
      if (row.audienceType !== 'platform') {
        await loadOptions(row.audienceType)
        if (!isCurrentDialogRequest(current, id, mode)) return
        ensureSelectedTargets(row.audienceType, row.targetIds)
      }
    }
  } catch (error) {
    if (isCurrentDialogRequest(current, id, mode))
      dialogError.value = error instanceof Error ? error.message : t('notificationTask.loadFailed')
  } finally {
    if (isCurrentDialogRequest(current, id, mode)) dialogLoading.value = false
  }
}

function isCurrentDialogRequest(sequence: number, id: number, mode: 'edit' | 'detail'): boolean {
  return (
    sequence === dialogSequence &&
    dialogOpen.value &&
    editingID.value === id &&
    dialogMode.value === mode
  )
}

function updateDialogOpen(value: boolean): void {
  dialogOpen.value = value
  if (value) return
  dialogSequence += 1
  dialogLoading.value = false
  resetOptions()
}

function openEdit(row: taskApi.NotificationTaskListItem): void {
  void loadDetail(row.id, 'edit')
}

function openDetail(row: taskApi.NotificationTaskListItem): void {
  void loadDetail(row.id, 'detail')
}

function changeAudience(value: taskApi.NotificationAudience): void {
  form.targetIds = []
  if (value !== 'platform') void loadOptions(value)
}

function changeLinkType(value: taskApi.NotificationTaskInput['linkType']): void {
  if (value === 'none') form.link = ''
}

async function save(): Promise<void> {
  const platformID = form.platformId
  if (platformID === null || !Number.isInteger(platformID) || platformID < 1) {
    ElMessage.warning(t('notificationTask.platformRequired'))
    return
  }
  if (form.scheduledAt !== null && Date.parse(form.scheduledAt) <= Date.now()) {
    ElMessage.warning(t('notificationTask.scheduledAtFuture'))
    return
  }
  if (form.audienceType !== 'platform' && form.targetIds.length === 0) {
    ElMessage.warning(t('notificationTask.targetsRequired'))
    return
  }
  saving.value = true
  try {
    const payload: taskApi.NotificationTaskInput = { ...form, platformId: platformID }
    if (editingID.value === null) await taskApi.createNotificationTask(payload)
    else await taskApi.updateNotificationTask(editingID.value, payload)
    ElNotification.success({ title: t('notificationTask.saveSuccess') })
    updateDialogOpen(false)
    await load()
  } catch (error) {
    if (error instanceof ProtocolError) {
      ElNotification.error({
        title: t('request.failed'),
        message: t('request.protocolError'),
      })
    }
  } finally {
    saving.value = false
  }
}

async function remove(row: taskApi.NotificationTaskListItem): Promise<void> {
  await ElMessageBox.confirm(t('notificationTask.deleteConfirm'), t('notificationTask.delete'))
  await taskApi.deleteNotificationTask(row.id)
  ElNotification.success({ title: t('notificationTask.deleteSuccess') })
  await load()
}

async function command(
  row: taskApi.NotificationTaskListItem,
  action: 'submit' | 'cancel' | 'copy',
): Promise<void> {
  if (action === 'submit')
    await ElMessageBox.confirm(t('notificationTask.submitConfirm'), t('notificationTask.submit'))
  if (action === 'cancel')
    await ElMessageBox.confirm(t('notificationTask.cancelConfirm'), t('notificationTask.cancel'))
  const result = await taskApi.commandNotificationTask(row.id, action)
  ElNotification.success({ title: t(`notificationTask.${action}Success`) })
  await load()
  if (action === 'copy' && can('message:notificationTask:update'))
    await loadDetail(result.id, 'edit')
}

function updatePagination(next: TablePaginationState): void {
  Object.assign(pagination, next)
  void load()
}

function search(): void {
  const platformID = platformIDFilter.value.trim()
  if (platformID !== '' && (!/^\d+$/.test(platformID) || Number(platformID) < 1)) {
    ElMessage.warning(t('notificationTask.platformIdInvalid'))
    return
  }
  pagination.currentPage = 1
  void load()
}

function resetSearch(): void {
  platformIDFilter.value = ''
  statusFilter.value = ''
  audienceFilter.value = ''
  keywordFilter.value = ''
  timeRange.value = []
  pagination.currentPage = 1
  void load()
}

onMounted(() => void load())
</script>

<template>
  <AppPage>
    <NotificationTaskSearch v-model="searchModel" @query="search" @reset="resetSearch" />
    <NotificationTaskTable
      :rows="rows"
      :loading="loading"
      :pagination="pagination"
      :error-message="errorMessage"
      @create="openCreate"
      @detail="openDetail"
      @edit="openEdit"
      @remove="remove"
      @command="command"
      @refresh="load"
      @update:pagination="updatePagination"
    />

    <AppDialog
      :model-value="dialogOpen"
      :title="dialogTitle"
      width="760px"
      height="70vh"
      @update:model-value="updateDialogOpen"
    >
      <div v-if="dialogLoading" class="notification-task-form__state">
        {{ t('notificationTask.loadingDetail') }}
      </div>
      <div v-else-if="dialogError" class="notification-task-form__state">
        <span>{{ dialogError }}</span>
        <el-button
          v-if="editingID !== null"
          link
          type="primary"
          @click="loadDetail(editingID, dialogMode === 'detail' ? 'detail' : 'edit')"
          >{{ t('notificationTask.retry') }}</el-button
        >
      </div>
      <NotificationTaskDetail v-else-if="readonly && detailTask !== null" :task="detailTask" />
      <el-form v-else class="notification-task-form" label-position="top">
        <div class="notification-task-form__grid">
          <el-form-item :label="t('notificationTask.platform')">
            <el-select-v2
              v-model="form.platformId"
              data-testid="notification-task-platform"
              :options="platformOptions"
              :placeholder="t('notificationTask.platformPlaceholder')"
              filterable
              remote
              :loading="optionStates.platform.loading"
              :remote-method="remotePlatformOptions"
            />
          </el-form-item>
          <el-form-item :label="t('notificationTask.audienceLabel')">
            <el-select-v2
              v-model="form.audienceType"
              data-testid="notification-task-audience"
              :options="audienceOptions"
              @change="changeAudience"
            />
          </el-form-item>
        </div>
        <el-form-item
          v-if="form.audienceType !== 'platform'"
          :label="t('notificationTask.targets')"
        >
          <el-select-v2
            v-model="form.targetIds"
            data-testid="notification-task-targets"
            :options="targetState.items"
            multiple
            filterable
            remote
            :loading="targetState.loading"
            :remote-method="remoteTargetOptions"
          />
          <el-button
            v-if="targetState.nextAfterId !== null"
            data-testid="notification-task-option-more"
            link
            @click="loadMoreOptions(targetKind)"
            >{{ t('notificationTask.loadMoreOptions') }}</el-button
          >
          <span v-if="targetState.error" class="notification-task-form__error">{{
            targetState.error
          }}</span>
        </el-form-item>
        <el-button
          v-else-if="optionStates.platform.nextAfterId !== null"
          data-testid="notification-task-option-more"
          link
          @click="loadMoreOptions('platform')"
          >{{ t('notificationTask.loadMoreOptions') }}</el-button
        >
        <el-form-item :label="t('notificationTask.title')"
          ><el-input
            v-model="form.title"
            data-testid="notification-task-title"
            maxlength="128"
            show-word-limit
        /></el-form-item>
        <div class="notification-task-form__grid notification-task-form__grid--three">
          <el-form-item :label="t('notificationTask.variantLabel')">
            <el-select-v2
              v-model="form.variant"
              data-testid="notification-task-variant"
              :options="variantOptions"
            />
          </el-form-item>
          <el-form-item :label="t('notificationTask.priorityLabel')">
            <el-select-v2
              v-model="form.priority"
              data-testid="notification-task-priority"
              :options="priorityOptions"
            />
          </el-form-item>
          <el-form-item :label="t('notificationTask.linkTypeLabel')">
            <el-select-v2
              v-model="form.linkType"
              data-testid="notification-task-link-type"
              :options="linkTypeOptions"
              @change="changeLinkType"
            />
          </el-form-item>
        </div>
        <el-form-item :label="t('notificationTask.content')"
          ><NotificationEditor v-model="form.contentHtml"
        /></el-form-item>
        <div class="notification-task-form__grid">
          <el-form-item :label="t('notificationTask.scheduledAt')"
            ><el-date-picker
              v-model="form.scheduledAt"
              class="notification-task-form__date-picker"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ss.SSSZ"
              clearable
          /></el-form-item>
          <el-form-item v-if="form.linkType !== 'none'" :label="t('notificationTask.link')"
            ><el-input v-model="form.link"
          /></el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="updateDialogOpen(false)">{{ t('notificationTask.close') }}</el-button>
        <el-button v-if="!readonly" type="primary" :loading="saving" @click="save">{{
          t('notificationTask.save')
        }}</el-button>
      </template>
    </AppDialog>
  </AppPage>
</template>

<style scoped src="./NotificationTaskPage.css"></style>
