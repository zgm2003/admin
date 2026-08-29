<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessageBox, ElNotification } from "element-plus";
import { CirclePlus, Refresh } from "@element-plus/icons-vue";
import { useI18n } from "vue-i18n";

import {
  createMenu,
  deleteMenu,
  getMenus,
  isComponentPath,
  isMenuI18nKey,
  isMenuIcon,
  isMenuPath,
  menuCodePattern,
  updateMenu,
  updateMenuStatus,
} from "../../../api/rbac/menu";
import type {
  CreateMenuInput,
  ManagedMenuNode,
  ManagedMenuType,
  MenuPlatformOption,
  UpdateMenuInput,
} from "../../../api/rbac/menu";
import { DIcon } from "../../../components/DIcon";
import { YesNo } from "../../../enums/yes-no";
import { useAccessStore } from "../../../store/access";
import { AppDialog } from "../../../components/AppDialog";
import { IconSelect } from "../../../components/IconSelect";
import type { MenuIconName } from "../../../icons/menu-icons";
import { filterManagedMenuTree } from "./filter-menu-tree";

const { t } = useI18n();
const access = useAccessStore();

const menus = ref<ManagedMenuNode[]>([]);
const platforms = ref<MenuPlatformOption[]>([]);
const activePlatformID = ref<number | null>(null);
const keyword = ref("");
const expandedIDs = ref<Set<number>>(new Set());
const expansionBeforeSearch = ref<Set<number> | null>(null);
const loading = ref(false);
const loadError = ref("");
const mutationError = ref("");
const dialogVisible = ref(false);
const iconSelectVisible = ref(false);
const dialogMode = ref<"create" | "edit">("create");
const editingID = ref<number | null>(null);

interface MenuFormState {
  parentId: number | null;
  menuType: ManagedMenuType;
  name: string;
  code: string;
  i18nKey: string;
  path: string | null;
  componentPath: string | null;
  icon: MenuIconName | null;
  sortOrder: number;
  isEnabled: YesNo;
  isHidden: YesNo;
  isProtected: YesNo;
}

const form = ref<MenuFormState>(newForm());

const canCreate = computed(() => access.hasPermission("rbac:menu:create"));
const canUpdate = computed(() => access.hasPermission("rbac:menu:update"));
const canDelete = computed(() => access.hasPermission("rbac:menu:delete"));
const activePlatform = computed(() => {
  if (activePlatformID.value === null) return null;
  return (
    platforms.value.find(
      (platform) => platform.id === activePlatformID.value,
    ) ?? null
  );
});

function menuTypeLabel(menuType: ManagedMenuType): string {
  return t(`menu.type.${menuType}`);
}

function menuTypeTag(
  menuType: ManagedMenuType,
): "primary" | "success" | "warning" {
  if (menuType === "directory") return "primary";
  if (menuType === "page") return "success";
  return "warning";
}

function publicErrorMessage(error: unknown): string {
  return error instanceof Error && error.message !== ""
    ? error.message
    : t("menu.loadFailed");
}

function newForm(): MenuFormState {
  return {
    parentId: null,
    menuType: "directory",
    name: "",
    code: "",
    i18nKey: "navigation.system",
    path: null,
    componentPath: null,
    icon: null,
    sortOrder: 100,
    isEnabled: YesNo.Yes,
    isHidden: YesNo.No,
    isProtected: YesNo.No,
  };
}

function flattenMenus(nodes: readonly ManagedMenuNode[]): ManagedMenuNode[] {
  const result: ManagedMenuNode[] = [];
  const stack = [...nodes].reverse();
  while (stack.length > 0) {
    const node = stack.pop();
    if (node === undefined) continue;
    result.push(node);
    stack.push(...[...node.children].reverse());
  }
  return result;
}

function collectSubtreeIDs(node: ManagedMenuNode): Set<number> {
  const ids = new Set<number>();
  const stack = [node];
  while (stack.length > 0) {
    const current = stack.pop();
    if (current === undefined) continue;
    ids.add(current.id);
    stack.push(...current.children);
  }
  return ids;
}

