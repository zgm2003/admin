<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { CirclePlus } from "@element-plus/icons-vue";
import { ElMessageBox, ElNotification } from "element-plus";
import { CircleHelp, RotateCcw } from "lucide-vue-next";
import { useI18n } from "vue-i18n";

import {
  createAuthPlatform,
  deleteAuthPlatform,
  getAuthPlatforms,
  updateAuthPlatform,
  updateAuthPlatformStatus,
} from "../../../api/auth-platform";
import type {
  AuthPlatformListItem,
  AuthPlatformListQuery,
  CreateAuthPlatformInput,
  UpdateAuthPlatformInput,
} from "../../../api/auth-platform";
import { YesNo } from "../../../enums/yes-no";
import { useAccessStore } from "../../../store/access";
import { AppDialog } from "../../../components/AppDialog";
import { AppTable } from "../../../components/AppTable";
import type {
  TableColumn,
  TablePaginationState,
} from "../../../components/AppTable";
import { Search } from "../../../components/Search";
import type { SearchField, SearchFormModel } from "../../../components/Search";

const { t, locale } = useI18n();
const access = useAccessStore();

const rows = ref<AuthPlatformListItem[]>([]);
const total = ref(0);
const query = ref<AuthPlatformListQuery>({ page: 1, pageSize: 20 });
const keyword = ref("");
const statusFilter = ref<"" | YesNo>("");
const loading = ref(false);
const loadError = ref("");
const mutationError = ref("");
const searchModel = computed<SearchFormModel>({
  get: () => ({ keyword: keyword.value, status: statusFilter.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === "string" ? value.keyword : "";
    statusFilter.value =
      value.status === YesNo.Yes || value.status === YesNo.No
        ? value.status
        : "";
  },
});
const searchFields = computed<SearchField[]>(() => [
  {
    key: "keyword",
    type: "input",
    label: t("authPlatform.keyword"),
    placeholder: t("authPlatform.keyword"),
    width: 260,
    testId: "auth-platform-keyword",
  },
  {
    key: "status",
    type: "select-v2",
    label: t("authPlatform.status.all"),
    placeholder: t("authPlatform.status.all"),
    options: [
      { label: t("authPlatform.status.all"), value: "" },
      { label: t("authPlatform.status.enabled"), value: YesNo.Yes },
      { label: t("authPlatform.status.disabled"), value: YesNo.No },
    ],
    width: 160,
    testId: "auth-platform-status-filter",
  },
]);

const dialogVisible = ref(false);
const dialogMode = ref<"create" | "edit">("create");
const editingPlatform = ref<AuthPlatformListItem | null>(null);
const submitting = ref(false);

const tablePagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}));
const tableColumns = computed<TableColumn<AuthPlatformListItem>[]>(() => [
  {
    key: "platform",
    prop: "id",
    label: t("authPlatform.column.platform"),
    minWidth: 160,
  },
  {
    key: "tokenTTL",
    prop: "id",
    label: t("authPlatform.column.tokenTTL"),
    minWidth: 175,
  },
  {
    key: "cacheTTL",
    prop: "id",
    label: t("authPlatform.column.cacheTTL"),
    minWidth: 175,
  },
  {
    key: "security",
    prop: "id",
    label: t("authPlatform.column.security"),
    minWidth: 165,
  },
  {
    key: "sessions",
    prop: "id",
    label: t("authPlatform.column.sessions"),
    width: 110,
  },
  {
    key: "registration",
    prop: "id",
    label: t("authPlatform.column.registration"),
    width: 105,
  },
  {
    key: "status",
    prop: "id",
    label: t("authPlatform.column.status"),
    width: 90,
  },
  { prop: "updatedAt", label: t("authPlatform.column.updatedAt"), width: 140 },
  {
    key: "actions",
    prop: "id",
    label: t("authPlatform.column.actions"),
    width: 190,
    fixed: "right",
  },
]);

interface AuthPlatformForm {
  code: string;
  name: string;
  accessTTLSeconds: number;
  refreshTTLSeconds: number;
  sessionCacheTTLSeconds: number;
  accessCacheTTLSeconds: number;
  bindDevice: YesNo;
  bindIP: YesNo;
  maxSessions: number;
  allowRegister: YesNo;
  isEnabled: YesNo;
}

