<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { CirclePlus } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import {
  createRole,
  deleteRole,
  getRoles,
  setDefaultRole,
  updateRole,
  updateRoleStatus,
} from '@/api/permission/role'
import type { RoleListItem, RoleListQuery } from '@/api/permission/role'
import { YesNo } from '@/enums/yes-no'
import { usePermissionStore } from '@/store/permission'
import { AppTable } from '@/components/AppTable'
import type { TablePaginationState } from '@/components/AppTable'
import { AppSearch } from '@/components/AppSearch'
import type { SearchFormModel } from '@/components/AppSearch'
import RoleFormDialog from './components/RoleFormDialog/index.vue'
import RolePermissionDialog from './components/RolePermissionDialog/index.vue'
import type { RoleFormState } from './components/types'
import { formatRoleTime, roleSearchFields, roleTableColumns } from './role-view'

const { t } = useI18n()
const access = usePermissionStore()

const rows = ref<RoleListItem[]>([])
const total = ref(0)
const query = ref<RoleListQuery>({ page: 1, pageSize: 20 })
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
const searchFields = computed(() => roleSearchFields(t))

const submitting = ref(false)
const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingRole = ref<RoleListItem | null>(null)

const form = ref<RoleFormState>({ code: '', name: '' })

const permissionDialogVisible = ref(false)
const permissionTarget = ref<RoleListItem | null>(null)

const tablePagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const tableColumns = computed(() => roleTableColumns(t))

const canCreate = computed(() => access.hasPermission('permission:role:create'))
const canUpdate = computed(() => access.hasPermission('permission:role:update'))
const canStatus = computed(() => access.hasPermission('permission:role:status'))
const canDefault = computed(() => access.hasPermission('permission:role:default'))
const canDelete = computed(() => access.hasPermission('permission:role:delete'))
const canAuthorize = computed(() => access.hasPermission('permission:role:authorize'))

const formValid = computed(() => {
  const code = form.value.code.trim()
  const name = form.value.name.trim()

  return /^[a-z][a-z0-9_]{2,63}$/.test(code) && name.length > 0 && [...name].length <= 64
})

function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message !== '') {
    return error.message
  }
  return t('role.mutationFailed')
}

async function loadRoles(): Promise<boolean> {
  loading.value = true
  loadError.value = ''

  try {
    const result = await getRoles(query.value)
    rows.value = result.list
    total.value = result.total
    return true
  } catch (error: unknown) {
    loadError.value = errorMessage(error)
    return false
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
  void loadRoles()
}

function reset(): void {
  keyword.value = ''
  statusFilter.value = ''
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
  }
  void loadRoles()
}

function changePage(page: number): void {
  query.value = { ...query.value, page }
  void loadRoles()
}

function changePageSize(pageSize: number): void {
  query.value = { ...query.value, page: 1, pageSize }
  void loadRoles()
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
  editingRole.value = null
  form.value = { code: '', name: '' }
  mutationError.value = ''
  dialogVisible.value = true
}

function openEdit(role: RoleListItem): void {
  dialogMode.value = 'edit'
  editingRole.value = role
  form.value = { code: role.code, name: role.name }
  mutationError.value = ''
  dialogVisible.value = true
}

async function submitForm(): Promise<void> {
  if (!formValid.value || submitting.value) {
    return
  }

  submitting.value = true
  mutationError.value = ''

  try {
    if (dialogMode.value === 'create') {
      await createRole({
        code: form.value.code.trim(),
        name: form.value.name.trim(),
      })
      query.value = { ...query.value, page: 1 }
    } else if (editingRole.value !== null) {
      await updateRole(editingRole.value.id, {
        name: form.value.name.trim(),
      })
    }

    if (await loadRoles()) {
      dialogVisible.value = false
      const successKey =
        dialogMode.value === 'create' ? 'role.success.created' : 'role.success.updated'
      ElNotification.success({ title: t(successKey) })
    }
  } catch (error: unknown) {
    mutationError.value = errorMessage(error)
  } finally {
    submitting.value = false
  }
}