const editingNode = computed(() => {
  if (editingID.value === null) return null;
  return (
    flattenMenus(menus.value).find((node) => node.id === editingID.value) ??
    null
  );
});
const editingProtected = computed(
  () => dialogMode.value === "edit" && form.value.isProtected === YesNo.Yes,
);

const parentOptions = computed(() => {
  const excluded =
    editingNode.value === null
      ? new Set<number>()
      : collectSubtreeIDs(editingNode.value);
  return flattenMenus(menus.value).filter((node) => {
    if (excluded.has(node.id)) return false;
    if (form.value.menuType === "directory")
      return node.menuType === "directory";
    if (form.value.menuType === "page") return node.menuType === "directory";
    return node.menuType === "page";
  });
});
const displayedMenus = computed(() =>
  filterManagedMenuTree(menus.value, keyword.value),
);
const expandedRowKeys = computed(() => [...expandedIDs.value]);

function flattenWithChildren(
  nodes: readonly ManagedMenuNode[],
): ManagedMenuNode[] {
  return flattenMenus(nodes);
}

function setExpandedForRoots(): void {
  expandedIDs.value = new Set(
    menus.value
      .filter((node) => node.children.length > 0)
      .map((node) => node.id),
  );
}

function expandAll(): void {
  expandedIDs.value = new Set(
    flattenWithChildren(displayedMenus.value)
      .filter((node) => node.children.length > 0)
      .map((node) => node.id),
  );
}

function collapseAll(): void {
  expandedIDs.value = new Set();
}

function updateKeyword(value: string): void {
  const wasEmpty = keyword.value.trim() === "";
  const isEmpty = value.trim() === "";
  if (wasEmpty && !isEmpty)
    expansionBeforeSearch.value = new Set(expandedIDs.value);
  keyword.value = value;
  if (!isEmpty) expandAll();
  else if (!wasEmpty)
    expandedIDs.value = expansionBeforeSearch.value ?? new Set();
}

const canSubmitForm = computed(() => {
  if (
    form.value.name === "" ||
    form.value.name.trim() !== form.value.name ||
    form.value.name.length > 128
  )
    return false;
  if (form.value.code.length > 128 || !menuCodePattern.test(form.value.code))
    return false;
  if (form.value.menuType !== "action" && !isMenuI18nKey(form.value.i18nKey))
    return false;
  if (form.value.icon !== null && !isMenuIcon(form.value.icon)) return false;
  if (form.value.menuType === "page") {
    return (
      form.value.path !== null &&
      isMenuPath(form.value.path) &&
      form.value.componentPath !== null &&
      isComponentPath(form.value.componentPath)
    );
  }
  if (form.value.menuType === "directory") {
    return form.value.path === null && form.value.componentPath === null;
  }
  return (
    form.value.path === null &&
    form.value.componentPath === null &&
    form.value.icon === null &&
    form.value.isHidden === YesNo.Yes
  );
});

const rootParentValue = "__root__" as const;

const parentSelectOptions = computed<
  Array<{ label: string; value: number | typeof rootParentValue }>
>(() => [
  { label: t("menu.form.root"), value: rootParentValue },
  ...parentOptions.value.map((node) => ({
    label: parentLabel(node),
    value: node.id,
  })),
]);
const menuTypeOptions = computed<Array<{ label: string; value: ManagedMenuType }>>(() => [
  { label: t("menu.type.directory"), value: "directory" },
  { label: t("menu.type.page"), value: "page" },
  { label: t("menu.type.action"), value: "action" },
]);
const tableHeaderCellStyle = { background: "var(--el-fill-color-light)" };

const parentSelection = computed<number | typeof rootParentValue>({
  get: () => form.value.parentId ?? rootParentValue,
  set: (value) => {
    form.value.parentId = value === rootParentValue ? null : value;
  },
});

