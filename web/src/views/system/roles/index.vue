<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import {
  CirclePlus,
  Delete,
  Edit,
  Key,
  Refresh,
  Star,
  SwitchButton,
} from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import {
  createRole,
  deleteRole,
  getRolePermissions,
  getRoles,
  setDefaultRole,
  updateRole,
  updateRolePermissions,
  updateRoleStatus,
} from '../../../api/role'
import type {
  RoleListItem,
  RoleListQuery,
  RolePermissionsResponse,
} from '../../../api/role.contract'
import { YesNo } from '../../../enums/yes-no'
import { useAccessStore } from '../../../store/access'
import RolePermissionDiffDialog from './components/RolePermissionDiffDialog.vue'
import RolePermissionMatrix from './components/RolePermissionMatrix.vue'
import {
  buildRolePermissionMatrix,
  diffMenuIDs,
  expandDirectMenuIDs,
  getRoleMatrixMenuIDs,
  normalizeDirectMenuIDs,
} from './role-permission-matrix'
import type { RolePermissionDiff } from './role-permission-matrix'

const { t } = useI18n()
const access = useAccessStore()

const rows = ref<RoleListItem[]>([])
const total = ref(0)
const query = ref<RoleListQuery>({ page: 1, pageSize: 20 })
const keyword = ref('')
const statusFilter = ref<'' | YesNo>('')
const loading = ref(false)
const loadError = ref('')
const mutationError = ref('')

const submitting = ref(false)
const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingRole = ref<RoleListItem | null>(null)

interface RoleFormState {
  code: string
  name: string
}

const form = ref<RoleFormState>({ code: '', name: '' })

const permissionDialogVisible = ref(false)
const permissionLoading = ref(false)
const permissionSaving = ref(false)
const permissionError = ref('')
const permissionData = ref<RolePermissionsResponse | null>(null)
const permissionTargetID = ref<number | null>(null)
const originalEffectiveMenuIDs = ref<number[]>([])
const selectedEffectiveMenuIDs = ref<number[]>([])
const permissionDiffVisible = ref(false)
const permissionDiff = ref<RolePermissionDiff>({ added: [], removed: [] })

const permissionGroups = computed(() => {
  if (permissionData.value === null) {
    return []
  }
  return buildRolePermissionMatrix(permissionData.value.menuTree)
})
const permissionLabelMap = computed(() => {
  const labels = new Map<number, string>()
  for (const group of permissionGroups.value) {
    for (const row of group.rows) {
      labels.set(row.pageId, `${t(row.pageI18nKey)} · ${row.pageCode}`)
      for (const action of row.actions) {
        labels.set(action.id, `${t(action.i18nKey)} · ${action.code}`)
      }
    }
  }
  return labels
})
const addedPermissionLabels = computed(() => permissionLabels(permissionDiff.value.added))
const removedPermissionLabels = computed(() => permissionLabels(permissionDiff.value.removed))

const canCreate = computed(() => access.hasPermission('system:role:create'))
const canUpdate = computed(() => access.hasPermission('system:role:update'))
const canStatus = computed(() => access.hasPermission('system:role:status'))
const canDefault = computed(() => access.hasPermission('system:role:default'))
const canDelete = computed(() => access.hasPermission('system:role:delete'))
const canAuthorize = computed(() => access.hasPermission('system:role:authorize'))

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
      const successKey = dialogMode.value === 'create' ? 'role.success.created' : 'role.success.updated'
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
    await ElMessageBox.confirm(message, t('role.action.default'), { type: 'warning' })
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

async function openPermissions(role: { id: number }): Promise<void> {
  permissionTargetID.value = role.id
  permissionDialogVisible.value = true
  permissionDiffVisible.value = false
  permissionLoading.value = true
  permissionError.value = ''
  permissionData.value = null
  permissionDiff.value = { added: [], removed: [] }
  originalEffectiveMenuIDs.value = []
  selectedEffectiveMenuIDs.value = []

  try {
    const data = await getRolePermissions(role.id)
    const groups = buildRolePermissionMatrix(data.menuTree)
    const effectiveMenuIDs = expandDirectMenuIDs(groups, data.menuIds)
    permissionData.value = data
    originalEffectiveMenuIDs.value = effectiveMenuIDs
    selectedEffectiveMenuIDs.value = [...effectiveMenuIDs]
  } catch (error: unknown) {
    permissionError.value = errorMessage(error)
  } finally {
    permissionLoading.value = false
  }
}

function retryPermissions(): void {
  if (permissionTargetID.value !== null) {
    void openPermissions({ id: permissionTargetID.value })
  }
}

function selectAllPermissions(): void {
  selectedEffectiveMenuIDs.value = getRoleMatrixMenuIDs(permissionGroups.value)
}

