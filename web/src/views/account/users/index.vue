<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessageBox, ElNotification } from "element-plus";
import { useI18n } from "vue-i18n";

import {
  deleteUser,
  getUserRoleOptions,
  getUserRoles,
  getUsers,
  updateUser,
  updateUserRoles,
  updateUserStatus,
} from "../../../api/user";
import type {
  UserListItem,
  UserListQuery,
  UserRolesResponse,
  UserRoleSummary,
} from "../../../api/user";
import { YesNo } from "../../../enums/yes-no";
import { useAccessStore } from "../../../store/access";
import { useAuthStore } from "../../../store/auth";
import { AppDialog } from "../../../components/AppDialog";
import { AppTable } from "../../../components/AppTable";
import type {
  TableColumn,
  TablePaginationState,
} from "../../../components/AppTable";
import { Search } from "../../../components/Search";
import type { SearchField, SearchFormModel } from "../../../components/Search";

const { t } = useI18n();
const access = useAccessStore();
const auth = useAuthStore();

const rows = ref<UserListItem[]>([]);
const roleOptions = ref<UserRoleSummary[]>([]);
const total = ref(0);
const query = ref<UserListQuery>({ page: 1, pageSize: 20 });
const keyword = ref("");
const statusFilter = ref<"" | YesNo>("");
const roleFilter = ref<"" | number>("");
const loading = ref(false);
const roleOptionsLoading = ref(false);
const loadError = ref("");
const roleOptionsError = ref("");
const mutationError = ref("");
const mutating = ref(false);

interface UserFormState {
  username: string;
  phone: string;
}
const editVisible = ref(false);
const editingUser = ref<UserListItem | null>(null);
const userForm = ref<UserFormState>({ username: "", phone: "" });
const editSaving = ref(false);
const editError = ref("");

const roleDialogVisible = ref(false);
const roleTarget = ref<UserListItem | null>(null);
const roleData = ref<UserRolesResponse | null>(null);
const selectedRoleIDs = ref<number[]>([]);
const roleLoading = ref(false);
const roleSaving = ref(false);
const roleError = ref("");

const tablePagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}));
const tableColumns = computed<TableColumn<UserListItem>[]>(() => [
  { prop: "id", label: t("user.id"), width: 80 },
  { prop: "username", label: t("user.username"), minWidth: 140 },
  { prop: "email", label: t("user.email"), minWidth: 210 },
  { prop: "phone", label: t("user.phone"), minWidth: 170 },
  { key: "roles", prop: "id", label: t("user.roles"), minWidth: 240 },
  { key: "status", prop: "id", label: t("user.status"), width: 100 },
  { prop: "createdAt", label: t("user.createdAt"), minWidth: 190 },
  { prop: "updatedAt", label: t("user.updatedAt"), minWidth: 190 },
  { key: "actions", prop: "id", label: t("user.actions"), width: 330 },
]);

const canUpdate = computed(() => access.hasPermission("account:user:update"));
const canStatus = computed(() => access.hasPermission("account:user:status"));
const canDelete = computed(() => access.hasPermission("account:user:delete"));
const canRoles = computed(() => access.hasPermission("account:user:roles"));
const isSuperAdminActor = computed(() =>
  access.roleCodes.includes("super_admin"),
);
const normalizedUsername = computed(() => userForm.value.username.trim());
const normalizedPhone = computed(() => userForm.value.phone.trim());
const usernameValid = computed(() => {
  const characters = [...normalizedUsername.value];
  return (
    characters.length >= 3 &&
    characters.length <= 64 &&
    characters.every((character) => /[\p{L}\p{N}_-]/u.test(character))
  );
});
const phoneValid = computed(() => {
  const value = normalizedPhone.value;
  return value === "" || ([...value].length <= 32 && !/\p{Cc}/u.test(value));
});
const submittedPhone = computed<string | null>(() =>
  normalizedPhone.value === "" ? null : normalizedPhone.value,
);
const hasEnabledSelection = computed(() => {
  if (roleData.value === null) return false;
  const selected = new Set(selectedRoleIDs.value);
  return roleData.value.roles.some(
    (role) => role.isEnabled === YesNo.Yes && selected.has(role.id),
  );
});

