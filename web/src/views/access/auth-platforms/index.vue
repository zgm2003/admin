<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { CirclePlus } from '@element-plus/icons-vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  createAuthPlatform,
  deleteAuthPlatform,
  getAuthPlatformDeployment,
  getAuthPlatforms,
  updateAuthPlatform,
  updateAuthPlatformStatus,
} from '../../../api/auth-platform'
import type {
  AuthPlatformDeployment,
  AuthPlatformListItem,
  AuthPlatformListQuery,
  CreateAuthPlatformInput,
  UpdateAuthPlatformInput,
} from '../../../api/auth-platform.contract'
import { YesNo } from '../../../enums/yes-no'
import { useAccessStore } from '../../../store/access'
import { AppDialog } from '../../../components/AppDialog'
import { AppTable } from '../../../components/AppTable'
import type { TableColumn, TablePaginationState } from '../../../components/AppTable'
import { Search } from '../../../components/Search'
import type { SearchField, SearchFormModel } from '../../../components/Search'

const { t } = useI18n()
const access = useAccessStore()

const rows = ref<AuthPlatformListItem[]>([])
const total = ref(0)
const query = ref<AuthPlatformListQuery>({ page: 1, pageSize: 20 })
const keyword = ref('')
const statusFilter = ref<'' | YesNo>('')
const loading = ref(false)
const loadError = ref('')
const mutationError = ref('')
const deployment = ref<AuthPlatformDeployment | null>(null)
const searchModel = computed<SearchFormModel>({
  get: () => ({ keyword: keyword.value, status: statusFilter.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    statusFilter.value = value.status === YesNo.Yes || value.status === YesNo.No ? value.status : ''
  },
})
const searchFields = computed<SearchField[]>(() => [
  { key: 'keyword', type: 'input', label: t('authPlatform.keyword'), placeholder: t('authPlatform.keyword'), width: 260, testId: 'auth-platform-keyword' },
  { key: 'status', type: 'select-v2', label: t('authPlatform.status.all'), options: [{ label: t('authPlatform.status.all'), value: '' }, { label: t('authPlatform.status.enabled'), value: YesNo.Yes }, { label: t('authPlatform.status.disabled'), value: YesNo.No }], width: 160, testId: 'auth-platform-status-filter' },
])

const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingPlatform = ref<AuthPlatformListItem | null>(null)
const submitting = ref(false)

const tablePagination = computed<TablePaginationState>(() => ({ currentPage: query.value.page, pageSize: query.value.pageSize, total: total.value }))
const tableColumns = computed<TableColumn<AuthPlatformListItem>[]>(() => [
  { key: 'platform', prop: 'id', label: t('authPlatform.column.platform'), minWidth: 180 },
  { key: 'tokenTTL', prop: 'id', label: t('authPlatform.column.tokenTTL'), minWidth: 160 },
  { key: 'cacheTTL', prop: 'id', label: t('authPlatform.column.cacheTTL'), minWidth: 170 },
  { key: 'security', prop: 'id', label: t('authPlatform.column.security'), minWidth: 150 },
  { key: 'sessions', prop: 'id', label: t('authPlatform.column.sessions'), width: 130 },
  { key: 'status', prop: 'id', label: t('authPlatform.column.status'), width: 110 },
  { prop: 'updatedAt', label: t('authPlatform.column.updatedAt'), width: 190 },
  { key: 'actions', prop: 'id', label: t('authPlatform.column.actions'), width: 180 },
])

interface AuthPlatformForm {
  code: string
  name: string
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: YesNo
  bindIP: YesNo
  maxSessions: number
  allowRegister: YesNo
  isEnabled: YesNo
}

const form = reactive<AuthPlatformForm>(defaultForm())

const canList = computed(() => access.hasPermission('auth:platform:list'))
const canCreate = computed(() => access.hasPermission('auth:platform:create'))
const canUpdate = computed(() => access.hasPermission('auth:platform:update'))
const canStatus = computed(() => access.hasPermission('auth:platform:status'))
const canDelete = computed(() => access.hasPermission('auth:platform:delete'))
const isEditing = computed(() => dialogMode.value === 'edit')
const formValid = computed(() => {
  const codeValid = dialogMode.value === 'edit' || /^[a-z][a-z0-9_]{1,48}$/.test(form.code.trim())
  return codeValid && form.name.trim() !== '' && form.name.trim().length <= 64 &&
    inRange(form.accessTTLSeconds, 60, 2_592_000) && inRange(form.refreshTTLSeconds, 60, 31_536_000) &&
    inRange(form.sessionCacheTTLSeconds, 60, 86_400) && inRange(form.accessCacheTTLSeconds, 60, 86_400) &&
    inRange(form.maxSessions, 0, 100)
})

async function loadPage(): Promise<void> {
  if (!canList.value) return
  loading.value = true
  loadError.value = ''
  try {
    const [result, status] = await Promise.all([getAuthPlatforms(query.value), getAuthPlatformDeployment()])
    rows.value = result.list
    total.value = result.total
    deployment.value = status
  } catch (error: unknown) {
    loadError.value = errorMessage(error, 'authPlatform.loadFailed')
  } finally {
    loading.value = false
  }
}

function search(): void {
  const normalizedKeyword = keyword.value.trim()
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(normalizedKeyword === '' ? {} : { keyword: normalizedKeyword }),
    ...(statusFilter.value === '' ? {} : { isEnabled: statusFilter.value }),
  }
  void loadPage()
}

