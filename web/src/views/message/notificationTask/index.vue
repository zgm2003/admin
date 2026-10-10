<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import * as taskApi from '@/api/message/notificationTask'
import {
  getNotificationTaskOptions,
  type NotificationTaskAdminOptions,
} from '@/api/message/notificationTaskOptions'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import type { SearchFormModel } from '@/components/AppSearch'
import type { TablePaginationState } from '@/components/AppTable/types'
import { usePermissionStore } from '@/store/permission'
import NotificationTaskSearch from './components/NotificationTaskSearch/index.vue'
import NotificationTaskStatusTabs from './components/NotificationTaskStatusTabs/index.vue'
import NotificationTaskTable from './components/NotificationTaskTable/index.vue'
import NotificationTaskDialog, {
  type NotificationTaskFormModel,
} from './components/NotificationTaskDialog/index.vue'
import { useNotificationTaskOptions } from './useNotificationTaskOptions'

const access = usePermissionStore()
const { t } = useI18n()
const {
  options: catalog,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<NotificationTaskAdminOptions | null>(getNotificationTaskOptions, () => null)

interface NotificationTaskSearchModel {
  platformId: string
  audienceType: string
  keyword: string
  timeRange: [] | [string, string]
}

const rows = ref<taskApi.NotificationTaskListItem[]>([])
const loading = ref(false)
const errorMessage = ref('')
const platformIDFilter = ref('')
const statusFilter = ref<taskApi.NotificationTaskStatus | ''>('')
const audienceFilter = ref('')
const keywordFilter = ref('')
const timeRange = ref<[] | [string, string]>([])
const pagination = reactive<TablePaginationState>({ currentPage: 1, pageSize: 20, total: 0 })
const dialogOpen = ref(false)
const dialogMode = ref<'create' | 'edit' | 'detail'>('create')
const dialogLoading = ref(false)
const dialogError = ref('')
const detailTask = ref<taskApi.NotificationTask | null>(null)
const saving = ref(false)
const busyIDs = ref<number[]>([])
const editingID = ref<number | null>(null)
let listSequence = 0
let dialogSequence = 0

const emptyForm = (): NotificationTaskFormModel => ({
  platformId: null,
  title: '',
  contentHtml: '',
  variant: catalog.value?.defaults.variant ?? '',
  priority: catalog.value?.defaults.priority ?? '',
  linkType: catalog.value?.defaults.linkType ?? '',
  link: '',
  audienceType: catalog.value?.defaults.audienceType ?? '',
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
  () => form.platformId,
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
const audienceOptions = computed(() => catalog.value?.audiences ?? [])
const statusOptions = computed(() => catalog.value?.statuses ?? [])
const searchModel = computed<SearchFormModel<NotificationTaskSearchModel>>({
  get: () => ({
    platformId: platformIDFilter.value,
    audienceType: audienceFilter.value,
    keyword: keywordFilter.value,
    timeRange: timeRange.value,
  }),
  set: (value) => {
    platformIDFilter.value =
      typeof value.platformId === 'string' || typeof value.platformId === 'number'
        ? String(value.platformId)
        : ''
    audienceFilter.value = typeof value.audienceType === 'string' ? value.audienceType : ''
    keywordFilter.value = typeof value.keyword === 'string' ? value.keyword : ''
    timeRange.value =
      Array.isArray(value.timeRange) &&
      value.timeRange.length === 2 &&
      value.timeRange.every((item) => typeof item === 'string')
        ? [value.timeRange[0], value.timeRange[1]]
        : []
  },
})
const variantOptions = computed(() => catalog.value?.variants ?? [])
const priorityOptions = computed(() => catalog.value?.priorities ?? [])
const linkTypeOptions = computed(() => catalog.value?.linkTypes ?? [])

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
  if (catalog.value === null || !can('message:notificationTask:create')) return
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

function updateForm(value: Partial<NotificationTaskFormModel>): void {
  Object.assign(form, value)
}

function openEdit(row: taskApi.NotificationTaskListItem): void {
  if (
    !row.actions.edit ||
    !can('message:notificationTask:update') ||
    busyIDs.value.includes(row.id)
  )
    return
  void loadDetail(row.id, 'edit')
}

function openDetail(row: taskApi.NotificationTaskListItem): void {
  if (!can('message:notificationTask:detail')) return
  void loadDetail(row.id, 'detail')
}

function changeAudience(value: taskApi.NotificationAudience): void {
  form.targetIds = []
  if (value !== 'platform') void loadOptions(value)
}

function changePlatform(value: number | null): void {
  form.targetIds = []
  if (value !== null && form.audienceType !== '' && form.audienceType !== 'platform')
    void loadOptions(form.audienceType)
}

function changeLinkType(value: taskApi.NotificationTaskInput['linkType']): void {
  if (value === 'none') form.link = ''
}

async function save(): Promise<void> {
  if (saving.value || catalog.value === null || !dialogOpen.value || readonly.value) return
  const permission =
    editingID.value === null ? 'message:notificationTask:create' : 'message:notificationTask:update'
  if (!can(permission) || (editingID.value !== null && !detailTask.value?.actions.edit)) return
  const platformID = form.platformId
  if (platformID === null || !Number.isInteger(platformID) || platformID < 1) {
    ElMessage.warning(t('notificationTask.platformRequired'))
    return
  }
  if (form.scheduledAt !== null && Date.parse(form.scheduledAt) <= Date.now()) {
    ElMessage.warning(t('notificationTask.scheduledAtFuture'))
    return
  }
  const audienceType = form.audienceType
  if (audienceType === '' || (audienceType !== 'platform' && form.targetIds.length === 0)) {
    ElMessage.warning(t('notificationTask.targetsRequired'))
    return
  }
  saving.value = true
  dialogError.value = ''
  const currentDialog = dialogSequence
  try {
    const payload: taskApi.NotificationTaskInput = {
      ...form,
      platformId: platformID,
      audienceType,
      targetIds: [...form.targetIds],
    }
    if (editingID.value === null) await taskApi.createNotificationTask(payload)
    else await taskApi.updateNotificationTask(editingID.value, payload)
    ElNotification.success({ title: t('notificationTask.saveSuccess') })
    if (currentDialog === dialogSequence) updateDialogOpen(false)
    await load()
  } catch (error) {
    if (currentDialog === dialogSequence)
      dialogError.value = error instanceof Error ? error.message : t('notificationTask.loadFailed')
  } finally {
    saving.value = false
  }
}

async function remove(row: taskApi.NotificationTaskListItem): Promise<void> {
  if (
    !row.actions.delete ||
    !can('message:notificationTask:delete') ||
    busyIDs.value.includes(row.id)
  )
    return
  busyIDs.value = [...busyIDs.value, row.id]
  errorMessage.value = ''
  try {
    await ElMessageBox.confirm(t('notificationTask.deleteConfirm'), t('notificationTask.delete'))
    await taskApi.deleteNotificationTask(row.id)
    ElNotification.success({ title: t('notificationTask.deleteSuccess') })
    await load()
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close')
      errorMessage.value = error instanceof Error ? error.message : t('notificationTask.loadFailed')
  } finally {
    busyIDs.value = busyIDs.value.filter((id) => id !== row.id)
  }
}

async function command(
  row: taskApi.NotificationTaskListItem,
  action: 'submit' | 'cancel' | 'copy',
): Promise<void> {
  if (
    !row.actions[action] ||
    !can(`message:notificationTask:${action}`) ||
    busyIDs.value.includes(row.id)
  )
    return
  busyIDs.value = [...busyIDs.value, row.id]
  errorMessage.value = ''
  try {
    if (action === 'submit')
      await ElMessageBox.confirm(t('notificationTask.submitConfirm'), t('notificationTask.submit'))
    if (action === 'cancel')
      await ElMessageBox.confirm(t('notificationTask.cancelConfirm'), t('notificationTask.cancel'))
    const result = await taskApi.commandNotificationTask(row.id, action)
    ElNotification.success({ title: t(`notificationTask.${action}Success`) })
    await load()
    if (action === 'copy' && result.actions.edit && can('message:notificationTask:update'))
      await loadDetail(result.id, 'edit')
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close')
      errorMessage.value = error instanceof Error ? error.message : t('notificationTask.loadFailed')
  } finally {
    busyIDs.value = busyIDs.value.filter((id) => id !== row.id)
  }
}

function updatePagination(next: TablePaginationState): void {
  Object.assign(pagination, next)
  void load()
}

function changeStatus(value: taskApi.NotificationTaskStatus | ''): void {
  statusFilter.value = value
  pagination.currentPage = 1
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
    <el-alert v-if="optionsError" :title="optionsError" type="error" :closable="false" show-icon>
      <el-button data-testid="notification-task-options-retry" link @click="reloadOptions">{{
        t('notificationTask.retry')
      }}</el-button>
    </el-alert>
    <NotificationTaskSearch
      v-model="searchModel"
      :audience-options="audienceOptions"
      @query="search"
      @reset="resetSearch"
    />
    <NotificationTaskStatusTabs
      v-model="statusFilter"
      :statuses="statusOptions"
      @change="changeStatus"
    />
    <NotificationTaskTable
      :rows="rows"
      :loading="loading"
      :pagination="pagination"
      :error-message="errorMessage"
      :statuses="statusOptions"
      :audiences="audienceOptions"
      :create-disabled="catalog === null"
      :busy-ids="busyIDs"
      @create="openCreate"
      @detail="openDetail"
      @edit="openEdit"
      @remove="remove"
      @command="command"
      @refresh="load"
      @update:pagination="updatePagination"
    />

    <NotificationTaskDialog
      v-model="dialogOpen"
      @update:form="updateForm"
      :title="dialogTitle"
      :readonly="readonly"
      :loading="dialogLoading"
      :error-message="dialogError"
      :task-id="editingID"
      :detail-task="detailTask"
      :form="form"
      :saving="saving"
      :platform-options="platformOptions"
      :option-states="optionStates"
      :target-state="targetState"
      :target-kind="targetKind"
      :audience-options="audienceOptions"
      :status-options="statusOptions"
      :title-max-length="catalog?.constraints.titleMaxLength"
      :variant-options="variantOptions"
      :priority-options="priorityOptions"
      :link-type-options="linkTypeOptions"
      :remote-platform-options="remotePlatformOptions"
      :remote-target-options="remoteTargetOptions"
      @save="save"
      @retry="loadDetail"
      @change-platform="changePlatform"
      @change-audience="changeAudience"
      @change-link-type="changeLinkType"
      @load-more="loadMoreOptions"
    />
  </AppPage>
</template>

<style scoped src="./NotificationTaskPage.scss" lang="scss"></style>