function openIconSelect(): void {
  iconSelectVisible.value = true;
}

function selectMenuIcon(value: MenuIconName): void {
  form.value.icon = value;
}

function clearMenuIcon(): void {
  form.value.icon = null;
}

function parentLabel(node: ManagedMenuNode): string {
  return `${node.name} (${node.code})`;
}

function handleFormTypeChange(nextType: ManagedMenuType): void {
  const previousType = form.value.menuType;
  form.value.menuType = nextType;
  if (previousType === "action" && nextType !== "action") {
    form.value.isHidden = YesNo.No;
    form.value.i18nKey = "";
  }
  if (nextType === "directory") {
    form.value.path = null;
    form.value.componentPath = null;
  } else if (nextType === "page") {
    form.value.path = form.value.path ?? "";
    form.value.componentPath = form.value.componentPath ?? "";
  } else {
    form.value.path = null;
    form.value.componentPath = null;
    form.value.icon = null;
    form.value.isHidden = YesNo.Yes;
    form.value.i18nKey = "";
  }
  if (
    form.value.parentId !== null &&
    !parentOptions.value.some((node) => node.id === form.value.parentId)
  ) {
    form.value.parentId = null;
  }
}

function openCreate(parent: ManagedMenuNode | null = null): void {
  const next = newForm();
  if (parent !== null) {
    next.parentId = parent.id;
    next.menuType = parent.menuType === "directory" ? "page" : "action";
    if (next.menuType === "page") {
      next.path = "";
      next.componentPath = "";
    } else {
      next.isHidden = YesNo.Yes;
    }
  }
  dialogMode.value = "create";
  editingID.value = null;
  mutationError.value = "";
  form.value = next;
  dialogVisible.value = true;
}

function openEdit(node: ManagedMenuNode): void {
  dialogMode.value = "edit";
  editingID.value = node.id;
  mutationError.value = "";
  form.value = {
    parentId: node.parentId,
    menuType: node.menuType,
    name: node.name,
    code: node.code,
    i18nKey: node.i18nKey ?? "",
    path: node.path,
    componentPath: node.componentPath,
    icon: node.icon,
    sortOrder: node.sortOrder,
    isEnabled: node.isEnabled,
    isHidden: node.isHidden,
    isProtected: node.isProtected,
  };
  dialogVisible.value = true;
}

function closeDialog(): void {
  dialogVisible.value = false;
  editingID.value = null;
}

async function submitForm(): Promise<void> {
  if (!canSubmitForm.value) return;
  mutationError.value = "";
  try {
    if (dialogMode.value === "create") {
      if (activePlatformID.value === null) {
        mutationError.value = t("menu.platform.unavailable");
        return;
      }
      const input: CreateMenuInput = {
        platformId: activePlatformID.value,
        parentId: form.value.parentId,
        menuType: form.value.menuType,
        name: form.value.name,
        code: form.value.code,
        i18nKey: form.value.menuType === "action" ? null : form.value.i18nKey,
        path: form.value.path,
        componentPath: form.value.componentPath,
        icon: form.value.icon,
        sortOrder: form.value.sortOrder,
        isEnabled: form.value.isEnabled,
        isHidden: form.value.isHidden,
      };
      await createMenu(input);
      await reloadMenus();
      closeDialog();
      notifyMutation("menu.success.created");
      return;
    }
    if (editingID.value === null) return;
    const input: UpdateMenuInput = {
      parentId: form.value.parentId,
      menuType: form.value.menuType,
      name: form.value.name,
      i18nKey: form.value.menuType === "action" ? null : form.value.i18nKey,
      path: form.value.path,
      componentPath: form.value.componentPath,
      icon: form.value.icon,
      sortOrder: form.value.sortOrder,
      isHidden: form.value.isHidden,
    };
    await updateMenu(editingID.value, input);
    await reloadMenus();
    closeDialog();
    notifyMutation("menu.success.updated");
  } catch (error: unknown) {
    mutationError.value = publicErrorMessage(error);
  }
}

