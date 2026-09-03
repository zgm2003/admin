<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  deleteUser,
  getUserRoleOptions,
  getUserRoles,
  getUsers,
  updateUser,
  updateUserRoles,
  updateUserStatus,
} from '@/api/user/account'
import type {
  UserListItem,
  UserListQuery,
  UserRolesResponse,
  UserRoleSummary,
} from '@/api/user/account'
import { YesNo } from '@/enums/yes-no'
import { usePermissionStore } from '@/store/permission'
import { useAuthStore } from '@/store/auth'
import { AppTable } from '@/components/AppTable'
import type { TablePaginationState } from '@/components/AppTable'
import { AppSearch } from '@/components/AppSearch'
import type { SearchFormModel } from '@/components/AppSearch'
import UserEditDialog from './components/UserEditDialog/index.vue'
import UserRoleDialog from './components/UserRoleDialog/index.vue'
import type { UserFormState } from './components/types'
import {
  hasSuperAdminRole,
  isPhoneValid,
  isProtectedTarget,
  isRoleToggleDisabled,
  isUsernameValid,
  normalizedPhone,
  normalizedUsername,
  protectedRoleIDs,
  userSearchFields,
  userTableColumns,
} from './user-rules'

const { t } = useI18n()
const access = usePermissionStore()
const auth = useAuthStore()

const rows = ref<UserListItem[]>([])
const roleOptions = ref<UserRoleSummary[]>([])
const total = ref(0)
const query = ref<UserListQuery>({ page: 1, pageSize: 20 })
const keyword = ref('')
const statusFilter = ref<'' | YesNo>('')
const roleFilter = ref<'' | number>('')
const loading = ref(false)
const roleOptionsLoading = ref(false)
const loadError = ref('')
const roleOptionsError = ref('')
const mutationError = ref('')
const mutating = ref(false)

const editVisible = ref(false)
const editingUser = ref<UserListItem | null>(null)
const userForm = ref<UserFormState>({ username: '', phone: '' })
const editSaving = ref(false)
const editError = ref('')

const roleDialogVisible = ref(false)
const roleTarget = ref<UserListItem | null>(null)
const roleData = ref<UserRolesResponse | null>(null)
const selectedRoleIDs = ref<number[]>([])
const roleLoading = ref(false)
const roleSaving = ref(false)
const roleError = ref('')

const tablePagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const tableColumns = computed(() => userTableColumns(t))

const canUpdate = computed(() => access.hasPermission('account:user:update'))
const canStatus = computed(() => access.hasPermission('account:user:status'))
const canDelete = computed(() => access.hasPermission('account:user:delete'))
const canRoles = computed(() => access.hasPermission('account:user:roles'))
const isSuperAdminActor = computed(() => access.roleCodes.includes('super_admin'))
const normalizedUsernameValue = computed(() => normalizedUsername(userForm.value.username))
const normalizedPhoneValue = computed(() => normalizedPhone(userForm.value.phone))
const usernameValid = computed(() => isUsernameValid(userForm.value.username))
const phoneValid = computed(() => isPhoneValid(userForm.value.phone))
const submittedPhone = computed<string | null>(() =>
  normalizedPhoneValue.value === '' ? null : normalizedPhoneValue.value,
)
const hasEnabledSelection = computed(() => {
  if (roleData.value === null) return false
  const selected = new Set(selectedRoleIDs.value)
  return roleData.value.roles.some((role) => role.isEnabled === YesNo.Yes && selected.has(role.id))
})