const searchModel = computed<SearchFormModel>({
  get: () => ({
    keyword: keyword.value,
    status: statusFilter.value,
    role: roleFilter.value,
  }),
  set: (value) => {
    keyword.value = typeof value.keyword === "string" ? value.keyword : "";
    statusFilter.value =
      value.status === YesNo.Yes || value.status === YesNo.No
        ? value.status
        : "";
    roleFilter.value = typeof value.role === "number" ? value.role : "";
  },
});
const searchFields = computed<SearchField[]>(() => [
  {
    key: "keyword",
    type: "input",
    label: t("user.keyword"),
    placeholder: t("user.keyword"),
    width: 280,
    testId: "user-keyword",
  },
  {
    key: "status",
    type: "select-v2",
    label: t("user.status"),
    options: [
      { label: t("user.status"), value: "" },
      { label: t("user.enabled"), value: YesNo.Yes },
      { label: t("user.disabled"), value: YesNo.No },
    ],
    width: 190,
  },
  {
    key: "role",
    type: "select-v2",
    label: t("user.role"),
    options: [
      { label: t("user.role"), value: "" },
      ...roleOptions.value.map((role) => ({
        label: `${role.name} (${role.code})${role.isEnabled === YesNo.No ? ` · ${t("user.roleDisabled")}` : ""}`,
        value: role.id,
      })),
    ],
    width: 220,
  },
]);

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message !== ""
    ? error.message
    : t(fallback);
}
async function loadUsers(): Promise<boolean> {
  if (loading.value) return false;
  loading.value = true;
  loadError.value = "";
  try {
    const result = await getUsers(query.value);
    rows.value = result.list;
    total.value = result.total;
    return true;
  } catch (error: unknown) {
    loadError.value = errorMessage(error, "user.loadFailed");
    return false;
  } finally {
    loading.value = false;
  }
}
async function loadRoleOptions(): Promise<void> {
  if (roleOptionsLoading.value) return;
  roleOptionsLoading.value = true;
  roleOptionsError.value = "";
  try {
    roleOptions.value = (await getUserRoleOptions()).roles;
  } catch (error: unknown) {
    roleOptionsError.value = errorMessage(error, "user.roleLoadFailed");
  } finally {
    roleOptionsLoading.value = false;
  }
}
function search(): void {
  const value = keyword.value.trim();
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(value === "" ? {} : { keyword: value }),
    ...(statusFilter.value === "" ? {} : { isEnabled: statusFilter.value }),
    ...(roleFilter.value === "" ? {} : { roleId: roleFilter.value }),
  };
  void loadUsers();
}
function reset(): void {
  keyword.value = "";
  statusFilter.value = "";
  roleFilter.value = "";
  query.value = { page: 1, pageSize: query.value.pageSize };
  void loadUsers();
}
function changePage(page: number): void {
  query.value = { ...query.value, page };
  void loadUsers();
}
function changePageSize(pageSize: number): void {
  query.value = { ...query.value, page: 1, pageSize };
  void loadUsers();
}
function updateTablePagination(next: TablePaginationState): void {
  if (next.pageSize !== query.value.pageSize) {
    changePageSize(next.pageSize);
    return;
  }
  changePage(next.currentPage);
}
function isSelf(row: UserListItem): boolean {
  return auth.user?.userId === row.id;
}
function hasSuperAdmin(row: UserListItem): boolean {
  return row.roles.some((role) => role.code === "super_admin");
}
function targetProtected(row: UserListItem): boolean {
  return hasSuperAdmin(row) && !isSuperAdminActor.value;
}
function editDisabled(row: UserListItem): boolean {
  return targetProtected(row);
}
function dangerDisabled(row: UserListItem): boolean {
  return isSelf(row) || targetProtected(row);
}
function protectionText(
  row: UserListItem,
  operation: "status" | "roles" | "delete",
): string {
  if (targetProtected(row)) return t("user.superAdminBlocked");
  if (isSelf(row))
    return t(
      operation === "status"
        ? "user.selfStatusBlocked"
        : operation === "roles"
          ? "user.selfRolesBlocked"
          : "user.selfDeleteBlocked",
    );
  return operation === "roles"
    ? t("user.assignRoles")
    : operation === "delete"
      ? t("permission.userDelete")
      : t("user.status");
}

