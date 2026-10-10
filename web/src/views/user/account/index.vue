<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { useI18n } from 'vue-i18n'

import {
  deleteUser,
  getUserRoleOptions,
  getUserFormOptions,
  getUserRoles,
  getUsers,
  updateUser,
  updateUserRoles,
  updateUserStatus,
} from '@/api/user/account'
import type {
  UserListItem,
  UserFormOptions,
  UserListQuery,
  UserRolesResponse,
  UserRoleSummary,
} from '@/api/user/account'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'
import { useAuthStore } from '@/store/auth'
import type { TablePaginationState } from '@/components/AppTable'
import type { SearchFormModel } from '@/components/AppSearch'
import { formatTime } from '@/utils/datetime'
import UserEditDialog from './components/UserEditDialog/index.vue'
import UserRoleDialog from './components/UserRoleDialog/index.vue'
import type { UserFormState } from './components/types'
import IdentityChangeHistoryAction from './components/IdentityChangeHistoryAction/index.vue'
import {
  normalizedUsername,
  userSearchFields,
  userTableColumns,
  type UserSearchModel,
} from './userRules'

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
const userForm = ref<UserFormState>({ username: '' })
const editSaving = ref(false)
const editError = ref('')

const roleDialogVisible = ref(false)
const roleTarget = ref<UserListItem | null>(null)
const roleData = ref<UserRolesResponse | null>(null)
const selectedRoleIDs = ref<number[]>([])
const roleLoading = ref(false)
const roleSaving = ref(false)
const roleError = ref('')
let roleDialogSequence = 0
let active = true

const tablePagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const tableColumns = computed(() => userTableColumns(t))

const canUpdate = computed(() => access.hasPermission('user:account:update'))
const canStatus = computed(() => access.hasPermission('user:account:status'))
const canDelete = computed(() => access.hasPermission('user:account:delete'))
const canRoles = computed(() => access.hasPermission('user:account:authorize'))
const canIdentityDetail = computed(() => access.hasPermission('user:account:detail'))
const normalizedUsernameValue = computed(() => normalizedUsername(userForm.value.username))
const formOptionsError = ref('')
const formOptions = ref<UserFormOptions | null>(null)
const usernameValid = computed(() => {
  const options = formOptions.value
  if (options === null) return false
  const value = normalizedUsernameValue.value
  const length = [...value].length
  return (
    length >= options.usernameMinLength &&
    length <= options.usernameMaxLength &&
    new RegExp(options.usernamePattern, 'u').test(value)
  )
})
async function loadFormOptions(): Promise<void> {
  formOptionsError.value = ''
  try {
    formOptions.value = await getUserFormOptions()
  } catch (error: unknown) {
    formOptionsError.value = errorMessage(error, 'user.loadFailed')
  }
}