const searchModel = computed<SearchFormModel>({
  get: () => ({
    keyword: keyword.value,
    status: statusFilter.value,
    role: roleFilter.value,
  }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    statusFilter.value = value.status === YesNo.Yes || value.status === YesNo.No ? value.status : ''
    roleFilter.value = typeof value.role === 'number' ? value.role : ''
  },
})
const searchFields = computed(() => userSearchFields(t, roleOptions.value))

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message !== '' ? error.message : t(fallback)
}
async function loadUsers(): Promise<boolean> {
  if (loading.value) return false
  loading.value = true
  loadError.value = ''
  try {
    const result = await getUsers(query.value)
    rows.value = result.list
    total.value = result.total
    return true
  } catch (error: unknown) {
    loadError.value = errorMessage(error, 'user.loadFailed')
    return false
  } finally {
    loading.value = false
  }
}
async function loadRoleOptions(): Promise<void> {
  if (roleOptionsLoading.value) return
  roleOptionsLoading.value = true
  roleOptionsError.value = ''
  try {
    roleOptions.value = (await getUserRoleOptions()).roles
  } catch (error: unknown) {
    roleOptionsError.value = errorMessage(error, 'user.roleLoadFailed')
  } finally {
    roleOptionsLoading.value = false
  }
}
function search(): void {
  const value = keyword.value.trim()
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(value === '' ? {} : { keyword: value }),
    ...(statusFilter.value === '' ? {} : { isEnabled: statusFilter.value }),
    ...(roleFilter.value === '' ? {} : { roleId: roleFilter.value }),
  }
  void loadUsers()
}
function reset(): void {
  keyword.value = ''
  statusFilter.value = ''
  roleFilter.value = ''
  query.value = { page: 1, pageSize: query.value.pageSize }
  void loadUsers()
}
function changePage(page: number): void {
  query.value = { ...query.value, page }
  void loadUsers()
}
function changePageSize(pageSize: number): void {
  query.value = { ...query.value, page: 1, pageSize }
  void loadUsers()
}
function updateTablePagination(next: TablePaginationState): void {
  if (next.pageSize !== query.value.pageSize) {
    changePageSize(next.pageSize)
    return
  }
  changePage(next.currentPage)
}
function isSelf(row: UserListItem): boolean {
  return auth.user?.userId === row.id
}
function hasSuperAdmin(row: UserListItem): boolean {
  return hasSuperAdminRole(row)
}
function targetProtected(row: UserListItem): boolean {
  return isProtectedTarget(row, isSuperAdminActor.value)
}
function editDisabled(row: UserListItem): boolean {
  return targetProtected(row)
}
function dangerDisabled(row: UserListItem): boolean {
  return isSelf(row) || targetProtected(row)
}
function protectionText(row: UserListItem, operation: 'status' | 'roles' | 'delete'): string {
  if (targetProtected(row)) return t('user.superAdminBlocked')
  if (isSelf(row))
    return t(
      operation === 'status'
        ? 'user.selfStatusBlocked'
        : operation === 'roles'
          ? 'user.selfRolesBlocked'
          : 'user.selfDeleteBlocked',
    )
  return operation === 'roles'
    ? t('user.assignRoles')
    : operation === 'delete'
      ? t('permission.userDelete')
      : t('user.status')
}

function openEdit(row: UserListItem): void {
  if (editDisabled(row)) return
  editingUser.value = row
  userForm.value = { username: row.username, phone: row.phone ?? '' }
  editError.value = ''
  editVisible.value = true
}
async function saveEdit(): Promise<void> {
  const target = editingUser.value
  if (target === null || !usernameValid.value || !phoneValid.value || editSaving.value) return
  editSaving.value = true
  editError.value = ''
  try {
    const result = await updateUser(target.id, {
      username: normalizedUsernameValue.value,
      phone: submittedPhone.value,
    })
    auth.updateProfile(result.id, result.username, result.phone)
    if (await loadUsers()) {
      editVisible.value = false
      ElNotification.success({ title: t('user.updateSuccess') })
    }
  } catch (error: unknown) {
    editError.value = errorMessage(error, 'user.saveFailed')
  } finally {
    editSaving.value = false
  }
}