function openEdit(row: UserListItem): void {
  if (editDisabled(row)) return;
  editingUser.value = row;
  userForm.value = { username: row.username, phone: row.phone ?? "" };
  editError.value = "";
  editVisible.value = true;
}
async function saveEdit(): Promise<void> {
  const target = editingUser.value;
  if (
    target === null ||
    !usernameValid.value ||
    !phoneValid.value ||
    editSaving.value
  )
    return;
  editSaving.value = true;
  editError.value = "";
  try {
    const result = await updateUser(target.id, {
      username: normalizedUsername.value,
      phone: submittedPhone.value,
    });
    auth.updateProfile(result.id, result.username, result.phone);
    if (await loadUsers()) {
      editVisible.value = false;
      ElNotification.success({ title: t("user.updateSuccess") });
    }
  } catch (error: unknown) {
    editError.value = errorMessage(error, "user.saveFailed");
  } finally {
    editSaving.value = false;
  }
}

async function openRoles(row: UserListItem): Promise<void> {
  if (dangerDisabled(row)) return;
  roleTarget.value = row;
  roleDialogVisible.value = true;
  roleLoading.value = true;
  roleError.value = "";
  roleData.value = null;
  selectedRoleIDs.value = [];
  try {
    const data = await getUserRoles(row.id);
    roleData.value = data;
    selectedRoleIDs.value = [...data.roleIds];
  } catch (error: unknown) {
    roleError.value = errorMessage(error, "user.roleLoadFailed");
  } finally {
    roleLoading.value = false;
  }
}
function protectedSuperRoleID(): number | null {
  if (isSuperAdminActor.value || roleData.value === null) return null;
  const role = roleData.value.roles.find((item) => item.code === "super_admin");
  return role === undefined ? null : role.id;
}
function protectedSelectedRoleIDs(): number[] {
  const protectedID = protectedSuperRoleID();
  return protectedID !== null && roleData.value?.roleIds.includes(protectedID)
    ? [protectedID]
    : [];
}
function selectAllRoles(): void {
  if (roleData.value === null) return;
  const protectedID = protectedSuperRoleID();
  selectedRoleIDs.value = [
    ...roleData.value.roles
      .filter((role) => role.id !== protectedID)
      .map((role) => role.id),
    ...protectedSelectedRoleIDs(),
  ].sort((a, b) => a - b);
}
function clearRoles(): void {
  selectedRoleIDs.value = protectedSelectedRoleIDs();
}
function roleToggleDisabled(role: UserRoleSummary): boolean {
  return role.code === "super_admin" && !isSuperAdminActor.value;
}
async function saveRoles(): Promise<void> {
  const target = roleTarget.value;
  if (
    target === null ||
    roleData.value === null ||
    !hasEnabledSelection.value ||
    roleSaving.value
  )
    return;
  roleSaving.value = true;
  roleError.value = "";
  try {
    const roleIds = [...new Set(selectedRoleIDs.value)].sort((a, b) => a - b);
    await updateUserRoles(target.id, { roleIds });
    if (await loadUsers()) {
      roleDialogVisible.value = false;
      ElNotification.success({ title: t("user.rolesSuccess") });
    }
  } catch (error: unknown) {
    roleError.value = errorMessage(error, "user.saveFailed");
  } finally {
    roleSaving.value = false;
  }
}