function notifyMutation(messageKey: string): void {
  ElNotification.success({
    title: t(messageKey),
    message: t("menu.success.refreshHint"),
  });
}

async function changeStatus(node: ManagedMenuNode): Promise<void> {
  const nextValue = node.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes;
  try {
    if (nextValue === YesNo.No) {
      await ElMessageBox.confirm(
        t("menu.confirm.disableMessage"),
        t("menu.confirm.disableTitle"),
        {
          confirmButtonText: t("menu.confirm.confirm"),
          cancelButtonText: t("menu.confirm.cancel"),
          type: "warning",
        },
      );
    }
    await updateMenuStatus(node.id, nextValue);
    await reloadMenus();
    notifyMutation("menu.success.statusChanged");
  } catch (error: unknown) {
    if (error === "cancel" || error === "close") return;
    mutationError.value = publicErrorMessage(error);
  }
}

async function removeNode(node: ManagedMenuNode): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t("menu.confirm.deleteMessage"),
      t("menu.confirm.deleteTitle"),
      {
        confirmButtonText: t("menu.confirm.confirm"),
        cancelButtonText: t("menu.confirm.cancel"),
        type: "warning",
      },
    );
    await deleteMenu(node.id);
    await reloadMenus();
    notifyMutation("menu.success.deleted");
  } catch (error: unknown) {
    if (error === "cancel" || error === "close") return;
    mutationError.value = publicErrorMessage(error);
  }
}

async function loadMenus(platformID?: number): Promise<void> {
  loading.value = true;
  loadError.value = "";
  try {
    const result =
      platformID === undefined
        ? await getMenus()
        : await getMenus({ platformId: platformID });
    const selectedPlatform =
      platformID === undefined
        ? (result.platforms.find((platform) => platform.code === "admin") ??
          result.platforms[0])
        : result.platforms.find((platform) => platform.id === platformID);
    if (selectedPlatform === undefined) {
      throw new Error(t("menu.platform.unavailable"));
    }
    platforms.value = result.platforms;
    activePlatformID.value = selectedPlatform.id;
    menus.value = result.menuTree;
    if (keyword.value.trim() === "") setExpandedForRoots();
  } catch (error: unknown) {
    loadError.value = publicErrorMessage(error);
  } finally {
    loading.value = false;
  }
}

async function reloadMenus(): Promise<void> {
  if (activePlatformID.value === null) {
    await loadMenus();
    return;
  }
  await loadMenus(activePlatformID.value);
}

async function switchPlatform(value: string | number): Promise<void> {
  const platformID = typeof value === "number" ? value : Number(value);
  if (!Number.isInteger(platformID) || platformID < 1) {
    loadError.value = t("menu.platform.unavailable");
    return;
  }
  menus.value = [];
  expandedIDs.value = new Set();
  expansionBeforeSearch.value = null;
  await loadMenus(platformID);
}

onMounted(() => loadMenus());
</script>