async function openRoles(row: UserListItem): Promise<void> {
  if (dangerDisabled(row)) return
  roleTarget.value = row
  roleDialogVisible.value = true
  roleLoading.value = true
  roleError.value = ''
  roleData.value = null
  selectedRoleIDs.value = []
  try {
    const data = await getUserRoles(row.id)
    roleData.value = data
    selectedRoleIDs.value = [...data.roleIds]
  } catch (error: unknown) {
    roleError.value = errorMessage(error, 'user.roleLoadFailed')
  } finally {
    roleLoading.value = false
  }
}
function protectedSelectedRoleIDs(): number[] {
  return protectedRoleIDs(roleData.value, isSuperAdminActor.value)
}
function selectAllRoles(): void {
  if (roleData.value === null) return
  const protectedIDs = new Set(protectedSelectedRoleIDs())
  selectedRoleIDs.value = [
    ...roleData.value.roles
      .filter((role) => isSuperAdminActor.value || role.code !== 'super_admin')
      .map((role) => role.id),
    ...protectedIDs,
  ].sort((a, b) => a - b)
}
function clearRoles(): void {
  selectedRoleIDs.value = protectedSelectedRoleIDs()
}
function roleToggleDisabled(role: UserRoleSummary): boolean {
  return isRoleToggleDisabled(role, isSuperAdminActor.value)
}
async function saveRoles(): Promise<void> {
  const target = roleTarget.value
  if (target === null || roleData.value === null || !hasEnabledSelection.value || roleSaving.value)
    return
  roleSaving.value = true
  roleError.value = ''
  try {
    const roleIds = [...new Set(selectedRoleIDs.value)].sort((a, b) => a - b)
    await updateUserRoles(target.id, { roleIds })
    if (await loadUsers()) {
      roleDialogVisible.value = false
      ElNotification.success({ title: t('user.rolesSuccess') })
    }
  } catch (error: unknown) {
    roleError.value = errorMessage(error, 'user.saveFailed')
  } finally {
    roleSaving.value = false
  }
}

async function changeStatus(row: UserListItem): Promise<void> {
  if (dangerDisabled(row) || mutating.value) return
  const next = row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes
  let message = t(next === YesNo.No ? 'user.disableConfirm' : 'user.enableConfirm')
  if (hasSuperAdmin(row)) message += ` ${t('user.superAdminImpact')}`
  try {
    await ElMessageBox.confirm(message, t('user.status'), { type: 'warning' })
    mutating.value = true
    await updateUserStatus(row.id, next)
    await loadUsers()
    ElNotification.success({ title: t('user.statusSuccess') })
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close')
      mutationError.value = errorMessage(error, 'user.saveFailed')
  } finally {
    mutating.value = false
  }
}
async function removeUser(row: UserListItem): Promise<void> {
  if (dangerDisabled(row) || mutating.value) return
  let message = t('user.deleteConfirm')
  if (hasSuperAdmin(row)) message += ` ${t('user.superAdminImpact')}`
  try {
    await ElMessageBox.confirm(message, t('permission.userDelete'), {
      type: 'warning',
    })
    mutating.value = true
    await deleteUser(row.id)
    const maxPage = Math.max(1, Math.ceil((total.value - 1) / query.value.pageSize))
    if (query.value.page > maxPage) query.value = { ...query.value, page: maxPage }
    await loadUsers()
    ElNotification.success({ title: t('user.deleteSuccess') })
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close')
      mutationError.value = errorMessage(error, 'user.saveFailed')
  } finally {
    mutating.value = false
  }
}

onMounted(() => {
  void loadRoleOptions()
  void loadUsers()
})
</script>

