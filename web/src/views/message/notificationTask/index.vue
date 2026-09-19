<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'

import * as taskApi from '@/api/message/notificationTask'
import type { TableColumn, TablePaginationState } from '@/components/AppTable/types'
import { usePermissionStore } from '@/store/permission'
import NotificationEditor from './components/NotificationEditor/index.vue'
import NotificationTaskDetail from './components/NotificationTaskDetail/index.vue'
import { useNotificationTaskOptions } from './useNotificationTaskOptions'

const access = usePermissionStore()
const { t } = useI18n()
const rows = ref<taskApi.NotificationTaskListItem[]>([])
const loading = ref(false)
const errorMessage = ref('')
const statusFilter = ref<taskApi.NotificationTaskStatus | ''>('')
const pagination = reactive<TablePaginationState>({ currentPage: 1, pageSize: 20, total: 0 })
const dialogOpen = ref(false)
const dialogMode = ref<'create' | 'edit' | 'detail'>('create')
const dialogLoading = ref(false)
const dialogError = ref('')
const detailTask = ref<taskApi.NotificationTask | null>(null)
const saving = ref(false)
const editingID = ref<number | null>(null)

const emptyForm = (): taskApi.NotificationTaskInput => ({
  platformId: 0,
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
const form = reactive<taskApi.NotificationTaskInput>(emptyForm())
const {
  ensureSelectedTargets,
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
  () => t('notificationTask.optionFailed'),
)
const can = (code: string): boolean => access.hasPermission(code)
const canUseDetail = computed(() => can('message:notificationTask:detail'))
const readonly = computed(() => dialogMode.value === 'detail')
const dialogTitle = computed(() =>
  dialogMode.value === 'detail'
    ? t('notificationTask.detailTitle')
    : t('notificationTask.dialogTitle'),
)
const columns = computed<TableColumn<taskApi.NotificationTaskListItem>[]>(() => [
  { prop: 'title', label: t('notificationTask.title'), minWidth: 180 },
  { prop: 'platformId', label: t('notificationTask.platform'), width: 100 },
  { prop: 'audienceType', label: t('notificationTask.audienceLabel'), width: 110 },
  { prop: 'status', label: t('notificationTask.statusLabel'), width: 110 },
  { prop: 'generatedCount', label: t('notificationTask.generatedCount'), width: 120 },
  { prop: 'scheduledAt', label: t('notificationTask.scheduledAt'), minWidth: 170 },
  { prop: 'submittedAt', label: t('notificationTask.submittedAt'), minWidth: 170 },
  { prop: 'completedAt', label: t('notificationTask.completedAt'), minWidth: 170 },
  { key: 'actions', label: t('notificationTask.actions'), width: 300 },
])
const statusOptions = computed(() => [
  { value: '', label: t('notificationTask.statusAll') },
  ...(
    ['draft', 'scheduled', 'queued', 'processing', 'completed', 'failed', 'canceled'] as const
  ).map((value) => ({ value, label: t(`notificationTask.status.${value}`) })),
])
const audienceOptions = computed(() =>
  (['platform', 'user', 'role'] as const).map((value) => ({
    value,
    label: t(`notificationTask.audience.${value}`),
  })),
)
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
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await taskApi.listNotificationTasks(
      pagination.currentPage,
      pagination.pageSize,
      statusFilter.value,
    )
    rows.value = result.list
    pagination.total = result.total
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('notificationTask.loadFailed')
  } finally {
    loading.value = false
  }
}