function clearPermissions(): void {
  selectedEffectiveMenuIDs.value = []
}

function permissionLabels(menuIDs: readonly number[]): string[] {
  return menuIDs.map((menuID) => {
    const label = permissionLabelMap.value.get(menuID)
    if (label === undefined) {
      throw new Error(`permission menu ${menuID} has no display label`)
    }
    return label
  })
}

function preparePermissionSave(): void {
  if (permissionData.value === null || permissionSaving.value) {
    return
  }

  permissionError.value = ''
  const nextDiff = diffMenuIDs(
    originalEffectiveMenuIDs.value,
    selectedEffectiveMenuIDs.value,
  )
  if (nextDiff.added.length === 0 && nextDiff.removed.length === 0) {
    permissionDialogVisible.value = false
    return
  }

  permissionDiff.value = nextDiff
  permissionDiffVisible.value = true
}

async function savePermissions(): Promise<void> {
  if (permissionData.value === null || permissionSaving.value) {
    return
  }

  permissionSaving.value = true
  permissionError.value = ''

  try {
    await updateRolePermissions(permissionData.value.role.id, {
      menuIds: normalizeDirectMenuIDs(
        permissionGroups.value,
        selectedEffectiveMenuIDs.value,
      ),
    })
    if (await loadRoles()) {
      permissionDiffVisible.value = false
      permissionDialogVisible.value = false
      ElNotification.success({ title: t('role.success.authorized') })
    }
  } catch (error: unknown) {
    permissionError.value = errorMessage(error)
  } finally {
    permissionSaving.value = false
  }
}