function reset(): void {
  keyword.value = ''
  statusFilter.value = ''
  query.value = { page: 1, pageSize: query.value.pageSize }
  void loadPage()
}

function refresh(): void {
  void loadPage()
}

function changePage(page: number): void {
  query.value = { ...query.value, page }
  void loadPage()
}

function changePageSize(pageSize: number): void {
  query.value = { ...query.value, page: 1, pageSize }
  void loadPage()
}

function updateTablePagination(next: TablePaginationState): void {
  if (next.pageSize !== query.value.pageSize) { changePageSize(next.pageSize); return }
  changePage(next.currentPage)
}

function openCreate(): void {
  dialogMode.value = 'create'
  editingPlatform.value = null
  Object.assign(form, defaultForm())
  mutationError.value = ''
  dialogVisible.value = true
}

function openEdit(platform: AuthPlatformListItem): void {
  dialogMode.value = 'edit'
  editingPlatform.value = platform
  Object.assign(form, {
    code: platform.code,
    name: platform.name,
    accessTTLSeconds: platform.accessTTLSeconds,
    refreshTTLSeconds: platform.refreshTTLSeconds,
    sessionCacheTTLSeconds: platform.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: platform.accessCacheTTLSeconds,
    bindDevice: platform.bindDevice,
    bindIP: platform.bindIP,
    maxSessions: platform.maxSessions,
    allowRegister: platform.allowRegister,
    isEnabled: platform.isEnabled,
  })
  mutationError.value = ''
  dialogVisible.value = true
}