const defaultTTL = Object.freeze({
  accessTTLSeconds: 900,
  refreshTTLSeconds: 86_400,
  sessionCacheTTLSeconds: 7_200,
  accessCacheTTLSeconds: 600,
});

const form = reactive<AuthPlatformForm>(defaultForm());

const canList = computed(() => access.hasPermission("auth:platform:list"));
const canCreate = computed(() => access.hasPermission("auth:platform:create"));
const canUpdate = computed(() => access.hasPermission("auth:platform:update"));
const canStatus = computed(() => access.hasPermission("auth:platform:status"));
const canDelete = computed(() => access.hasPermission("auth:platform:delete"));
const isEditing = computed(() => dialogMode.value === "edit");
const isBuiltinAdminEdit = computed(() => {
  const platform = editingPlatform.value;
  return (
    dialogMode.value === "edit" &&
    platform?.code === "admin" &&
    platform.isBuiltin === YesNo.Yes
  );
});
const formValid = computed(() => {
  const codeValid =
    dialogMode.value === "edit" ||
    /^[a-z][a-z0-9_]{1,48}$/.test(form.code.trim());
  return (
    codeValid &&
    form.name.trim() !== "" &&
    form.name.trim().length <= 64 &&
    inRange(form.accessTTLSeconds, 60, 2_592_000) &&
    inRange(form.refreshTTLSeconds, 60, 31_536_000) &&
    inRange(form.sessionCacheTTLSeconds, 60, 86_400) &&
    inRange(form.accessCacheTTLSeconds, 60, 86_400) &&
    inRange(form.maxSessions, 0, 100)
  );
});

async function loadPage(): Promise<void> {
  if (!canList.value) return;
  loading.value = true;
  loadError.value = "";
  try {
    const result = await getAuthPlatforms(query.value);
    rows.value = result.list;
    total.value = result.total;
  } catch (error: unknown) {
    loadError.value = errorMessage(error, "authPlatform.loadFailed");
  } finally {
    loading.value = false;
  }
}

function search(): void {
  const normalizedKeyword = keyword.value.trim();
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(normalizedKeyword === "" ? {} : { keyword: normalizedKeyword }),
    ...(statusFilter.value === "" ? {} : { isEnabled: statusFilter.value }),
  };
  void loadPage();
}

function reset(): void {
  keyword.value = "";
  statusFilter.value = "";
  query.value = { page: 1, pageSize: query.value.pageSize };
  void loadPage();
}

function refresh(): void {
  void loadPage();
}

function changePage(page: number): void {
  query.value = { ...query.value, page };
  void loadPage();
}

function changePageSize(pageSize: number): void {
  query.value = { ...query.value, page: 1, pageSize };
  void loadPage();
}

function updateTablePagination(next: TablePaginationState): void {
  if (next.pageSize !== query.value.pageSize) {
    changePageSize(next.pageSize);
    return;
  }
  changePage(next.currentPage);
}

function openCreate(): void {
  dialogMode.value = "create";
  editingPlatform.value = null;
  Object.assign(form, defaultForm());
  mutationError.value = "";
  dialogVisible.value = true;
}

function openEdit(platform: AuthPlatformListItem): void {
  dialogMode.value = "edit";
  editingPlatform.value = platform;
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
    allowRegister:
      platform.code === "admin" && platform.isBuiltin === YesNo.Yes
        ? YesNo.No
        : platform.allowRegister,
    isEnabled: platform.isEnabled,
  });
  mutationError.value = "";
  dialogVisible.value = true;
}

async function submit(): Promise<void> {
  if (!formValid.value || submitting.value) return;
  if (dialogMode.value === "edit" && editingPlatform.value !== null) {
    if (
      form.maxSessions < editingPlatform.value.maxSessions &&
      form.maxSessions > 0 &&
      !(await confirmAction("authPlatform.confirm.limit"))
    )
      return;
    if (
      securityChanged(editingPlatform.value) &&
      !(await confirmAction("authPlatform.confirm.security"))
    )
      return;
  }
  submitting.value = true;
  mutationError.value = "";
  try {
    if (dialogMode.value === "create") {
      await createAuthPlatform(createInput());
      query.value = { ...query.value, page: 1 };
    } else if (editingPlatform.value !== null) {
      await updateAuthPlatform(editingPlatform.value.id, updateInput());
    }
    await loadPage();
    dialogVisible.value = false;
    ElNotification.success({
      title: t(
        dialogMode.value === "create"
          ? "authPlatform.success.created"
          : "authPlatform.success.updated",
      ),
    });
  } catch (error: unknown) {
    mutationError.value = errorMessage(error, "authPlatform.mutationFailed");
  } finally {
    submitting.value = false;
  }
}