function isSystem(role: RoleListItem): boolean {
  return role.code === 'super_admin' || role.code === 'registered_user'
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
  <section class="role-page">
    <header class="role-toolbar">
      <h1>{{ t('role.title') }}</h1>
      <div>
        <el-button :icon="Refresh" @click="loadRoles">
          {{ t('role.refresh') }}
        </el-button>
        <el-button v-if="canCreate" type="primary" :icon="CirclePlus" @click="openCreate">
          {{ t('role.create') }}
        </el-button>
      </div>
    </header>

    <div class="role-filters">
      <el-input
        v-model="keyword"
        clearable
        :placeholder="t('role.keyword')"
        @keyup.enter="search"
      />
      <el-select v-model="statusFilter">
        <el-option :label="t('role.status.all')" value="" />
        <el-option :label="t('role.status.enabled')" :value="YesNo.Yes" />
        <el-option :label="t('role.status.disabled')" :value="YesNo.No" />
      </el-select>
      <el-button type="primary" @click="search">
        {{ t('role.search') }}
      </el-button>
      <el-button @click="reset">
        {{ t('role.reset') }}
      </el-button>
    </div>

    <el-alert v-if="loadError" :title="loadError" type="error" show-icon>
      <el-button link @click="loadRoles">
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

    <el-table
      v-loading="loading"
      :data="rows"
      row-key="id"
      class="role-table"
      empty-text=""
    >
      <el-table-column prop="name" :label="t('role.column.name')" min-width="150" />
      <el-table-column prop="code" :label="t('role.column.code')" min-width="170" />
      <el-table-column :label="t('role.column.default')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.isDefault === YesNo.Yes ? 'success' : 'info'">
            {{ t(row.isDefault === YesNo.Yes ? 'role.default.yes' : 'role.default.no') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('role.column.status')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'danger'">
            {{ t(row.isEnabled === YesNo.Yes ? 'role.status.enabled' : 'role.status.disabled') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="userCount" :label="t('role.column.users')" width="100" />
      <el-table-column
        prop="permissionCount"
        :label="t('role.column.permissions')"
        width="130"
      />
      <el-table-column prop="createdAt" :label="t('role.column.createdAt')" min-width="190" />
      <el-table-column prop="updatedAt" :label="t('role.column.updatedAt')" min-width="190" />
      <el-table-column fixed="right" :label="t('role.column.actions')" width="230">
        <template #default="{ row }">
          <el-tooltip v-if="canUpdate" :content="editTooltip(row)">
            <el-button circle :icon="Edit" :disabled="isSystem(row)" @click="openEdit(row)" />
          </el-tooltip>
          <el-tooltip v-if="canStatus" :content="statusTooltip(row)">
            <el-button
              circle
              :icon="SwitchButton"
              :disabled="row.code === 'super_admin' || row.isDefault === YesNo.Yes"
              @click="changeStatus(row)"
            />
          </el-tooltip>
          <el-tooltip v-if="canDefault" :content="defaultTooltip(row)">
            <el-button
              circle
              :icon="Star"
              :disabled="
                row.code === 'super_admin' ||
                row.isDefault === YesNo.Yes ||
                row.isEnabled === YesNo.No
              "
              @click="makeDefault(row)"
            />
          </el-tooltip>
          <el-tooltip
            v-if="canAuthorize && row.code !== 'super_admin'"
            :content="t('role.action.authorize')"
          >
            <el-button circle :icon="Key" @click="openPermissions(row)" />
          </el-tooltip>
          <el-tooltip v-if="canDelete" :content="deleteTooltip(row)">
            <el-button
              circle
              type="danger"
              plain
              :icon="Delete"
              :disabled="isSystem(row) || row.isDefault === YesNo.Yes || row.userCount > 0"
              @click="removeRole(row)"
            />
          </el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="!loading && rows.length === 0 && !loadError" class="role-empty">
      {{ t('role.empty') }}
    </div>

    <el-pagination
      :current-page="query.page"
      :page-size="query.pageSize"
      :total="total"
      :page-sizes="[20, 50, 100]"
      layout="total, sizes, prev, pager, next"
      @current-change="changePage"
      @size-change="changePageSize"
    />

    <el-dialog
      v-model="dialogVisible"
      :title="t(dialogMode === 'create' ? 'role.form.createTitle' : 'role.form.editTitle')"
      width="520px"
      append-to-body
    >
      <el-alert v-if="mutationError" :title="mutationError" type="error" />
      <el-form label-position="top">
        <el-form-item :label="t('role.form.code')">
          <el-input v-model="form.code" :disabled="dialogMode === 'edit'" />
        </el-form-item>
        <el-form-item :label="t('role.form.name')">
          <el-input v-model="form.name" maxlength="64" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          {{ t('role.form.cancel') }}
        </el-button>
        <el-button
          type="primary"
          :loading="submitting"
          :disabled="!formValid"
          @click="submitForm"
        >
          {{ t('role.form.submit') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="permissionDialogVisible"
      class="role-permission-dialog"
      width="min(1040px, 94vw)"
      append-to-body
    >
      <template #header>
        <strong>
          {{ t('role.permission.title') }}
          <template v-if="permissionData">
            · {{ permissionData.role.name }} ({{ permissionData.role.code }})
          </template>
        </strong>
      </template>

      <div class="permission-scroll">
        <div v-if="permissionLoading">
          {{ t('role.permission.loading') }}
        </div>
        <template v-else-if="permissionData">
          <el-alert
            v-if="permissionError"
            :title="permissionError"
            type="error"
            show-icon
            closable
            @close="permissionError = ''"
          />
          <div class="permission-toolbar">
            <el-button @click="selectAllPermissions">
              {{ t('role.permission.selectAll') }}
            </el-button>
            <el-button @click="clearPermissions">
              {{ t('role.permission.clear') }}
            </el-button>
          </div>
          <RolePermissionMatrix
            v-model="selectedEffectiveMenuIDs"
            :groups="permissionGroups"
          />
        </template>
        <el-alert v-else-if="permissionError" :title="permissionError" type="error" show-icon>
          <el-button link @click="retryPermissions">
            {{ t('role.retry') }}
          </el-button>
        </el-alert>
        <div v-else>
          {{ t('role.permission.empty') }}
        </div>
      </div>

      <template #footer>
        <el-button @click="permissionDialogVisible = false">
          {{ t('role.form.cancel') }}
        </el-button>
        <el-button
          type="primary"
          :disabled="permissionData === null || permissionSaving"
          @click="preparePermissionSave"
        >
          {{ t('role.permission.save') }}
        </el-button>
      </template>
    </el-dialog>

    <RolePermissionDiffDialog
      v-model="permissionDiffVisible"
      :added-labels="addedPermissionLabels"
      :removed-labels="removedPermissionLabels"
      :saving="permissionSaving"
      :error="permissionError"
      @confirm="savePermissions"
    />
  </section>
</template>

<style scoped>
.role-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}

.role-toolbar,
.role-filters {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.role-toolbar h1 {
  margin: 0;
  font-size: 20px;
}

.role-filters {
  justify-content: flex-start;
  flex-wrap: wrap;
}

.role-filters .el-input {
  width: 280px;
}

.role-filters .el-select {
  width: 160px;
}

.role-table {
  width: 100%;
}

.role-empty {
  padding: 28px;
  color: var(--el-text-color-secondary);
  text-align: center;
}

.el-pagination {
  justify-content: flex-end;
}

.permission-scroll {
  max-height: min(62vh, 620px);
  padding-right: 8px;
  overflow-y: auto;
}

.permission-toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 12px;
}

@media (max-width: 720px) {
  .role-toolbar {
    align-items: flex-start;
  }

  .role-filters .el-input,
  .role-filters .el-select {
    width: 100%;
  }

  .el-pagination {
    justify-content: flex-start;
    overflow-x: auto;
  }
}
</style>