<template>
  <section class="user-management management-page">
    <AppSearch
      v-model="searchModel"
      class="user-filters management-page__filters"
      :fields="searchFields"
      :query-label="t('user.search')"
      :reset-label="t('user.reset')"
      query-test-id="user-search"
      reset-test-id="user-reset"
      @query="search"
      @reset="reset"
    />
    <el-alert v-if="roleOptionsError" :title="roleOptionsError" type="error" show-icon /><el-alert
      v-if="loadError"
      :title="loadError"
      type="error"
      show-icon
    /><el-alert
      v-if="mutationError"
      :title="mutationError"
      type="error"
      show-icon
      closable
      @close="mutationError = ''"
    />
    <AppTable
      class="user-table"
      :columns="tableColumns"
      :data="rows"
      :loading="loading"
      :pagination="tablePagination"
      :aria-label="t('user.title')"
      :refresh-label="t('user.refresh')"
      @refresh="loadUsers"
      @update:pagination="updateTablePagination"
    >
      <template #cell-roles="{ row }: { row: UserListItem }">
        <div v-if="row.id > 0" class="role-tags">
          <el-tooltip v-for="role in row.roles" :key="role.id" :content="role.code"
            ><el-tag :type="role.isEnabled === YesNo.Yes ? 'primary' : 'info'"
              >{{ role.name
              }}<span v-if="role.isEnabled === YesNo.No">
                · {{ t('user.roleDisabled') }}</span
              ></el-tag
            ></el-tooltip
          >
        </div>
      </template>
      <template #cell-status="{ row }: { row: UserListItem }"
        ><el-tag v-if="row.id > 0" :type="row.isEnabled === YesNo.Yes ? 'success' : 'danger'">{{
          t(row.isEnabled === YesNo.Yes ? 'user.enabled' : 'user.disabled')
        }}</el-tag></template
      >
      <template #cell-phone="{ row }: { row: UserListItem }">
        {{ row.phone ?? '-' }}
      </template>
      <template #cell-actions="{ row }: { row: UserListItem }"
        ><template v-if="row.id > 0">
          <el-space wrap :size="6">
            <el-tooltip
              v-if="canUpdate"
              :content="editDisabled(row) ? t('user.superAdminBlocked') : t('user.edit')"
              ><el-button
                text
                type="primary"
                :disabled="editDisabled(row)"
                @click="openEdit(row)"
                >{{ t('user.edit') }}</el-button
              ></el-tooltip
            >
            <el-tooltip v-if="canStatus" :content="protectionText(row, 'status')"
              ><el-button
                text
                type="warning"
                :disabled="dangerDisabled(row) || mutating"
                @click="changeStatus(row)"
                >{{
                  row.isEnabled === YesNo.Yes ? t('user.disabled') : t('user.enabled')
                }}</el-button
              ></el-tooltip
            >
            <el-tooltip v-if="canRoles" :content="protectionText(row, 'roles')"
              ><el-button
                text
                type="primary"
                :disabled="dangerDisabled(row)"
                @click="openRoles(row)"
                >{{ t('user.assignRoles') }}</el-button
              ></el-tooltip
            >
            <el-tooltip v-if="canDelete" :content="protectionText(row, 'delete')"
              ><el-button
                text
                type="danger"
                :disabled="dangerDisabled(row) || mutating"
                @click="removeUser(row)"
                >{{ t('permission.userDelete') }}</el-button
              ></el-tooltip
            >
          </el-space>
        </template></template
      >
      <template #empty><el-empty :description="t('user.noRoles')" /></template>
    </AppTable>

    <UserEditDialog
      v-model="editVisible"
      v-model:form="userForm"
      :editing-user="editingUser"
      :edit-error="editError"
      :edit-saving="editSaving"
      :username-valid="usernameValid"
      :phone-valid="phoneValid"
      @save="saveEdit"
    />
    <UserRoleDialog
      v-model="roleDialogVisible"
      v-model:selected-role-i-ds="selectedRoleIDs"
      :role-data="roleData"
      :role-loading="roleLoading"
      :role-error="roleError"
      :role-saving="roleSaving"
      :has-enabled-selection="hasEnabledSelection"
      :role-toggle-disabled="roleToggleDisabled"
      @select-all="selectAllRoles"
      @clear="clearRoles"
      @save="saveRoles"
    />
  </section>
</template>

<style scoped src="./UserManagement.css"></style>