async function submit(): Promise<void> {
  if (!formValid.value || submitting.value) return
  if (dialogMode.value === 'edit' && editingPlatform.value !== null) {
    if (form.maxSessions < editingPlatform.value.maxSessions && form.maxSessions > 0 && !(await confirmAction('authPlatform.confirm.limit'))) return
    if (securityChanged(editingPlatform.value) && !(await confirmAction('authPlatform.confirm.security'))) return
  }
  submitting.value = true
  mutationError.value = ''
  try {
    if (dialogMode.value === 'create') {
      await createAuthPlatform(createInput())
      query.value = { ...query.value, page: 1 }
    } else if (editingPlatform.value !== null) {
      await updateAuthPlatform(editingPlatform.value.id, updateInput())
    }
    await loadPage()
    dialogVisible.value = false
    ElNotification.success({ title: t(dialogMode.value === 'create' ? 'authPlatform.success.created' : 'authPlatform.success.updated') })
  } catch (error: unknown) {
    mutationError.value = errorMessage(error, 'authPlatform.mutationFailed')
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(platform: AuthPlatformListItem): Promise<void> {
  if (!canStatus.value) return
  const next = platform.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes
  if (next === YesNo.No && !(await confirmAction('authPlatform.confirm.disable'))) return
  try {
    await updateAuthPlatformStatus(platform.id, next)
    await loadPage()
    ElNotification.success({ title: t('authPlatform.success.status') })
  } catch (error: unknown) {
    mutationError.value = errorMessage(error, 'authPlatform.mutationFailed')
  }
}

async function remove(platform: AuthPlatformListItem): Promise<void> {
  if (!canDelete.value || platform.isBuiltin === YesNo.Yes) return
  if (!(await confirmAction('authPlatform.confirm.delete'))) return
  try {
    await deleteAuthPlatform(platform.id)
    const maxPage = Math.max(1, Math.ceil((total.value - 1) / query.value.pageSize))
    if (query.value.page > maxPage) query.value = { ...query.value, page: maxPage }
    await loadPage()
    ElNotification.success({ title: t('authPlatform.success.deleted') })
  } catch (error: unknown) {
    mutationError.value = errorMessage(error, 'authPlatform.mutationFailed')
  }
}

async function confirmAction(key: 'authPlatform.confirm.disable' | 'authPlatform.confirm.delete' | 'authPlatform.confirm.limit' | 'authPlatform.confirm.security'): Promise<boolean> {
  try {
    await ElMessageBox.confirm(t(key), t('authPlatform.title'), { type: 'warning' })
    return true
  } catch (error: unknown) {
    return error !== 'cancel' && error !== 'close' ? false : false
  }
}

function securityChanged(platform: AuthPlatformListItem): boolean {
  return form.bindDevice !== platform.bindDevice || form.bindIP !== platform.bindIP ||
    form.accessTTLSeconds !== platform.accessTTLSeconds || form.refreshTTLSeconds !== platform.refreshTTLSeconds
}

function createInput(): CreateAuthPlatformInput {
  return {
    code: form.code.trim(), name: form.name.trim(), accessTTLSeconds: form.accessTTLSeconds,
    refreshTTLSeconds: form.refreshTTLSeconds, sessionCacheTTLSeconds: form.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: form.accessCacheTTLSeconds, bindDevice: form.bindDevice, bindIP: form.bindIP,
    maxSessions: form.maxSessions, allowRegister: form.allowRegister, isEnabled: form.isEnabled,
  }
}

function updateInput(): UpdateAuthPlatformInput {
  return {
    name: form.name.trim(), accessTTLSeconds: form.accessTTLSeconds, refreshTTLSeconds: form.refreshTTLSeconds,
    sessionCacheTTLSeconds: form.sessionCacheTTLSeconds, accessCacheTTLSeconds: form.accessCacheTTLSeconds,
    bindDevice: form.bindDevice, bindIP: form.bindIP, maxSessions: form.maxSessions, allowRegister: form.allowRegister,
  }
}

function sessionLabel(value: number): string {
  if (value === 0) return t('authPlatform.unlimited')
  if (value === 1) return t('authPlatform.singleSession')
  return t('authPlatform.maxSessions', { count: value })
}

function ttlLabel(value: number): string {
  if (value % 86_400 === 0) return `${t('authPlatform.seconds', { count: value })} · ${t('authPlatform.readableDays', { count: value / 86_400 })}`
  if (value % 3_600 === 0) return `${t('authPlatform.seconds', { count: value })} · ${t('authPlatform.readableHours', { count: value / 3_600 })}`
  if (value % 60 === 0) return `${t('authPlatform.seconds', { count: value })} · ${t('authPlatform.readableMinutes', { count: value / 60 })}`
  return t('authPlatform.seconds', { count: value })
}

function defaultForm(): AuthPlatformForm {
  return {
    code: '', name: '', accessTTLSeconds: 900, refreshTTLSeconds: 86_400,
    sessionCacheTTLSeconds: 7_200, accessCacheTTLSeconds: 600, bindDevice: YesNo.Yes,
    bindIP: YesNo.No, maxSessions: 1, allowRegister: YesNo.No, isEnabled: YesNo.Yes,
  }
}

function inRange(value: number, minimum: number, maximum: number): boolean {
  return Number.isInteger(value) && value >= minimum && value <= maximum
}

function errorMessage(error: unknown, fallbackKey: 'authPlatform.loadFailed' | 'authPlatform.mutationFailed'): string {
  return error instanceof Error && error.message !== '' ? error.message : t(fallbackKey)
}

onMounted(() => { void loadPage() })
</script>

<template>
	<section class="auth-platform-page management-page">
    <Search
      v-model="searchModel"
		class="auth-platform-filters management-page__filters"
      :fields="searchFields"
      :query-label="t('authPlatform.search')"
      :reset-label="t('authPlatform.reset')"
      query-test-id="auth-platform-search"
      reset-test-id="auth-platform-reset"
      @query="search"
      @reset="reset"
    />

    <el-alert v-if="loadError" :title="loadError" type="error" show-icon />
    <el-alert v-if="mutationError" :title="mutationError" type="error" show-icon closable @close="mutationError = ''" />

    <AppTable class="auth-platform-table" :columns="tableColumns" :data="rows" :loading="loading" :pagination="tablePagination" result-state="success" :aria-label="t('authPlatform.title')" :refresh-label="t('authPlatform.refresh')" @refresh="refresh" @update:pagination="updateTablePagination">
      <template #toolbar-right>
        <el-button v-if="canCreate" type="primary" :icon="CirclePlus" data-testid="auth-platform-create" @click="openCreate">{{ t('authPlatform.create') }}</el-button>
      </template>
      <template #cell-platform="{ row }: { row: AuthPlatformListItem }">
          <strong>{{ row.name }}</strong>
          <code>{{ row.code }}</code>
          <el-tag v-if="row.isBuiltin === YesNo.Yes" size="small" type="info">{{ t('authPlatform.builtin') }}</el-tag>
      </template>
      <template #cell-tokenTTL="{ row }: { row: AuthPlatformListItem }">{{ ttlLabel(row.accessTTLSeconds) }} / {{ ttlLabel(row.refreshTTLSeconds) }}</template>
      <template #cell-cacheTTL="{ row }: { row: AuthPlatformListItem }">{{ ttlLabel(row.sessionCacheTTLSeconds) }} / {{ ttlLabel(row.accessCacheTTLSeconds) }}</template>
      <template #cell-security="{ row }: { row: AuthPlatformListItem }">{{ row.bindDevice === YesNo.Yes ? t('authPlatform.bindDevice') : '' }} {{ row.bindIP === YesNo.Yes ? t('authPlatform.bindIP') : '' }}</template>
      <template #cell-sessions="{ row }: { row: AuthPlatformListItem }">{{ sessionLabel(row.maxSessions) }}</template>
      <template #cell-status="{ row }: { row: AuthPlatformListItem }"><el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'danger'">{{ t(row.isEnabled === YesNo.Yes ? 'authPlatform.enabled' : 'authPlatform.disabled') }}</el-tag></template>
      <template #cell-actions="{ row }: { row: AuthPlatformListItem }"><template v-if="row.id > 0">
          <el-button v-if="canUpdate" text type="primary" data-testid="auth-platform-update" @click="openEdit(row)">{{ t('authPlatform.edit') }}</el-button>
          <el-button v-if="canStatus" text type="warning" data-testid="auth-platform-status" @click="toggleStatus(row)">{{ t('authPlatform.statusAction') }}</el-button>
          <el-button v-if="canDelete && row.isBuiltin === YesNo.No" text type="danger" data-testid="auth-platform-delete" @click="remove(row)">{{ t('authPlatform.delete') }}</el-button>
      </template></template>
      <template #empty><el-empty :description="t('authPlatform.empty')" /></template>
    </AppTable>

    <section v-if="deployment !== null" class="auth-platform-deployment" aria-labelledby="auth-platform-deployment-title">
      <h2 id="auth-platform-deployment-title">{{ t('authPlatform.deployment') }}</h2>
      <dl>
        <div><dt>{{ t('authPlatform.cookieSecure') }}</dt><dd>{{ deployment.cookieSecure ? t('authPlatform.enabled') : t('authPlatform.disabled') }}</dd></div>
        <div><dt>{{ t('authPlatform.corsOrigin') }}</dt><dd>{{ deployment.corsOrigin }}</dd></div>
        <div><dt>{{ t('authPlatform.trustedProxy') }}</dt><dd>{{ deployment.trustedProxyMode }} ({{ deployment.trustedProxyCount }})</dd></div>
        <div><dt>{{ t('authPlatform.redis') }}</dt><dd>{{ t(deployment.redisStatus === 'up' ? 'authPlatform.up' : 'authPlatform.down') }}</dd></div>
      </dl>
    </section>

    <AppDialog v-model="dialogVisible" :title="t(dialogMode === 'create' ? 'authPlatform.createTitle' : 'authPlatform.editTitle')" width="560px" height="min(68vh, 680px)" :append-to-body="false">
      <div class="auth-platform-dialog-body">
        <el-form label-position="top">
          <el-form-item :label="t('authPlatform.code')"><el-input v-model="form.code" data-testid="auth-platform-code" :disabled="isEditing" /></el-form-item>
          <el-form-item :label="t('authPlatform.name')"><el-input v-model="form.name" data-testid="auth-platform-name" /></el-form-item>
          <el-form-item :label="t('authPlatform.accessTTL')"><el-input-number v-model="form.accessTTLSeconds" :min="60" :max="2_592_000" /></el-form-item>
          <el-form-item :label="t('authPlatform.refreshTTL')"><el-input-number v-model="form.refreshTTLSeconds" :min="60" :max="31_536_000" /></el-form-item>
          <el-form-item :label="t('authPlatform.sessionCacheTTL')"><el-input-number v-model="form.sessionCacheTTLSeconds" :min="60" :max="86_400" /></el-form-item>
          <el-form-item :label="t('authPlatform.accessCacheTTL')"><el-input-number v-model="form.accessCacheTTLSeconds" :min="60" :max="86_400" /></el-form-item>
          <el-form-item :label="t('authPlatform.bindDevice')"><el-switch v-model="form.bindDevice" :active-value="YesNo.Yes" :inactive-value="YesNo.No" /></el-form-item>
          <el-form-item :label="t('authPlatform.bindIP')"><el-switch v-model="form.bindIP" :active-value="YesNo.Yes" :inactive-value="YesNo.No" /></el-form-item>
          <el-form-item :label="t('authPlatform.maxSessionsField')"><el-input-number v-model="form.maxSessions" :min="0" :max="100" /></el-form-item>
          <el-form-item :label="t('authPlatform.allowRegister')"><el-switch v-model="form.allowRegister" :active-value="YesNo.Yes" :inactive-value="YesNo.No" /></el-form-item>
          <el-form-item v-if="dialogMode === 'create'" :label="t('authPlatform.isEnabled')"><el-switch v-model="form.isEnabled" :active-value="YesNo.Yes" :inactive-value="YesNo.No" /></el-form-item>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('authPlatform.cancel') }}</el-button>
        <el-button type="primary" :loading="submitting" :disabled="!formValid" @click="submit">{{ t('authPlatform.save') }}</el-button>
      </template>
    </AppDialog>
  </section>
</template>

<style scoped>
.auth-platform-page { min-width: 0; }
.auth-platform-filters { display: flex; align-items: center; gap: 12px; }
.auth-platform-filters .el-input { width: 260px; }
.auth-platform-filters .el-select-v2 { width: 150px; }
.auth-platform-table code { display: block; margin-top: 4px; color: var(--el-text-color-secondary); }
.auth-platform-table .el-tag { margin-left: 8px; }
.auth-platform-deployment { padding-top: 8px; border-top: 1px solid var(--el-border-color-light); }
.auth-platform-deployment h2 { margin: 0 0 12px; font-size: 16px; }
.auth-platform-deployment dl { display: flex; flex-wrap: wrap; gap: 16px 32px; margin: 0; }
.auth-platform-deployment dl div { min-width: 180px; }
.auth-platform-deployment dt { color: var(--el-text-color-secondary); font-size: 12px; }
.auth-platform-deployment dd { margin: 4px 0 0; }
.auth-platform-dialog-body { max-height: min(68vh, 680px); overflow-y: auto; padding-right: 8px; }
@media (max-width: 760px) { .auth-platform-filters { align-items: stretch; flex-direction: column; } .auth-platform-filters .el-input, .auth-platform-filters .el-select { width: 100%; } }
</style>