async function changeStatus(row: UserListItem): Promise<void> {
  if (dangerDisabled(row) || mutating.value) return;
  const next = row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes;
  let message = t(
    next === YesNo.No ? "user.disableConfirm" : "user.enableConfirm",
  );
  if (hasSuperAdmin(row)) message += ` ${t("user.superAdminImpact")}`;
  try {
    await ElMessageBox.confirm(message, t("user.status"), { type: "warning" });
    mutating.value = true;
    await updateUserStatus(row.id, next);
    await loadUsers();
    ElNotification.success({ title: t("user.statusSuccess") });
  } catch (error: unknown) {
    if (error !== "cancel" && error !== "close")
      mutationError.value = errorMessage(error, "user.saveFailed");
  } finally {
    mutating.value = false;
  }
}
async function removeUser(row: UserListItem): Promise<void> {
  if (dangerDisabled(row) || mutating.value) return;
  let message = t("user.deleteConfirm");
  if (hasSuperAdmin(row)) message += ` ${t("user.superAdminImpact")}`;
  try {
    await ElMessageBox.confirm(message, t("permission.userDelete"), {
      type: "warning",
    });
    mutating.value = true;
    await deleteUser(row.id);
    const maxPage = Math.max(
      1,
      Math.ceil((total.value - 1) / query.value.pageSize),
    );
    if (query.value.page > maxPage)
      query.value = { ...query.value, page: maxPage };
    await loadUsers();
    ElNotification.success({ title: t("user.deleteSuccess") });
  } catch (error: unknown) {
    if (error !== "cancel" && error !== "close")
      mutationError.value = errorMessage(error, "user.saveFailed");
  } finally {
    mutating.value = false;
  }
}

onMounted(() => {
  void loadRoleOptions();
  void loadUsers();
});
</script>