async function changeStatus(role: RoleListItem): Promise<void> {
  const next = role.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes
  const message =
    next === YesNo.Yes
      ? t('role.confirm.enableMessage', { count: role.userCount })
      : t('role.confirm.disableMessage', { count: role.userCount })
  const title = t(next === YesNo.Yes ? 'role.action.enable' : 'role.action.disable')

  try {
    await ElMessageBox.confirm(message, title, { type: 'warning' })
    await updateRoleStatus(role.id, next)
    await loadRoles()
    ElNotification.success({ title: t('role.success.status') })
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') {
      mutationError.value = errorMessage(error)
    }
  }
}

async function makeDefault(role: RoleListItem): Promise<void> {
  const zeroPermissionWarning = role.permissionCount === 0 ? t('role.warning.zeroPermission') : ''
  const message = `${t('role.confirm.defaultMessage')} ${zeroPermissionWarning}`

  try {
    await ElMessageBox.confirm(message, t('role.action.default'), {
      type: 'warning',
    })
    await setDefaultRole(role.id)
    await loadRoles()
    ElNotification.success({ title: t('role.success.default') })
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') {
      mutationError.value = errorMessage(error)
    }
  }
}

async function removeRole(role: RoleListItem): Promise<void> {
  try {
    await ElMessageBox.confirm(t('role.confirm.deleteMessage'), t('role.action.delete'), {
      type: 'warning',
    })
    await deleteRole(role.id)

    const maxPage = Math.max(1, Math.ceil((total.value - 1) / query.value.pageSize))
    if (query.value.page > maxPage) {
      query.value = { ...query.value, page: maxPage }
    }

    await loadRoles()
    ElNotification.success({ title: t('role.success.deleted') })
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') {
      mutationError.value = errorMessage(error)
    }
  }
}

function openPermissions(role: RoleListItem): void {
  permissionTarget.value = role
  permissionDialogVisible.value = true
}

async function handlePermissionsSaved(): Promise<void> {
  if (await loadRoles()) ElNotification.success({ title: t('role.success.authorized') })
}

function isSystem(role: RoleListItem): boolean {
  return role.code === 'super_admin' || role.code === 'registered_user'
}

function formatTime(value: string): string {
  return formatRoleTime(value)
}

function editTooltip(role: RoleListItem): string {
  return isSystem(role) ? t('role.protection.systemName') : t('role.action.edit')
}

function statusTooltip(role: RoleListItem): string {
  if (role.code === 'super_admin') {
    return t('role.protection.superAdminEnabled')
  }
  if (role.isDefault === YesNo.Yes) {
    return t('role.protection.defaultEnabled')
  }
  return t(role.isEnabled === YesNo.Yes ? 'role.action.disable' : 'role.action.enable')
}

function defaultTooltip(role: RoleListItem): string {
  if (role.code === 'super_admin') {
    return t('role.protection.superAdminDefault')
  }
  if (role.isDefault === YesNo.Yes) {
    return t('role.protection.alreadyDefault')
  }
  if (role.isEnabled === YesNo.No) {
    return t('role.protection.disabledDefault')
  }
  return t('role.action.default')
}

function deleteTooltip(role: RoleListItem): string {
  if (isSystem(role)) {
    return t('role.protection.systemDelete')
  }
  if (role.isDefault === YesNo.Yes) {
    return t('role.protection.defaultDelete')
  }
  if (role.userCount > 0) {
    return t('role.protection.usersDelete', { count: role.userCount })
  }
  return t('role.action.delete')
}

onMounted(() => {
  void loadRoles()
})
</script>

