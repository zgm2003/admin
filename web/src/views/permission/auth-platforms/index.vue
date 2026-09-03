<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { CirclePlus } from '@element-plus/icons-vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  createAuthPlatform,
  deleteAuthPlatform,
  getAuthPlatforms,
  updateAuthPlatform,
  updateAuthPlatformStatus,
} from '@/api/auth/platform'
import type { AuthPlatformListItem, AuthPlatformListQuery } from '@/api/auth/platform'
import { YesNo } from '@/enums/yes-no'
import { usePermissionStore } from '@/store/permission'
import { AppTable } from '@/components/AppTable'
import type { TablePaginationState } from '@/components/AppTable'
import { AppSearch } from '@/components/AppSearch'
import type { SearchFormModel } from '@/components/AppSearch'
import AuthPlatformDialog from './components/AuthPlatformDialog/index.vue'
import type { AuthPlatformForm } from './components/AuthPlatformDialog/types'
import {
  authPlatformDefaultTTL,
  authPlatformSecurityChanged,
  createAuthPlatformForm,
  createAuthPlatformInput,
  editAuthPlatformForm,
  isAuthPlatformFormValid,
  updateAuthPlatformInput,
} from './auth-platform-form'
import {
  authPlatformSearchFields,
  authPlatformSessionLabel,
  authPlatformTableColumns,
  authPlatformTTLLabel,
  formatAuthPlatformDate,
  formatAuthPlatformTime,
} from './auth-platform-view'

const { t, locale } = useI18n()
const access = usePermissionStore()