<template>
  <section
    class="menu-management-page management-page"
    :aria-label="t('menu.title')"
  >
    <el-tabs
      v-if="platforms.length > 0"
      v-model="activePlatformID"
      data-testid="menu-platform-tabs"
      class="menu-platform-tabs"
      @tab-change="switchPlatform"
    >
      <el-tab-pane
        v-for="platform in platforms"
        :key="platform.id"
        :name="platform.id"
      >
        <template #label>
          <span class="menu-platform-tab">
            <span>{{ platform.name }}</span>
            <code>{{ platform.code }}</code>
            <el-tag
              v-if="platform.isEnabled === YesNo.No"
              size="small"
              type="info"
              effect="plain"
            >
              {{ t("menu.platform.disabled") }}
            </el-tag>
          </span>
        </template>
      </el-tab-pane>
    </el-tabs>
    <div class="menu-management__toolbar-actions management-page__actions">
      <el-input
        data-testid="menu-search"
        :model-value="keyword"
        clearable
        :placeholder="t('menu.search.placeholder')"
        @update:model-value="updateKeyword"
      />
      <el-button data-testid="menu-expand-all" @click="expandAll">{{
        t("menu.expandAll")
      }}</el-button>
      <el-button data-testid="menu-collapse-all" @click="collapseAll">{{
        t("menu.collapseAll")
      }}</el-button>
      <el-button
        v-if="canCreate"
        data-testid="add-root-menu"
        type="primary"
        :icon="CirclePlus"
        :disabled="activePlatform === null"
        @click="openCreate()"
      >
        {{ t("menu.addRoot") }}
      </el-button>
      <el-button
        data-testid="refresh-menus"
        :icon="Refresh"
        :loading="loading"
        @click="reloadMenus"
      >
        {{ t("menu.refresh") }}
      </el-button>
    </div>
    <div class="menu-management__content">
      <el-alert
        v-if="loadError !== ''"
        data-testid="menu-load-error"
        type="error"
        :title="loadError"
        :closable="false"
        show-icon
      >
        <template #default>
          <el-button size="small" :icon="Refresh" @click="reloadMenus">
            {{ t("menu.retry") }}
          </el-button>
        </template>
      </el-alert>

      <el-alert
        v-if="mutationError !== ''"
        data-testid="menu-mutation-error"
        type="error"
        :title="mutationError"
        :closable="false"
        show-icon
      />

      <el-table
        v-if="loadError === ''"
        v-loading="loading"
        border
        data-testid="menu-table"
        class="menu-management__table"
        :data="displayedMenus"
        :expand-row-keys="expandedRowKeys"
        :header-cell-style="tableHeaderCellStyle"
        row-key="id"
        :tree-props="{ children: 'children' }"
        table-layout="fixed"
      >
        <el-table-column
          :label="t('menu.column.title')"
          min-width="190"
          align="center"
          header-align="center"
        >
          <template #default="{ row }: { row: ManagedMenuNode }">
            <span class="menu-title-cell">{{ row.name }}</span>
          </template>
        </el-table-column>

        <el-table-column
          :label="t('menu.column.type')"
          width="116"
          align="center"
          header-align="center"
        >
          <template #default="{ row }: { row: ManagedMenuNode }">
            <el-tag
              size="small"
              effect="plain"
              :type="menuTypeTag(row.menuType)"
            >
              {{ menuTypeLabel(row.menuType) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column
          prop="code"
          :label="t('menu.column.code')"
          min-width="190"
          align="center"
          header-align="center"
          show-overflow-tooltip
        />

        <el-table-column
          :label="t('menu.column.route')"
          min-width="180"
          align="center"
          header-align="center"
        >
          <template #default="{ row }: { row: ManagedMenuNode }">
            <code v-if="row.path !== null" class="menu-route-cell">{{
              row.path
            }}</code>
            <span v-else class="menu-cell-empty">-</span>
          </template>
        </el-table-column>

        <el-table-column
          :label="t('menu.column.componentPath')"
          min-width="190"
          align="center"
          header-align="center"
        >
          <template #default="{ row }: { row: ManagedMenuNode }">
            <code v-if="row.componentPath !== null" class="menu-route-cell">{{
              row.componentPath
            }}</code>
            <span v-else class="menu-cell-empty">-</span>
          </template>
        </el-table-column>

        <el-table-column
          :label="t('menu.column.icon')"
          width="112"
          align="center"
          header-align="center"
        >
          <template #default="{ row }: { row: ManagedMenuNode }">
            <span v-if="row.icon !== null" class="menu-icon-cell">
              <DIcon :icon="row.icon" />
              <span>{{ row.icon }}</span>
            </span>
            <span v-else class="menu-cell-empty">-</span>
          </template>
        </el-table-column>

        <el-table-column
          prop="sortOrder"
          :label="t('menu.column.sortOrder')"
          width="88"
          align="center"
          header-align="center"
        />

        <el-table-column
          :label="t('menu.column.visibility')"
          width="104"
          align="center"
          header-align="center"
        >
          <template #default="{ row }: { row: ManagedMenuNode }">
            <el-tag
              :type="row.isHidden === YesNo.No ? 'success' : 'info'"
              size="small"
              effect="plain"
            >
              {{
                row.isHidden === YesNo.No
                  ? t("menu.visibility.visible")
                  : t("menu.visibility.hidden")
              }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column
          :label="t('menu.column.status')"
          width="104"
          align="center"
          header-align="center"
        >
          <template #default="{ row }: { row: ManagedMenuNode }">
            <span :data-menu-enabled="row.isEnabled">
              <el-tag
                :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'"
                size="small"
                effect="plain"
              >
                {{
                  row.isEnabled === YesNo.Yes
                    ? t("menu.status.enabled")
                    : t("menu.status.disabled")
                }}
              </el-tag>
            </span>
          </template>
        </el-table-column>

        <el-table-column
          :label="t('menu.column.actions')"
          :width="280"
          fixed="right"
          align="center"
          header-align="center"
        >
          <template #default="{ row }: { row: ManagedMenuNode }">
            <el-button
              v-if="canCreate && row.menuType !== 'action'"
              :data-testid="`add-child-${row.id}`"
              text
              type="primary"
              @click="openCreate(row)"
              >{{ t("menu.addChild") }}</el-button
            >
            <el-button
              v-if="canUpdate"
              :data-testid="`edit-${row.id}`"
              text
              type="primary"
              @click="openEdit(row)"
              >{{ t("menu.edit") }}</el-button
            >
            <el-button
              v-if="canUpdate"
              :data-testid="`status-${row.id}`"
              text
              type="warning"
              :disabled="row.isProtected === YesNo.Yes"
              :title="
                row.isProtected === YesNo.Yes
                  ? t('menu.form.protectedHint')
                  : undefined
              "
              @click="changeStatus(row)"
              >{{
                row.isEnabled === YesNo.Yes
                  ? t("menu.disable")
                  : t("menu.enable")
              }}</el-button
            >
            <el-button
              v-if="canDelete"
              :data-testid="`delete-${row.id}`"
              text
              type="danger"
              :disabled="row.isProtected === YesNo.Yes"
              :title="
                row.isProtected === YesNo.Yes
                  ? t('menu.form.protectedHint')
                  : undefined
              "
              @click="removeNode(row)"
              >{{ t("menu.delete") }}</el-button
            >
          </template>
        </el-table-column>

        <template #empty>
          <div data-testid="menu-empty" class="menu-management__empty">
            {{ t("menu.empty") }}
          </div>
        </template>
      </el-table>
    </div>

    <AppDialog
      v-model="dialogVisible"
      :title="
        dialogMode === 'create'
          ? t('menu.form.createTitle')
          : t('menu.form.editTitle')
      "
      width="900px"
      data-testid="menu-dialog"
    >
      <el-alert
        v-if="mutationError !== ''"
        data-testid="menu-form-error"
        type="error"
        :title="mutationError"
        :closable="false"
        show-icon
      />
      <el-alert
        v-if="editingProtected"
        data-testid="menu-form-protected-hint"
        type="info"
        :title="t('menu.form.protectedHint')"
        :closable="false"
        show-icon
      />
      <el-form
        class="menu-form"
        label-position="right"
        label-width="96px"
        @submit.prevent="submitForm"
      >
        <el-row :gutter="24" class="menu-form__grid">
          <el-col v-if="dialogMode === 'edit'" :span="24">
          <el-form-item
            :label="t('menu.form.platform')"
          >
            <div data-testid="menu-form-platform" class="menu-form__readonly">
              <span>{{ activePlatform?.name }}</span>
              <code>{{ activePlatform?.code }}</code>
            </div>
          </el-form-item>
          </el-col>

          <el-col :xs="24" :sm="12">
          <el-form-item :label="t('menu.form.parent')">
            <el-select-v2
              v-model="parentSelection"
              data-testid="menu-form-parent"
              clearable
              :disabled="editingProtected"
              :title="
                editingProtected ? t('menu.form.protectedHint') : undefined
              "
              :options="parentSelectOptions"
            />
          </el-form-item>
          </el-col>

          <el-col :xs="24" :sm="12">
          <el-form-item :label="t('menu.form.menuType')">
            <el-select-v2
              :model-value="form.menuType"
              data-testid="menu-form-type"
              :disabled="editingProtected"
              :title="
                editingProtected ? t('menu.form.protectedHint') : undefined
              "
              :options="menuTypeOptions"
              @update:model-value="handleFormTypeChange"
            />
          </el-form-item>
          </el-col>

          <el-col :span="24">
          <el-form-item :label="t('menu.form.code')">
            <div class="menu-form__control">
              <el-input
                v-model="form.code"
                data-testid="menu-form-code"
                :readonly="dialogMode === 'edit'"
                :disabled="editingProtected"
                :title="
                  editingProtected ? t('menu.form.protectedHint') : undefined
                "
                :placeholder="t('menu.form.codePlaceholder')"
              />
              <p class="menu-form__hint">{{ t("menu.form.codeHint") }}</p>
            </div>
          </el-form-item>
          </el-col>

          <el-col :span="24">
          <el-form-item :label="t('menu.form.name')">
            <el-input
              v-model="form.name"
              data-testid="menu-form-name"
              maxlength="128"
            />
          </el-form-item>
          </el-col>

          <el-col v-if="form.menuType !== 'action'" :span="24">
          <el-form-item
            :label="t('menu.form.i18nKey')"
          >
            <div class="menu-form__control">
              <el-input
                v-model="form.i18nKey"
                data-testid="menu-form-i18n-key"
              />
              <p class="menu-form__hint">{{ t("menu.form.i18nKeyHint") }}</p>
            </div>
          </el-form-item>
          </el-col>

          <el-col v-if="form.menuType === 'page'" :span="24">
          <el-form-item
            v-if="form.menuType === 'page'"
            :label="t('menu.form.path')"
          >
            <div class="menu-form__control">
              <el-input
                v-model="form.path"
                data-testid="menu-form-path"
                :disabled="editingProtected"
                :title="
                  editingProtected ? t('menu.form.protectedHint') : undefined
                "
                :placeholder="t('menu.form.pathPlaceholder')"
              />
              <p class="menu-form__hint">{{ t("menu.form.pathHint") }}</p>
            </div>
          </el-form-item>
          </el-col>

          <el-col v-if="form.menuType === 'page'" :span="24">
          <el-form-item
            v-if="form.menuType === 'page'"
            :label="t('menu.form.componentPath')"
          >
            <div class="menu-form__control">
              <el-input
                v-model="form.componentPath"
                data-testid="menu-form-component-path"
                :disabled="editingProtected"
                :title="
                  editingProtected ? t('menu.form.protectedHint') : undefined
                "
                :placeholder="t('menu.form.componentPathPlaceholder')"
              />
              <p class="menu-form__hint">
                {{ t("menu.form.componentPathHint") }}
              </p>
            </div>
          </el-form-item>
          </el-col>

          <el-col :xs="24" :sm="12">
          <el-form-item
            v-if="form.menuType !== 'action'"
            :label="t('menu.form.icon')"
          >
            <div class="menu-icon-picker">
              <el-button
                data-testid="menu-form-icon"
                :disabled="editingProtected"
                @click="openIconSelect"
              >
                <DIcon v-if="form.icon !== null" :icon="form.icon" />
                {{ form.icon ?? t("menu.form.noIcon") }}
              </el-button>
              <el-button
                v-if="form.icon !== null"
                text
                type="danger"
                :disabled="editingProtected"
                @click="clearMenuIcon"
                >{{ t("menu.form.noIcon") }}</el-button
              >
            </div>
          </el-form-item>
          </el-col>

          <el-col :xs="24" :sm="12">
          <el-form-item :label="t('menu.form.sortOrder')">
            <el-input-number
              v-model="form.sortOrder"
              data-testid="menu-form-sort-order"
              :min="0"
              :step="10"
            />
          </el-form-item>
          </el-col>

          <el-col v-if="dialogMode === 'create'" :xs="24" :sm="12">
          <el-form-item
            v-if="dialogMode === 'create'"
            :label="t('menu.form.isEnabled')"
          >
            <el-switch
              v-model="form.isEnabled"
              :active-value="YesNo.Yes"
              :inactive-value="YesNo.No"
              data-testid="menu-form-enabled"
            />
          </el-form-item>
          </el-col>

          <el-col v-if="form.menuType !== 'action'" :xs="24" :sm="12">
          <el-form-item
            :label="t('menu.form.isHidden')"
          >
            <el-switch
              v-model="form.isHidden"
              :active-value="YesNo.No"
              :inactive-value="YesNo.Yes"
              :disabled="editingProtected"
              :title="
                editingProtected ? t('menu.form.protectedHint') : undefined
              "
              data-testid="menu-form-hidden"
            />
          </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <div class="menu-form-actions">
          <el-button data-testid="menu-form-cancel" @click="closeDialog">{{
            t("menu.form.cancel")
          }}</el-button>
          <el-button
            data-testid="menu-form-submit"
            type="primary"
            :disabled="!canSubmitForm"
            @click="submitForm"
          >
            {{ t("menu.form.submit") }}
          </el-button>
        </div>
      </template>
    </AppDialog>

    <IconSelect
      v-model="iconSelectVisible"
      :title="t('menu.form.icon')"
      :empty-text="t('menu.form.noIcon')"
      @select-icon="selectMenuIcon"
    />
  </section>
</template>

<style scoped lang="scss">
.menu-management-page {
  min-width: 0;
}

.menu-platform-tabs {
  min-width: 0;
  margin-bottom: 8px;
}

.menu-platform-tab,
.menu-form__readonly {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.menu-platform-tab code,
.menu-form__readonly code {
  color: var(--admin-text-soft);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 12px;
}

.menu-form__readonly {
  min-height: 32px;
  color: var(--admin-text);
}

.menu-management__toolbar-actions,
.menu-icon-cell {
  display: flex;
  align-items: center;
}

.menu-management__toolbar-actions {
  gap: 8px;
}

.menu-management__content {
  min-width: 0;
  padding: 0;
}

.menu-management__table {
  width: 100%;
  border: 1px solid var(--admin-border);
  border-radius: 6px;
}

.menu-title-cell {
  color: var(--admin-text);
  font-weight: 650;
}

.menu-route-cell {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.menu-route-cell {
  color: var(--admin-text);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 12px;
}

.menu-cell-empty {
  color: var(--admin-text-soft);
  font-size: 12px;
}

.menu-icon-cell {
  min-width: 0;
  gap: 6px;
}

.menu-management__empty {
  color: var(--admin-text-soft);
}

.menu-form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.menu-form__grid {
  width: 100%;
}

.menu-form__control,
.menu-icon-picker {
  width: 100%;
}

.menu-form__grid :deep(.el-input),
.menu-form__grid :deep(.el-select),
.menu-form__grid :deep(.el-input-number) {
  width: 100%;
}

.menu-icon-picker {
  display: flex;
  align-items: center;
  gap: 6px;
}

.menu-icon-picker .el-button:first-child {
  flex: 1;
}

.menu-form__hint {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}

@media (max-width: 720px) {
  .menu-management__toolbar-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }
}

@media (max-width: 480px) {
  .menu-management__toolbar-actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