<template>
  <section class="role-page management-page">
    <AppSearch
      v-model="searchModel"
      class="role-filters management-page__filters"
      :fields="searchFields"
      :query-label="t('role.search')"
      :reset-label="t('role.reset')"
      query-test-id="role-search"
      reset-test-id="role-reset"
      @query="search"
      @reset="reset"
    />

    <el-alert v-if="loadError" :title="loadError" type="error" show-icon>
      <el-button text @click="loadRoles">
        {{ t('role.retry') }}
      </el-button>
    </el-alert>
    <el-alert
      v-if="mutationError"
      :title="mutationError"
      type="error"
      show-icon
      closable
      @close="mutationError = ''"
    />

    <AppTable
      class="role-table"
      :columns="tableColumns"
      :data="rows"
      :loading="loading"
      :pagination="tablePagination"
      :aria-label="t('role.title')"
      :refresh-label="t('role.refresh')"
      @refresh="loadRoles"
      @update:pagination="updateTablePagination"
    >
      <template #toolbar-right>
        <el-button v-if="canCreate" type="primary" :icon="CirclePlus" @click="openCreate">
          {{ t('role.create') }}
        </el-button>
      </template>
      <template #cell-default="{ row }: { row: RoleListItem }">
        <el-tag :type="row.isDefault === YesNo.Yes ? 'success' : 'info'">
          {{ t(row.isDefault === YesNo.Yes ? 'role.default.yes' : 'role.default.no') }}
        </el-tag>
      </template>
      <template #cell-status="{ row }: { row: RoleListItem }">
        <el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'danger'">
          {{ t(row.isEnabled === YesNo.Yes ? 'role.status.enabled' : 'role.status.disabled') }}
        </el-tag>
      </template>
      <template #cell-createdAt="{ row }: { row: RoleListItem }">{{
        formatTime(row.createdAt)
      }}</template>
      <template #cell-updatedAt="{ row }: { row: RoleListItem }">{{
        formatTime(row.updatedAt)
      }}</template>
      <template #cell-actions="{ row }: { row: RoleListItem }">
        <template v-if="row.id > 0">
          <el-space wrap :size="6">
            <el-tooltip v-if="canUpdate" :content="editTooltip(row)">
              <el-button text type="primary" :disabled="isSystem(row)" @click="openEdit(row)">{{
                t('role.action.edit')
              }}</el-button>
            </el-tooltip>
            <el-tooltip v-if="canStatus" :content="statusTooltip(row)">
              <el-button
                text
                type="warning"
                :disabled="row.code === 'super_admin' || row.isDefault === YesNo.Yes"
                @click="changeStatus(row)"
                >{{
                  row.isEnabled === YesNo.Yes ? t('role.action.disable') : t('role.action.enable')
                }}</el-button
              >
            </el-tooltip>
            <el-tooltip v-if="canDefault" :content="defaultTooltip(row)">
              <el-button
                text
                type="success"
                :disabled="
                  row.code === 'super_admin' ||
                  row.isDefault === YesNo.Yes ||
                  row.isEnabled === YesNo.No
                "
                @click="makeDefault(row)"
                >{{ t('role.action.default') }}</el-button
              >
            </el-tooltip>
            <el-tooltip
              v-if="canAuthorize && row.code !== 'super_admin'"
              :content="t('role.action.authorize')"
            >
              <el-button text type="primary" @click="openPermissions(row)">{{
                t('role.action.authorize')
              }}</el-button>
            </el-tooltip>
            <el-tooltip v-if="canDelete" :content="deleteTooltip(row)">
              <el-button
                text
                type="danger"
                :disabled="isSystem(row) || row.isDefault === YesNo.Yes || row.userCount > 0"
                @click="removeRole(row)"
                >{{ t('role.action.delete') }}</el-button
              >
            </el-tooltip>
          </el-space>
        </template>
      </template>
      <template #empty
        ><div class="role-empty">{{ t('role.empty') }}</div></template
      >
    </AppTable>

    <RoleFormDialog
      v-model="dialogVisible"
      v-model:form="form"
      :editing="dialogMode === 'edit'"
      :mutation-error="mutationError"
      :submitting="submitting"
      :form-valid="formValid"
      @save="submitForm"
    />
    <RolePermissionDialog
      v-model="permissionDialogVisible"
      :role="permissionTarget"
      @saved="handlePermissionsSaved"
    />
  </section>
</template>

<style scoped src="./RolePage.css"></style>