const rows = ref<AuthPlatformListItem[]>([])
const total = ref(0)
const query = ref<AuthPlatformListQuery>({ page: 1, pageSize: 20 })
const keyword = ref('')
const statusFilter = ref<'' | YesNo>('')
const loading = ref(false)
const loadError = ref('')
const mutationError = ref('')
const searchModel = computed<SearchFormModel>({
  get: () => ({ keyword: keyword.value, status: statusFilter.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    statusFilter.value = value.status === YesNo.Yes || value.status === YesNo.No ? value.status : ''
  },
})
const searchFields = computed(() => authPlatformSearchFields(t))

const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingPlatform = ref<AuthPlatformListItem | null>(null)
const submitting = ref(false)

const tablePagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const tableColumns = computed(() => authPlatformTableColumns(t))
const form = reactive<AuthPlatformForm>(createAuthPlatformForm())

const canList = computed(() => access.hasPermission('auth:platform:list'))
const canCreate = computed(() => access.hasPermission('auth:platform:create'))
const canUpdate = computed(() => access.hasPermission('auth:platform:update'))
const canStatus = computed(() => access.hasPermission('auth:platform:status'))
const canDelete = computed(() => access.hasPermission('auth:platform:delete'))
const isEditing = computed(() => dialogMode.value === 'edit')
const isBuiltinAdminEdit = computed(() => {
  const platform = editingPlatform.value
  return (
    dialogMode.value === 'edit' && platform?.code === 'admin' && platform.isBuiltin === YesNo.Yes
  )
})
const formValid = computed(() => isAuthPlatformFormValid(form, isEditing.value))

async function loadPage(): Promise<void> {
  if (!canList.value) return
  loading.value = true
  loadError.value = ''
  try {
    const result = await getAuthPlatforms(query.value)
    rows.value = result.list
    total.value = result.total
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
  if (next.pageSize !== query.value.pageSize) {
    changePageSize(next.pageSize)
    return
  }
  changePage(next.currentPage)
}

function openCreate(): void {
  dialogMode.value = 'create'
  editingPlatform.value = null
  Object.assign(form, createAuthPlatformForm())
  mutationError.value = ''
  dialogVisible.value = true
}

function openEdit(platform: AuthPlatformListItem): void {
  dialogMode.value = 'edit'
  editingPlatform.value = platform
  Object.assign(form, editAuthPlatformForm(platform))
  mutationError.value = ''
  dialogVisible.value = true
}

async function submit(): Promise<void> {
  if (!formValid.value || submitting.value) return
  if (dialogMode.value === 'edit' && editingPlatform.value !== null) {
    if (
      form.maxSessions < editingPlatform.value.maxSessions &&
      form.maxSessions > 0 &&
      !(await confirmAction('authPlatform.confirm.limit'))
    )
      return
    if (
      authPlatformSecurityChanged(form, editingPlatform.value) &&
      !(await confirmAction('authPlatform.confirm.security'))
    )
      return
  }
  submitting.value = true
  mutationError.value = ''
  try {
    if (dialogMode.value === 'create') {
      await createAuthPlatform(createAuthPlatformInput(form))
      query.value = { ...query.value, page: 1 }
    } else if (editingPlatform.value !== null) {
      await updateAuthPlatform(
        editingPlatform.value.id,
        updateAuthPlatformInput(form, isBuiltinAdminEdit.value),
      )
    }
    await loadPage()
    dialogVisible.value = false
    ElNotification.success({
      title: t(
        dialogMode.value === 'create'
          ? 'authPlatform.success.created'
          : 'authPlatform.success.updated',
      ),
    })
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

async function confirmAction(
  key:
    | 'authPlatform.confirm.disable'
    | 'authPlatform.confirm.delete'
    | 'authPlatform.confirm.limit'
    | 'authPlatform.confirm.security',
): Promise<boolean> {
  try {
    await ElMessageBox.confirm(t(key), t('authPlatform.title'), {
      type: 'warning',
    })
    return true
  } catch (error: unknown) {
    return error !== 'cancel' && error !== 'close' ? false : false
  }
}

function sessionLabel(value: number): string {
  return authPlatformSessionLabel(value, t)
}

function ttlLabel(value: number): string {
  return authPlatformTTLLabel(value, t)
}

function formatUpdatedDate(value: string): string {
  return formatAuthPlatformDate(value, locale.value)
}

function formatUpdatedTime(value: string): string {
  return formatAuthPlatformTime(value, locale.value)
}

function restoreDefaultTTL(): void {
  Object.assign(form, authPlatformDefaultTTL)
}

function errorMessage(
  error: unknown,
  fallbackKey: 'authPlatform.loadFailed' | 'authPlatform.mutationFailed',
): string {
  return error instanceof Error && error.message !== '' ? error.message : t(fallbackKey)
}

onMounted(() => {
  void loadPage()
})
</script>

<template>
  <section class="auth-platform-page management-page">
    <AppSearch
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
    <el-alert
      v-if="mutationError"
      :title="mutationError"
      type="error"
      show-icon
      closable
      @close="mutationError = ''"
    />

    <AppTable
      class="auth-platform-table"
      :columns="tableColumns"
      :data="rows"
      :loading="loading"
      :pagination="tablePagination"
      result-state="success"
      :aria-label="t('authPlatform.title')"
      :refresh-label="t('authPlatform.refresh')"
      @refresh="refresh"
      @update:pagination="updateTablePagination"
    >
      <template #toolbar-left>
        <el-button
          v-if="canCreate"
          type="primary"
          :icon="CirclePlus"
          data-testid="auth-platform-create"
          @click="openCreate"
          >{{ t('authPlatform.create') }}</el-button
        >
      </template>
      <template #cell-platform="{ row }: { row: AuthPlatformListItem }">
        <div class="auth-platform-identity auth-platform-identity--centered">
          <strong>{{ row.name }}</strong>
          <div class="auth-platform-identity__meta">
            <code>{{ row.code }}</code>
            <el-tag v-if="row.isBuiltin === YesNo.Yes" size="small" type="info" effect="plain">{{
              t('authPlatform.builtin')
            }}</el-tag>
          </div>
        </div>
      </template>
      <template #cell-tokenTTL="{ row }: { row: AuthPlatformListItem }">
        <div class="auth-platform-policy-stack">
          <div>
            <span>{{ t('authPlatform.accessToken') }}</span
            ><strong :title="t('authPlatform.seconds', { count: row.accessTTLSeconds })">{{
              ttlLabel(row.accessTTLSeconds)
            }}</strong>
          </div>
          <div>
            <span>{{ t('authPlatform.refreshToken') }}</span
            ><strong :title="t('authPlatform.seconds', { count: row.refreshTTLSeconds })">{{
              ttlLabel(row.refreshTTLSeconds)
            }}</strong>
          </div>
        </div>
      </template>
      <template #cell-cacheTTL="{ row }: { row: AuthPlatformListItem }">
        <div class="auth-platform-policy-stack">
          <div>
            <span>{{ t('authPlatform.sessionCache') }}</span
            ><strong :title="t('authPlatform.seconds', { count: row.sessionCacheTTLSeconds })">{{
              ttlLabel(row.sessionCacheTTLSeconds)
            }}</strong>
          </div>
          <div>
            <span>{{ t('authPlatform.accessCache') }}</span
            ><strong :title="t('authPlatform.seconds', { count: row.accessCacheTTLSeconds })">{{
              ttlLabel(row.accessCacheTTLSeconds)
            }}</strong>
          </div>
        </div>
      </template>
      <template #cell-security="{ row }: { row: AuthPlatformListItem }">
        <div class="auth-platform-tag-list" data-testid="auth-platform-security">
          <el-tag
            size="small"
            effect="plain"
            :type="row.bindDevice === YesNo.Yes ? 'success' : 'info'"
            >{{
              t(
                row.bindDevice === YesNo.Yes
                  ? 'authPlatform.deviceBound'
                  : 'authPlatform.deviceUnbound',
              )
            }}</el-tag
          >
          <el-tag
            size="small"
            effect="plain"
            :type="row.bindIP === YesNo.Yes ? 'success' : 'info'"
            >{{
              t(row.bindIP === YesNo.Yes ? 'authPlatform.ipBound' : 'authPlatform.ipUnbound')
            }}</el-tag
          >
        </div>
      </template>
      <template #cell-sessions="{ row }: { row: AuthPlatformListItem }">
        <el-tag size="small" effect="plain">{{ sessionLabel(row.maxSessions) }}</el-tag>
      </template>
      <template #cell-registration="{ row }: { row: AuthPlatformListItem }">
        <el-tag
          size="small"
          effect="plain"
          :type="row.allowRegister === YesNo.Yes ? 'success' : 'info'"
          data-testid="auth-platform-registration"
          >{{
            t(
              row.allowRegister === YesNo.Yes
                ? 'authPlatform.registrationAllowed'
                : 'authPlatform.registrationDenied',
            )
          }}</el-tag
        >
      </template>
      <template #cell-status="{ row }: { row: AuthPlatformListItem }"
        ><el-tag size="small" :type="row.isEnabled === YesNo.Yes ? 'success' : 'danger'">{{
          t(row.isEnabled === YesNo.Yes ? 'authPlatform.enabled' : 'authPlatform.disabled')
        }}</el-tag></template
      >
      <template #cell-updatedAt="{ row }: { row: AuthPlatformListItem }">
        <time class="auth-platform-updated" :datetime="row.updatedAt">
          <span>{{ formatUpdatedDate(row.updatedAt) }}</span>
          <small>{{ formatUpdatedTime(row.updatedAt) }}</small>
        </time>
      </template>
      <template #cell-actions="{ row }: { row: AuthPlatformListItem }"
        ><template v-if="row.id > 0">
          <el-button
            v-if="canUpdate"
            text
            type="primary"
            data-testid="auth-platform-update"
            @click="openEdit(row)"
            >{{ t('authPlatform.edit') }}</el-button
          >
          <el-button
            v-if="canStatus"
            text
            type="warning"
            data-testid="auth-platform-status"
            @click="toggleStatus(row)"
            >{{
              t(row.isEnabled === YesNo.Yes ? 'authPlatform.disable' : 'authPlatform.enable')
            }}</el-button
          >
          <el-button
            v-if="canDelete && row.isBuiltin === YesNo.No"
            text
            type="danger"
            data-testid="auth-platform-delete"
            @click="remove(row)"
            >{{ t('authPlatform.delete') }}</el-button
          >
        </template></template
      >
      <template #empty><el-empty :description="t('authPlatform.empty')" /></template>
    </AppTable>

    <AuthPlatformDialog
      v-model="dialogVisible"
      v-model:form="form"
      :dialog-mode="dialogMode"
      :is-editing="isEditing"
      :is-builtin-admin-edit="isBuiltinAdminEdit"
      :submitting="submitting"
      :form-valid="formValid"
      @restore-defaults="restoreDefaultTTL"
      @save="submit"
    />
  </section>
</template>

<style scoped src="./AuthPlatformPage.css"></style>