const searchModel = computed<SearchFormModel<UserSearchModel>>({
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
function openEdit(row: UserListItem): void {
  if (!canUpdate.value || !row.actions.update) return
  editingUser.value = row
  userForm.value = { username: row.username }
  editError.value = ''
  editVisible.value = true
}
async function saveEdit(): Promise<void> {
  const target = editingUser.value
  if (
    target === null ||
    !canUpdate.value ||
    !target.actions.update ||
    !usernameValid.value ||
    editSaving.value
  )
    return
  editSaving.value = true
  editError.value = ''
  try {
    const result = await updateUser(target.id, {
      username: normalizedUsernameValue.value,
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

function closeRoleDialog(): void {
  roleDialogSequence += 1
  roleDialogVisible.value = false
  roleTarget.value = null
  roleData.value = null
  selectedRoleIDs.value = []
  roleError.value = ''
  roleLoading.value = false
}
function updateRoleDialogVisible(visible: boolean): void {
  if (!visible) closeRoleDialog()
}
function isCurrentRoleDialog(sequence: number, targetID: number): boolean {
  return (
    active &&
    sequence === roleDialogSequence &&
    roleDialogVisible.value &&
    roleTarget.value?.id === targetID &&
    canRoles.value &&
    roleTarget.value.actions.authorize
  )
}
function canEditRoleSelection(): boolean {
  const target = roleTarget.value
  return (
    target !== null &&
    isCurrentRoleDialog(roleDialogSequence, target.id) &&
    roleData.value?.user.id === target.id &&
    !roleLoading.value &&
    !roleSaving.value
  )
}
async function openRoles(row: UserListItem): Promise<void> {
  if (!active || !canRoles.value || !row.actions.authorize) return
  const sequence = ++roleDialogSequence
  const targetID = row.id
  roleTarget.value = row
  roleDialogVisible.value = true
  roleLoading.value = true
  roleError.value = ''
  roleData.value = null
  selectedRoleIDs.value = []
  try {
    const data = await getUserRoles(targetID)
    if (!isCurrentRoleDialog(sequence, targetID)) return
    if (data.user.id !== targetID) {
      roleError.value = t('user.roleLoadFailed')
      return
    }
    roleData.value = data
    selectedRoleIDs.value = [...data.roleIds]
  } catch (error: unknown) {
    if (isCurrentRoleDialog(sequence, targetID))
      roleError.value = errorMessage(error, 'user.roleLoadFailed')
  } finally {
    if (isCurrentRoleDialog(sequence, targetID)) roleLoading.value = false
  }
}

function updateSelectedRoleIDs(ids: number[]): void {
  if (canEditRoleSelection()) selectedRoleIDs.value = [...ids]
}
function lockedSelectedRoleIDs(): number[] {
  const data = roleData.value
  if (data === null) return []
  return data.roles
    .filter((role) => role.locked && data.roleIds.includes(role.id))
    .map((role) => role.id)
}
function selectAllRoles(): void {
  if (!canEditRoleSelection() || roleData.value === null) return
  selectedRoleIDs.value = [
    ...new Set([
      ...roleData.value.roles.filter((role) => role.selectable).map((role) => role.id),
      ...lockedSelectedRoleIDs(),
    ]),
  ].sort((a, b) => a - b)
}
function clearRoles(): void {
  if (!canEditRoleSelection()) return
  selectedRoleIDs.value = lockedSelectedRoleIDs()
}
async function saveRoles(): Promise<void> {
  const target = roleTarget.value
  if (target === null || !canEditRoleSelection()) return
  const targetID = target.id
  const sequence = roleDialogSequence
  const roleIds = [...new Set(selectedRoleIDs.value)].sort((a, b) => a - b)
  roleSaving.value = true
  roleError.value = ''
  try {
    await updateUserRoles(targetID, { roleIds })
    if (!active) return
    if ((await loadUsers()) && isCurrentRoleDialog(sequence, targetID)) {
      closeRoleDialog()
      ElNotification.success({ title: t('user.rolesSuccess') })
    }
  } catch (error: unknown) {
    if (isCurrentRoleDialog(sequence, targetID))
      roleError.value = errorMessage(error, 'user.saveFailed')
  } finally {
    roleSaving.value = false
  }
}

watch(
  () => [canRoles.value, roleTarget.value?.actions.authorize],
  ([authorized, targetAuthorized]) => {
    if (roleDialogVisible.value && (!authorized || !targetAuthorized)) closeRoleDialog()
  },
  { flush: 'sync' },
)
onBeforeUnmount(() => {
  active = false
  closeRoleDialog()
})

async function changeStatus(row: UserListItem): Promise<void> {
  if (!canStatus.value || !row.actions.status || mutating.value) return
  const next = row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes
  const message = t(next === YesNo.No ? 'user.disableConfirm' : 'user.enableConfirm')
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
  if (!canDelete.value || !row.actions.delete || mutating.value) return
  const message = t('user.deleteConfirm')
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
  void loadFormOptions()
  void loadRoleOptions()
  void loadUsers()
})
</script>

<template>
  <AppPage class="user-management">
    <el-alert v-if="formOptionsError" :title="formOptionsError" type="error" show-icon>
      <el-button text @click="loadFormOptions">{{ t('user.reset') }}</el-button>
    </el-alert>
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
      <template #cell-createdAt="{ row }: { row: UserListItem }">{{
        row.id > 0 ? formatTime(row.createdAt) : ''
      }}</template>
      <template #cell-updatedAt="{ row }: { row: UserListItem }">{{
        row.id > 0 ? formatTime(row.updatedAt) : ''
      }}</template>
      <template #cell-actions="{ row }: { row: UserListItem }"
        ><template v-if="row.id > 0">
          <el-space wrap :size="6">
            <el-tooltip v-if="canUpdate" :content="row.actionLabels.update"
              ><el-button
                text
                type="primary"
                :disabled="!row.actions.update"
                @click="openEdit(row)"
                >{{ t('user.edit') }}</el-button
              ></el-tooltip
            >
            <el-tooltip v-if="canStatus" :content="row.actionLabels.status"
              ><el-button
                text
                type="warning"
                :disabled="!row.actions.status || mutating"
                @click="changeStatus(row)"
                >{{
                  row.isEnabled === YesNo.Yes ? t('user.disabled') : t('user.enabled')
                }}</el-button
              ></el-tooltip
            >
            <el-tooltip v-if="canRoles" :content="row.actionLabels.authorize"
              ><el-button
                text
                type="primary"
                :disabled="!row.actions.authorize"
                @click="openRoles(row)"
                >{{ t('user.assignRoles') }}</el-button
              ></el-tooltip
            >
            <IdentityChangeHistoryAction :user="row" :enabled="canIdentityDetail" />
            <el-tooltip v-if="canDelete" :content="row.actionLabels.delete"
              ><el-button
                text
                type="danger"
                :disabled="!row.actions.delete || mutating"
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
      :username-max-length="formOptions?.usernameMaxLength"
      @save="saveEdit"
    />
    <UserRoleDialog
      :model-value="roleDialogVisible"
      :selected-role-i-ds="selectedRoleIDs"
      :role-data="roleData"
      :role-loading="roleLoading"
      :role-error="roleError"
      :role-saving="roleSaving"
      @update:model-value="updateRoleDialogVisible"
      @update:selected-role-i-ds="updateSelectedRoleIDs"
      @select-all="selectAllRoles"
      @clear="clearRoles"
      @save="saveRoles"
    />
  </AppPage>
</template>

<style scoped src="./UserManagement.scss" lang="scss"></style>