function resetForm(): void {
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
  editingID.value = id
  dialogMode.value = mode
  dialogOpen.value = true
  dialogLoading.value = true
  try {
    const row = await taskApi.getNotificationTask(id)
    applyDetail(row)
    if (mode === 'edit') {
      await loadOptions('platform')
      if (row.audienceType !== 'platform') {
        await loadOptions(row.audienceType)
        ensureSelectedTargets(row.audienceType, row.targetIds)
      }
    }
  } catch (error) {
    dialogError.value = error instanceof Error ? error.message : t('notificationTask.loadFailed')
  } finally {
    dialogLoading.value = false
  }
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
    if (editingID.value === null) await taskApi.createNotificationTask({ ...form })
    else await taskApi.updateNotificationTask(editingID.value, { ...form })
    ElMessage.success(t('notificationTask.saveSuccess'))
    dialogOpen.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function remove(row: taskApi.NotificationTaskListItem): Promise<void> {
  await ElMessageBox.confirm(t('notificationTask.deleteConfirm'), t('notificationTask.delete'))
  await taskApi.deleteNotificationTask(row.id)
  ElMessage.success(t('notificationTask.deleteSuccess'))
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
  ElMessage.success(t(`notificationTask.${action}Success`))
  await load()
  if (
    action === 'copy' &&
    can('message:notificationTask:detail') &&
    can('message:notificationTask:update')
  )
    await loadDetail(result.id, 'edit')
}

function updatePagination(next: TablePaginationState): void {
  Object.assign(pagination, next)
  void load()
}

onMounted(() => void load())
watch(statusFilter, () => {
  pagination.currentPage = 1
  void load()
})
</script>

<template>
  <AppPage>
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :result-state="errorMessage ? 'error' : rows.length === 0 ? 'empty' : 'success'"
      :status-message="errorMessage"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #toolbar-left>
        <el-select-v2 v-model="statusFilter" :options="statusOptions" />
      </template>
      <template #toolbar-right>
        <el-button
          v-if="can('message:notificationTask:create') && canUseDetail"
          data-testid="notification-task-create"
          type="primary"
          @click="openCreate"
          >{{ t('notificationTask.create') }}</el-button
        >
      </template>
      <template #actions="{ row }">
        <el-button
          v-if="canUseDetail"
          :data-testid="`notification-task-detail-${row.id}`"
          link
          @click.stop="openDetail(row)"
          >{{ t('notificationTask.detail') }}</el-button
        >
        <el-button
          v-if="row.status === 'draft' && canUseDetail && can('message:notificationTask:update')"
          :data-testid="`notification-task-edit-${row.id}`"
          link
          @click.stop="openEdit(row)"
          >{{ t('notificationTask.edit') }}</el-button
        >
        <el-button
          v-if="row.status === 'draft' && can('message:notificationTask:delete')"
          :data-testid="`notification-task-delete-${row.id}`"
          link
          type="danger"
          @click.stop="remove(row)"
          >{{ t('notificationTask.delete') }}</el-button
        >
        <el-button
          v-if="row.status === 'draft' && can('message:notificationTask:submit')"
          :data-testid="`notification-task-submit-${row.id}`"
          link
          @click.stop="command(row, 'submit')"
          >{{ t('notificationTask.submit') }}</el-button
        >
        <el-button
          v-if="
            ['scheduled', 'queued', 'processing'].includes(row.status) &&
            can('message:notificationTask:cancel')
          "
          :data-testid="`notification-task-cancel-${row.id}`"
          link
          @click.stop="command(row, 'cancel')"
          >{{ t('notificationTask.cancel') }}</el-button
        >
        <el-button
          v-if="row.status !== 'draft' && can('message:notificationTask:copy')"
          :data-testid="`notification-task-copy-${row.id}`"
          link
          @click.stop="command(row, 'copy')"
          >{{ t('notificationTask.copy') }}</el-button
        >
      </template>
    </AppTable>

    <AppDialog v-model="dialogOpen" :title="dialogTitle" width="760px" height="70vh">
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
      <el-form v-else label-position="top">
        <div class="notification-task-form__grid">
          <el-form-item :label="t('notificationTask.platform')">
            <el-select-v2
              v-model="form.platformId"
              :options="platformOptions"
              filterable
              remote
              :loading="optionStates.platform.loading"
              :remote-method="remotePlatformOptions"
            />
          </el-form-item>
          <el-form-item :label="t('notificationTask.audienceLabel')">
            <el-select-v2
              v-model="form.audienceType"
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
            @click="loadOptions(targetKind, '', true)"
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
          @click="loadOptions('platform', '', true)"
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
        <el-button @click="dialogOpen = false">{{ t('notificationTask.close') }}</el-button>
        <el-button v-if="!readonly" type="primary" :loading="saving" @click="save">{{
          t('notificationTask.save')
        }}</el-button>
      </template>
    </AppDialog>
  </AppPage>
</template>

<style scoped>
.notification-task-form__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.notification-task-form__error {
  color: var(--el-color-danger);
  font-size: 12px;
}
.notification-task-form__grid--three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
.notification-task-form__state {
  display: flex;
  min-height: 160px;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
@media (max-width: 720px) {
  .notification-task-form__grid {
    grid-template-columns: 1fr;
  }
}
</style>