async function toggleStatus(platform: AuthPlatformListItem): Promise<void> {
  if (!canStatus.value) return;
  const next = platform.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes;
  if (
    next === YesNo.No &&
    !(await confirmAction("authPlatform.confirm.disable"))
  )
    return;
  try {
    await updateAuthPlatformStatus(platform.id, next);
    await loadPage();
    ElNotification.success({ title: t("authPlatform.success.status") });
  } catch (error: unknown) {
    mutationError.value = errorMessage(error, "authPlatform.mutationFailed");
  }
}

async function remove(platform: AuthPlatformListItem): Promise<void> {
  if (!canDelete.value || platform.isBuiltin === YesNo.Yes) return;
  if (!(await confirmAction("authPlatform.confirm.delete"))) return;
  try {
    await deleteAuthPlatform(platform.id);
    const maxPage = Math.max(
      1,
      Math.ceil((total.value - 1) / query.value.pageSize),
    );
    if (query.value.page > maxPage)
      query.value = { ...query.value, page: maxPage };
    await loadPage();
    ElNotification.success({ title: t("authPlatform.success.deleted") });
  } catch (error: unknown) {
    mutationError.value = errorMessage(error, "authPlatform.mutationFailed");
  }
}

async function confirmAction(
  key:
    | "authPlatform.confirm.disable"
    | "authPlatform.confirm.delete"
    | "authPlatform.confirm.limit"
    | "authPlatform.confirm.security",
): Promise<boolean> {
  try {
    await ElMessageBox.confirm(t(key), t("authPlatform.title"), {
      type: "warning",
    });
    return true;
  } catch (error: unknown) {
    return error !== "cancel" && error !== "close" ? false : false;
  }
}

function securityChanged(platform: AuthPlatformListItem): boolean {
  return (
    form.bindDevice !== platform.bindDevice ||
    form.bindIP !== platform.bindIP ||
    form.accessTTLSeconds !== platform.accessTTLSeconds ||
    form.refreshTTLSeconds !== platform.refreshTTLSeconds
  );
}

function createInput(): CreateAuthPlatformInput {
  return {
    code: form.code.trim(),
    name: form.name.trim(),
    accessTTLSeconds: form.accessTTLSeconds,
    refreshTTLSeconds: form.refreshTTLSeconds,
    sessionCacheTTLSeconds: form.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: form.accessCacheTTLSeconds,
    bindDevice: form.bindDevice,
    bindIP: form.bindIP,
    maxSessions: form.maxSessions,
    allowRegister: form.allowRegister,
    isEnabled: form.isEnabled,
  };
}

function updateInput(): UpdateAuthPlatformInput {
  return {
    name: form.name.trim(),
    accessTTLSeconds: form.accessTTLSeconds,
    refreshTTLSeconds: form.refreshTTLSeconds,
    sessionCacheTTLSeconds: form.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: form.accessCacheTTLSeconds,
    bindDevice: form.bindDevice,
    bindIP: form.bindIP,
    maxSessions: form.maxSessions,
    allowRegister: isBuiltinAdminEdit.value ? YesNo.No : form.allowRegister,
  };
}

function sessionLabel(value: number): string {
  if (value === 0) return t("authPlatform.unlimited");
  if (value === 1) return t("authPlatform.singleSession");
  return t("authPlatform.maxSessions", { count: value });
}

function ttlLabel(value: number): string {
  if (value % 86_400 === 0)
    return t("authPlatform.readableDays", { count: value / 86_400 });
  if (value % 3_600 === 0)
    return t("authPlatform.readableHours", { count: value / 3_600 });
  if (value % 60 === 0)
    return t("authPlatform.readableMinutes", { count: value / 60 });
  return t("authPlatform.seconds", { count: value });
}