<template>
  <section class="user-management management-page">
    <Search
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
    <el-alert
      v-if="roleOptionsError"
      :title="roleOptionsError"
      type="error"
      show-icon
    /><el-alert
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
      result-state="success"
      :aria-label="t('user.title')"
      :refresh-label="t('user.refresh')"
      @refresh="loadUsers"
      @update:pagination="updateTablePagination"
    >
      <template #cell-roles="{ row }: { row: UserListItem }">
        <div v-if="row.id > 0" class="role-tags">
          <el-tooltip
            v-for="role in row.roles"
            :key="role.id"
            :content="role.code"
            ><el-tag :type="role.isEnabled === YesNo.Yes ? 'primary' : 'info'"
              >{{ role.name
              }}<span v-if="role.isEnabled === YesNo.No">
                · {{ t("user.roleDisabled") }}</span
              ></el-tag
            ></el-tooltip
          >
        </div>
      </template>
      <template #cell-status="{ row }: { row: UserListItem }"
        ><el-tag
          v-if="row.id > 0"
          :type="row.isEnabled === YesNo.Yes ? 'success' : 'danger'"
          >{{
            t(row.isEnabled === YesNo.Yes ? "user.enabled" : "user.disabled")
          }}</el-tag
        ></template
      >
      <template #cell-phone="{ row }: { row: UserListItem }">
        {{ row.phone ?? "-" }}
      </template>
      <template #cell-actions="{ row }: { row: UserListItem }"
        ><template v-if="row.id > 0">
          <el-space wrap :size="6">
          <el-tooltip
            v-if="canUpdate"
            :content="
              editDisabled(row) ? t('user.superAdminBlocked') : t('user.edit')
            "
            ><el-button
              text
              type="primary"
              :disabled="editDisabled(row)"
              @click="openEdit(row)"
              >{{ t("user.edit") }}</el-button
            ></el-tooltip
          >
          <el-tooltip v-if="canStatus" :content="protectionText(row, 'status')"
            ><el-button
              text
              type="warning"
              :disabled="dangerDisabled(row) || mutating"
              @click="changeStatus(row)"
              >{{
                row.isEnabled === YesNo.Yes
                  ? t("user.disabled")
                  : t("user.enabled")
              }}</el-button
            ></el-tooltip
          >
          <el-tooltip v-if="canRoles" :content="protectionText(row, 'roles')"
            ><el-button
              text
              type="primary"
              :disabled="dangerDisabled(row)"
              @click="openRoles(row)"
              >{{ t("user.assignRoles") }}</el-button
            ></el-tooltip
          >
          <el-tooltip v-if="canDelete" :content="protectionText(row, 'delete')"
            ><el-button
              text
              type="danger"
              :disabled="dangerDisabled(row) || mutating"
              @click="removeUser(row)"
              >{{ t("permission.userDelete") }}</el-button
            ></el-tooltip
          >
          </el-space>
        </template></template
      >
      <template #empty><el-empty :description="t('user.noRoles')" /></template>
    </AppTable>

    <AppDialog
      v-model="editVisible"
      class="user-edit-dialog"
      :title="t('user.editTitle')"
      width="min(520px, 94vw)"
      append-to-body
    >
      <el-alert v-if="editError" :title="editError" type="error" /><el-form
        label-position="top"
        ><el-form-item :label="t('user.email')"
          ><el-input
            :model-value="editingUser?.email ?? ''"
            disabled /></el-form-item
        ><el-form-item
          :label="t('user.username')"
          :error="
            userForm.username !== '' && !usernameValid
              ? t('user.invalidUsername')
              : ''
          "
          ><el-input v-model="userForm.username" maxlength="64" /></el-form-item
        ><el-form-item
          :label="t('user.phone')"
          :error="
            userForm.phone !== '' && !phoneValid
              ? t('user.invalidPhone')
              : ''
          "
          ><el-input
            v-model="userForm.phone"
            data-testid="user-phone" /></el-form-item
      ></el-form>
      <template #footer
        ><el-button @click="editVisible = false">{{
          t("user.cancel")
        }}</el-button
        ><el-button
          type="primary"
          :loading="editSaving"
          :disabled="!usernameValid || !phoneValid"
          @click="saveEdit"
          >{{ t("user.save") }}</el-button
        ></template
      >
    </AppDialog>
    <AppDialog
      v-model="roleDialogVisible"
      class="user-role-dialog"
      :title="t('user.assignRolesTitle')"
      width="min(680px, 94vw)"
      height="min(62vh, 620px)"
      append-to-body
    >
      <div class="role-dialog-scroll">
        <div v-if="roleLoading">{{ t("user.roleLoadFailed") }}</div>
        <el-alert
          v-if="roleError"
          :title="roleError"
          type="error"
          show-icon
        /><template v-if="roleData">
          <el-space class="role-dialog-toolbar" wrap :size="8">
            <el-button @click="selectAllRoles">{{
              t("user.selectAll")
            }}</el-button
            ><el-button @click="clearRoles">{{ t("user.clear") }}</el-button>
          </el-space>
          <el-checkbox-group v-model="selectedRoleIDs" class="role-checks"
            ><el-checkbox
              v-for="role in roleData.roles"
              :key="role.id"
              :value="role.id"
              :disabled="roleToggleDisabled(role)"
              ><span>{{ role.name }} ({{ role.code }})</span
              ><el-tag
                v-if="role.isEnabled === YesNo.No"
                type="info"
                size="small"
                >{{ t("user.roleDisabled") }}</el-tag
              ></el-checkbox
            ></el-checkbox-group
          ><el-alert
            v-if="!hasEnabledSelection"
            :title="t('user.enabledRoleRequired')"
            type="warning"
          />
        </template>
      </div>
      <template #footer
        ><el-button @click="roleDialogVisible = false">{{
          t("user.cancel")
        }}</el-button
        ><el-button
          type="primary"
          :loading="roleSaving"
          :disabled="roleData === null || !hasEnabledSelection"
          @click="saveRoles"
          >{{ t("user.save") }}</el-button
        ></template
      >
    </AppDialog>
  </section>
</template>

<style scoped lang="scss">
.user-management {
  min-height: 0;
  min-width: 0;
}

.user-filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.user-table {
  width: 100%;
}

.role-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.el-pagination {
  justify-content: flex-end;
}

.role-dialog-scroll {
  max-height: min(62vh, 620px);
  padding-right: 8px;
  overflow-y: auto;
}

.role-dialog-toolbar {
  justify-content: flex-end;
  margin-bottom: 12px;
}

.role-checks {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 12px;
}

.role-checks .el-checkbox {
  height: auto;
  margin-right: 0;
}

.role-checks span {
  margin-right: 8px;
}

@media (max-width: 720px) {
  .el-pagination {
    justify-content: flex-start;
    overflow-x: auto;
  }
}
</style>