function formatUpdatedDate(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return new Intl.DateTimeFormat(locale.value, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(parsed);
}

function formatUpdatedTime(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  return new Intl.DateTimeFormat(locale.value, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(parsed);
}

function defaultForm(): AuthPlatformForm {
  return {
    code: "",
    name: "",
    ...defaultTTL,
    bindDevice: YesNo.Yes,
    bindIP: YesNo.No,
    maxSessions: 1,
    allowRegister: YesNo.No,
    isEnabled: YesNo.Yes,
  };
}

function restoreDefaultTTL(): void {
  Object.assign(form, defaultTTL);
}

function inRange(value: number, minimum: number, maximum: number): boolean {
  return Number.isInteger(value) && value >= minimum && value <= maximum;
}

function errorMessage(
  error: unknown,
  fallbackKey: "authPlatform.loadFailed" | "authPlatform.mutationFailed",
): string {
  return error instanceof Error && error.message !== ""
    ? error.message
    : t(fallbackKey);
}

onMounted(() => {
  void loadPage();
});
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
          >{{ t("authPlatform.create") }}</el-button
        >
      </template>
      <template #cell-platform="{ row }: { row: AuthPlatformListItem }">
        <div class="auth-platform-identity auth-platform-identity--centered">
          <strong>{{ row.name }}</strong>
          <div class="auth-platform-identity__meta">
            <code>{{ row.code }}</code>
            <el-tag v-if="row.isBuiltin === YesNo.Yes" size="small" type="info" effect="plain">{{
              t("authPlatform.builtin")
            }}</el-tag>
          </div>
        </div>
      </template>
      <template #cell-tokenTTL="{ row }: { row: AuthPlatformListItem }">
        <div class="auth-platform-policy-stack">
          <div><span>{{ t("authPlatform.accessToken") }}</span><strong :title="t('authPlatform.seconds', { count: row.accessTTLSeconds })">{{ ttlLabel(row.accessTTLSeconds) }}</strong></div>
          <div><span>{{ t("authPlatform.refreshToken") }}</span><strong :title="t('authPlatform.seconds', { count: row.refreshTTLSeconds })">{{ ttlLabel(row.refreshTTLSeconds) }}</strong></div>
        </div>
      </template>
      <template #cell-cacheTTL="{ row }: { row: AuthPlatformListItem }">
        <div class="auth-platform-policy-stack">
          <div><span>{{ t("authPlatform.sessionCache") }}</span><strong :title="t('authPlatform.seconds', { count: row.sessionCacheTTLSeconds })">{{ ttlLabel(row.sessionCacheTTLSeconds) }}</strong></div>
          <div><span>{{ t("authPlatform.accessCache") }}</span><strong :title="t('authPlatform.seconds', { count: row.accessCacheTTLSeconds })">{{ ttlLabel(row.accessCacheTTLSeconds) }}</strong></div>
        </div>
      </template>
      <template #cell-security="{ row }: { row: AuthPlatformListItem }">
        <div class="auth-platform-tag-list" data-testid="auth-platform-security">
          <el-tag size="small" effect="plain" :type="row.bindDevice === YesNo.Yes ? 'success' : 'info'">{{
            t(row.bindDevice === YesNo.Yes ? "authPlatform.deviceBound" : "authPlatform.deviceUnbound")
          }}</el-tag>
          <el-tag size="small" effect="plain" :type="row.bindIP === YesNo.Yes ? 'success' : 'info'">{{
            t(row.bindIP === YesNo.Yes ? "authPlatform.ipBound" : "authPlatform.ipUnbound")
          }}</el-tag>
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
        >{{ t(row.allowRegister === YesNo.Yes ? "authPlatform.registrationAllowed" : "authPlatform.registrationDenied") }}</el-tag>
      </template>
      <template #cell-status="{ row }: { row: AuthPlatformListItem }"
        ><el-tag size="small" :type="row.isEnabled === YesNo.Yes ? 'success' : 'danger'">{{
          t(
            row.isEnabled === YesNo.Yes
              ? "authPlatform.enabled"
              : "authPlatform.disabled",
          )
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
            >{{ t("authPlatform.edit") }}</el-button
          >
          <el-button
            v-if="canStatus"
            text
            type="warning"
            data-testid="auth-platform-status"
            @click="toggleStatus(row)"
            >{{ t(row.isEnabled === YesNo.Yes ? "authPlatform.disable" : "authPlatform.enable") }}</el-button
          >
          <el-button
            v-if="canDelete && row.isBuiltin === YesNo.No"
            text
            type="danger"
            data-testid="auth-platform-delete"
            @click="remove(row)"
            >{{ t("authPlatform.delete") }}</el-button
          >
        </template></template
      >
      <template #empty
        ><el-empty :description="t('authPlatform.empty')"
      /></template>
    </AppTable>

    <AppDialog
      v-model="dialogVisible"
      :title="
        t(
          dialogMode === 'create'
            ? 'authPlatform.createTitle'
            : 'authPlatform.editTitle',
        )
      "
      width="800px"
      :append-to-body="false"
    >
      <el-form label-position="top" class="auth-platform-form auth-platform-form-scroll" data-testid="auth-platform-form">
        <div class="auth-platform-form-section">
          <h3>{{ t("authPlatform.form.basicSection") }}</h3>
          <el-row :gutter="16" class="auth-platform-form-grid auth-platform-form-grid--basic">
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('authPlatform.code')">
                <el-input v-model="form.code" data-testid="auth-platform-code" :disabled="isEditing" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('authPlatform.name')">
                <el-input v-model="form.name" data-testid="auth-platform-name" />
              </el-form-item>
            </el-col>
          </el-row>
        </div>
        <div class="auth-platform-form-section">
          <div class="auth-platform-form-section__heading">
            <h3>{{ t("authPlatform.form.tokenSection") }}</h3>
            <el-button
              text
              type="primary"
              :icon="RotateCcw"
              data-testid="auth-platform-ttl-defaults"
              @click="restoreDefaultTTL"
            >{{ t("authPlatform.form.restoreTTLDefaults") }}</el-button>
          </div>
          <el-row :gutter="16" class="auth-platform-form-grid auth-platform-form-grid--four">
            <el-col v-for="field in [
              { key: 'accessTTLSeconds', testId: 'auth-platform-access-ttl', label: 'authPlatform.accessTTL', help: 'authPlatform.form.accessTTLHelp', min: 60, max: 2_592_000 },
              { key: 'refreshTTLSeconds', testId: 'auth-platform-refresh-ttl', label: 'authPlatform.refreshTTL', help: 'authPlatform.form.refreshTTLHelp', min: 60, max: 31_536_000 },
              { key: 'sessionCacheTTLSeconds', testId: 'auth-platform-session-cache-ttl', label: 'authPlatform.sessionCacheTTL', help: 'authPlatform.form.sessionCacheTTLHelp', min: 60, max: 86_400 },
              { key: 'accessCacheTTLSeconds', testId: 'auth-platform-access-cache-ttl', label: 'authPlatform.accessCacheTTL', help: 'authPlatform.form.accessCacheTTLHelp', min: 60, max: 86_400 },
            ]" :key="field.key" :xs="24" :sm="12" :lg="6">
            <el-form-item>
              <template #label>
                <span class="auth-platform-field-label auth-platform-field-label--nowrap">{{ t(field.label) }}
                  <el-tooltip :content="t(field.help)" placement="top">
                    <CircleHelp data-testid="auth-platform-ttl-help" aria-hidden="true" />
                  </el-tooltip>
                </span>
              </template>
              <el-input-number v-model="form[field.key]" :min="field.min" :max="field.max" class="auth-platform-number" :data-testid="field.testId" />
            </el-form-item>
            </el-col>
          </el-row>
        </div>
        <div class="auth-platform-form-section">
          <h3>{{ t("authPlatform.form.policySection") }}</h3>
          <el-row :gutter="16" class="auth-platform-form-grid auth-platform-form-grid--four auth-platform-policy-grid auth-platform-policy-grid--three-up">
            <el-col :xs="8" :sm="8" :lg="8">
            <el-form-item :label="t('authPlatform.bindDevice')">
              <el-switch v-model="form.bindDevice" :active-value="YesNo.Yes" :inactive-value="YesNo.No" />
            </el-form-item>
            </el-col>
            <el-col :xs="8" :sm="8" :lg="8">
            <el-form-item :label="t('authPlatform.bindIP')">
              <el-switch v-model="form.bindIP" :active-value="YesNo.Yes" :inactive-value="YesNo.No" />
            </el-form-item>
            </el-col>
            <el-col :xs="8" :sm="8" :lg="8">
            <el-form-item :label="t('authPlatform.allowRegister')">
              <el-switch v-model="form.allowRegister" :active-value="YesNo.Yes" :inactive-value="YesNo.No" :disabled="isBuiltinAdminEdit" data-testid="auth-platform-allow-register" />
              <span v-if="isBuiltinAdminEdit" class="auth-platform-form-help">{{ t("authPlatform.adminRegistrationLocked") }}</span>
            </el-form-item>
            </el-col>
            <el-col v-if="dialogMode === 'create'" :xs="8" :sm="8" :lg="8">
            <el-form-item :label="t('authPlatform.isEnabled')">
              <el-switch v-model="form.isEnabled" :active-value="YesNo.Yes" :inactive-value="YesNo.No" />
            </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="16" class="auth-platform-form-grid auth-platform-form-grid--session">
            <el-col :xs="24" :sm="12" :lg="8">
            <el-form-item :label="t('authPlatform.maxSessionsField')">
              <el-input-number v-model="form.maxSessions" :min="0" :max="100" class="auth-platform-number" />
            </el-form-item>
            </el-col>
          </el-row>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{
          t("authPlatform.cancel")
        }}</el-button>
        <el-button
          type="primary"
          :loading="submitting"
          :disabled="!formValid"
          @click="submit"
          >{{ t("authPlatform.save") }}</el-button
        >
      </template>
    </AppDialog>
  </section>
</template>

<style scoped lang="scss">
.auth-platform-page {
  min-width: 0;
}
.auth-platform-form-help {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.4;
}
.auth-platform-filters {
  display: flex;
  align-items: center;
  gap: 12px;
}
.auth-platform-filters .el-input {
  width: 260px;
}
.auth-platform-filters .el-select-v2 {
  width: 150px;
}
.auth-platform-identity,
.auth-platform-policy-stack,
.auth-platform-updated {
  display: flex;
  min-width: 0;
  flex-direction: column;
}
.auth-platform-identity {
  align-items: center;
  gap: 5px;
  text-align: center;
}
.auth-platform-identity--centered {
  width: 100%;
}
.auth-platform-identity__meta {
  display: flex;
  align-items: center;
  gap: 6px;
}
.auth-platform-identity code {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.auth-platform-policy-stack {
  width: max-content;
  max-width: 100%;
  gap: 6px;
  margin: 0 auto;
  text-align: left;
}
.auth-platform-policy-stack > div {
  display: grid;
  grid-template-columns: 68px minmax(0, 1fr);
  align-items: baseline;
  gap: 8px;
}
.auth-platform-policy-stack span,
.auth-platform-updated small {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.auth-platform-policy-stack strong {
  overflow: hidden;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.auth-platform-tag-list {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 6px;
}
.auth-platform-updated {
  align-items: center;
  gap: 3px;
  text-align: center;
  line-height: 1.25;
}
.auth-platform-form {
  display: flex;
  flex-direction: column;
  gap: 22px;
}
.auth-platform-form-scroll {
  max-height: min(68vh, 620px);
  overflow-y: auto;
  padding-right: 8px;
}
.auth-platform-form-section {
  min-width: 0;
}
.auth-platform-form-section h3 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 14px;
  font-weight: 600;
}
.auth-platform-form-section__heading {
  display: flex;
  min-height: 32px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.auth-platform-form-section > h3 {
  margin-bottom: 14px;
}
.auth-platform-field-label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.auth-platform-field-label--nowrap {
  white-space: nowrap;
}
.auth-platform-field-label svg {
  width: 15px;
  height: 15px;
  color: var(--el-text-color-secondary);
  cursor: help;
}
.auth-platform-form-grid {
  width: 100%;
}
.auth-platform-form-grid--session {
  margin-top: 12px;
}
.auth-platform-form :deep(.el-form-item) {
  min-width: 0;
  margin-bottom: 0;
}
.auth-platform-form :deep(.el-input-number),
.auth-platform-number {
  width: 100%;
}
@media (max-width: 760px) {
  .auth-platform-filters {
    align-items: stretch;
    flex-direction: column;
  }
  .auth-platform-filters .el-input,
  .auth-platform-filters .el-select {
    width: 100%;
  }
}
@media (max-width: 480px) {
  .auth-platform-policy-grid {
    --el-row-gutter: 8px;
  }
}
</style>
