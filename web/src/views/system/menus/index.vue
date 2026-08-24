<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { CirclePlus, Refresh } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import { menuIcons } from '../../../access/menu-icons'
import { menuTitleKeys, type MenuTitleKey } from '../../../access/menu-title-keys'
import { routeViews, type RouteViewKey } from '../../../access/route-views'
import { createMenu, deleteMenu, getMenus, updateMenu, updateMenuStatus } from '../../../api/menu'
import type { CreateMenuInput, ManagedMenuNode, ManagedMenuType, UpdateMenuInput } from '../../../api/menu.contract'
import { YesNo } from '../../../enums/yes-no'
import { useAccessStore } from '../../../store/access'
import { IconSelect } from '../../../components/IconSelect'

const { t } = useI18n()
const access = useAccessStore()

const menus = ref<ManagedMenuNode[]>([])
const loading = ref(false)
const loadError = ref('')
const expandedRowKeys = ref<number[]>([])
const mutationError = ref('')
const drawerVisible = ref(false)
const iconSelectVisible = ref(false)
const drawerMode = ref<'create' | 'edit'>('create')
const editingID = ref<number | null>(null)

interface MenuFormState {
  parentId: number | null
  menuType: ManagedMenuType
  code: string
  i18nKey: MenuTitleKey
  path: string | null
  viewKey: RouteViewKey | null
  icon: keyof typeof menuIcons | null
  sortOrder: number
  isEnabled: YesNo
}

const form = ref<MenuFormState>(newForm())

const canCreate = computed(() => access.hasPermission('system:menu:create'))
const canUpdate = computed(() => access.hasPermission('system:menu:update'))
const canDelete = computed(() => access.hasPermission('system:menu:delete'))

function menuTypeLabel(menuType: ManagedMenuType): string {
  return t(`menu.type.${menuType}`)
}

function menuTypeTag(menuType: ManagedMenuType): 'primary' | 'success' | 'warning' {
  if (menuType === 'directory') return 'primary'
  if (menuType === 'page') return 'success'
  return 'warning'
}

function publicErrorMessage(error: unknown): string {
  return error instanceof Error && error.message !== '' ? error.message : t('menu.loadFailed')
}

function newForm(): MenuFormState {
  return {
    parentId: null,
    menuType: 'directory',
    code: '',
    i18nKey: 'navigation.system',
    path: null,
    viewKey: null,
    icon: null,
    sortOrder: 100,
    isEnabled: YesNo.Yes,
  }
}

function flattenMenus(nodes: readonly ManagedMenuNode[]): ManagedMenuNode[] {
  const result: ManagedMenuNode[] = []
  const stack = [...nodes].reverse()
  while (stack.length > 0) {
    const node = stack.pop()
    if (node === undefined) continue
    result.push(node)
    stack.push(...[...node.children].reverse())
  }
  return result
}

function collectSubtreeIDs(node: ManagedMenuNode): Set<number> {
  const ids = new Set<number>()
  const stack = [node]
  while (stack.length > 0) {
    const current = stack.pop()
    if (current === undefined) continue
    ids.add(current.id)
    stack.push(...current.children)
  }
  return ids
}

const editingNode = computed(() => {
  if (editingID.value === null) return null
  return flattenMenus(menus.value).find((node) => node.id === editingID.value) ?? null
})

const parentOptions = computed(() => {
  const excluded = editingNode.value === null ? new Set<number>() : collectSubtreeIDs(editingNode.value)
  return flattenMenus(menus.value).filter((node) => {
    if (excluded.has(node.id)) return false
    if (form.value.menuType === 'directory') return node.menuType === 'directory'
    if (form.value.menuType === 'page') return node.menuType === 'directory'
    return node.menuType === 'page'
  })
})

const canSubmitForm = computed(() => {
  if (form.value.code.trim() === '') return false
  if (form.value.menuType === 'page') return form.value.path !== null && form.value.path.trim() !== '' && form.value.viewKey !== null
  return form.value.menuType === 'directory' || (form.value.path === null && form.value.viewKey === null && form.value.icon === null)
})

const iconKeys = Object.keys(menuIcons) as Array<keyof typeof menuIcons>
const titleKeys = [...menuTitleKeys]
const viewKeys = Object.keys(routeViews) as RouteViewKey[]
const rootParentValue = '__root__' as const

const parentSelectOptions = computed<Array<{ label: string; value: number | typeof rootParentValue }>>(() => [
  { label: t('menu.form.root'), value: rootParentValue },
  ...parentOptions.value.map((node) => ({ label: parentLabel(node), value: node.id })),
])
const menuTypeOptions: Array<{ label: string; value: ManagedMenuType }> = [
  { label: t('menu.type.directory'), value: 'directory' },
  { label: t('menu.type.page'), value: 'page' },
  { label: t('menu.type.action'), value: 'action' },
]
const titleSelectOptions = computed(() => titleKeys.map((key) => ({ label: t(key), value: key })))
const viewSelectOptions = computed(() => viewKeys.map((key) => ({ label: key, value: key })))
const menuIconOptions = iconKeys.map((key) => ({ name: key, label: key }))

const parentSelection = computed<number | typeof rootParentValue>({
  get: () => form.value.parentId ?? rootParentValue,
  set: (value) => {
    form.value.parentId = value === rootParentValue ? null : value
  },
})

function openIconSelect(): void {
  iconSelectVisible.value = true
}

function selectMenuIcon(value: string): void {
  if (Object.prototype.hasOwnProperty.call(menuIcons, value)) {
    form.value.icon = value as keyof typeof menuIcons
  }
}

function clearMenuIcon(): void {
  form.value.icon = null
}

function parentLabel(node: ManagedMenuNode): string {
  return `${t(node.i18nKey)} (${node.code})`
}

function handleFormTypeChange(nextType: ManagedMenuType): void {
  form.value.menuType = nextType
  if (nextType === 'directory') {
    form.value.path = null
    form.value.viewKey = null
  } else if (nextType === 'page') {
    form.value.path = form.value.path ?? ''
    form.value.viewKey = form.value.viewKey ?? 'system-menus'
  } else {
    form.value.path = null
    form.value.viewKey = null
    form.value.icon = null
  }
  if (form.value.parentId !== null && !parentOptions.value.some((node) => node.id === form.value.parentId)) {
    form.value.parentId = null
  }
}

function openCreate(parent: ManagedMenuNode | null = null): void {
  const next = newForm()
  if (parent !== null) {
    next.parentId = parent.id
    next.menuType = parent.menuType === 'directory' ? 'page' : 'action'
    if (next.menuType === 'page') {
      next.path = ''
      next.viewKey = 'system-menus'
    }
  }
  drawerMode.value = 'create'
  editingID.value = null
  mutationError.value = ''
  form.value = next
  drawerVisible.value = true
}

function openEdit(node: ManagedMenuNode): void {
  drawerMode.value = 'edit'
  editingID.value = node.id
  mutationError.value = ''
  form.value = {
    parentId: node.parentId,
    menuType: node.menuType,
    code: node.code,
    i18nKey: node.i18nKey,
    path: node.path,
    viewKey: node.viewKey,
    icon: node.icon,
    sortOrder: node.sortOrder,
    isEnabled: node.isEnabled,
  }
  drawerVisible.value = true
}

function closeDrawer(): void {
  drawerVisible.value = false
  editingID.value = null
}

async function submitForm(): Promise<void> {
  if (!canSubmitForm.value) return
  mutationError.value = ''
  try {
    if (drawerMode.value === 'create') {
      const input: CreateMenuInput = { ...form.value }
      await createMenu(input)
      await loadMenus()
      closeDrawer()
      notifyMutation('menu.success.created')
      return
    }
    if (editingID.value === null) return
    const input: UpdateMenuInput = {
      parentId: form.value.parentId,
      menuType: form.value.menuType,
      i18nKey: form.value.i18nKey,
      path: form.value.path,
      viewKey: form.value.viewKey,
      icon: form.value.icon,
      sortOrder: form.value.sortOrder,
    }
    await updateMenu(editingID.value, input)
    await loadMenus()
    closeDrawer()
    notifyMutation('menu.success.updated')
  } catch (error: unknown) {
    mutationError.value = publicErrorMessage(error)
  }
}

function notifyMutation(messageKey: string): void {
  ElNotification.success({
    title: t(messageKey),
    message: t('menu.success.refreshHint'),
  })
}

async function changeStatus(node: ManagedMenuNode): Promise<void> {
  const nextValue = node.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes
  try {
    if (nextValue === YesNo.No) {
      await ElMessageBox.confirm(t('menu.confirm.disableMessage'), t('menu.confirm.disableTitle'), {
        confirmButtonText: t('menu.confirm.confirm'),
        cancelButtonText: t('menu.confirm.cancel'),
        type: 'warning',
      })
    }
    await updateMenuStatus(node.id, nextValue)
    await loadMenus()
    notifyMutation('menu.success.statusChanged')
  } catch (error: unknown) {
    if (error === 'cancel' || error === 'close') return
    mutationError.value = publicErrorMessage(error)
  }
}

async function removeNode(node: ManagedMenuNode): Promise<void> {
  try {
    await ElMessageBox.confirm(t('menu.confirm.deleteMessage'), t('menu.confirm.deleteTitle'), {
      confirmButtonText: t('menu.confirm.confirm'),
      cancelButtonText: t('menu.confirm.cancel'),
      type: 'warning',
    })
    await deleteMenu(node.id)
    await loadMenus()
    notifyMutation('menu.success.deleted')
  } catch (error: unknown) {
    if (error === 'cancel' || error === 'close') return
    mutationError.value = publicErrorMessage(error)
  }
}

function collectExpandedIDs(nodes: readonly ManagedMenuNode[]): number[] {
  const ids: number[] = []
  const stack = [...nodes]
  while (stack.length > 0) {
    const node = stack.pop()
    if (node === undefined) continue
    if (node.children.length > 0) ids.push(node.id)
    stack.push(...node.children)
  }
  return ids
}

async function loadMenus(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const result = await getMenus()
    menus.value = result
    expandedRowKeys.value = collectExpandedIDs(result)
  } catch (error: unknown) {
    loadError.value = publicErrorMessage(error)
  } finally {
    loading.value = false
  }
}

onMounted(loadMenus)
</script>

<template>
  <section class="menu-management-page system-page" :aria-label="t('menu.title')">
      <div class="menu-management__toolbar-actions system-page__actions">
        <el-button
          v-if="canCreate"
          data-testid="add-root-menu"
          type="primary"
          :icon="CirclePlus"
          @click="openCreate()"
        >
          {{ t('menu.addRoot') }}
        </el-button>
        <el-button
          data-testid="refresh-menus"
          :icon="Refresh"
          :loading="loading"
          @click="loadMenus"
        >
          {{ t('menu.refresh') }}
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
          <el-button size="small" :icon="Refresh" @click="loadMenus">
            {{ t('menu.retry') }}
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
        data-testid="menu-table"
        class="menu-management__table"
        :data="menus"
        row-key="id"
        :expand-row-keys="expandedRowKeys"
        :tree-props="{ children: 'children' }"
        table-layout="fixed"
      >
        <el-table-column :label="t('menu.column.title')" min-width="190">
          <template #default="{ row }: { row: ManagedMenuNode }">
            <span class="menu-title-cell">{{ t(row.i18nKey) }}</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('menu.column.type')" width="116">
          <template #default="{ row }: { row: ManagedMenuNode }">
            <el-tag size="small" effect="plain" :type="menuTypeTag(row.menuType)">
              {{ menuTypeLabel(row.menuType) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="code" :label="t('menu.column.code')" min-width="190" show-overflow-tooltip />

        <el-table-column :label="t('menu.column.route')" min-width="210">
          <template #default="{ row }: { row: ManagedMenuNode }">
            <div v-if="row.path !== null" class="menu-route-cell">
              <code>{{ row.path }}</code>
              <span>{{ row.viewKey }}</span>
            </div>
            <span v-else class="menu-cell-empty">-</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('menu.column.icon')" width="112">
          <template #default="{ row }: { row: ManagedMenuNode }">
            <span v-if="row.icon !== null" class="menu-icon-cell">
              <el-icon><component :is="menuIcons[row.icon]" /></el-icon>
              <span>{{ row.icon }}</span>
            </span>
            <span v-else class="menu-cell-empty">-</span>
          </template>
        </el-table-column>

        <el-table-column prop="sortOrder" :label="t('menu.column.sortOrder')" width="88" align="center" />

        <el-table-column :label="t('menu.column.status')" width="104">
          <template #default="{ row }: { row: ManagedMenuNode }">
            <span :data-menu-enabled="row.isEnabled">
              <el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'" size="small" effect="plain">
                {{ row.isEnabled === YesNo.Yes ? t('menu.status.enabled') : t('menu.status.disabled') }}
              </el-tag>
            </span>
          </template>
        </el-table-column>

        <el-table-column :label="t('menu.column.builtin')" width="76" align="center">
          <template #default="{ row }: { row: ManagedMenuNode }">
            {{ row.isBuiltin ? t('menu.builtin.yes') : t('menu.builtin.no') }}
          </template>
        </el-table-column>

        <el-table-column :label="t('menu.column.actions')" width="176" fixed="right" align="right">
          <template #default="{ row }: { row: ManagedMenuNode }">
            <div class="menu-row-actions">
              <el-button
                v-if="canCreate && row.menuType !== 'action'"
                :data-testid="`add-child-${row.id}`"
                text
                type="primary"
                @click="openCreate(row)"
              >{{ t('menu.addChild') }}</el-button>
              <el-button v-if="canUpdate" :data-testid="`edit-${row.id}`" text type="primary" @click="openEdit(row)">{{ t('menu.edit') }}</el-button>
              <el-button
                v-if="canUpdate"
                :data-testid="`status-${row.id}`"
                text
                type="warning"
                :disabled="row.isBuiltin"
                @click="changeStatus(row)"
              >{{ row.isEnabled === YesNo.Yes ? t('menu.disable') : t('menu.enable') }}</el-button>
              <el-button v-if="canDelete" :data-testid="`delete-${row.id}`" text type="danger" :disabled="row.isBuiltin" @click="removeNode(row)">{{ t('menu.delete') }}</el-button>
            </div>
          </template>
        </el-table-column>

        <template #empty>
          <div data-testid="menu-empty" class="menu-management__empty">{{ t('menu.empty') }}</div>
        </template>
      </el-table>
    </div>

    <el-drawer
      v-model="drawerVisible"
      :title="drawerMode === 'create' ? t('menu.form.createTitle') : t('menu.form.editTitle')"
      direction="rtl"
      size="min(520px, 100%)"
      data-testid="menu-drawer"
    >
      <el-alert
        v-if="mutationError !== ''"
        data-testid="menu-form-error"
        type="error"
        :title="mutationError"
        :closable="false"
        show-icon
      />
      <el-form label-position="top" @submit.prevent="submitForm">
        <el-form-item :label="t('menu.form.parent')">
          <el-select-v2
            v-model="parentSelection"
            data-testid="menu-form-parent"
            clearable
            :options="parentSelectOptions"
            :disabled="editingNode?.isBuiltin === true && drawerMode === 'edit'"
          />
        </el-form-item>

        <el-form-item :label="t('menu.form.menuType')">
          <el-select-v2
            :model-value="form.menuType"
            data-testid="menu-form-type"
            :options="menuTypeOptions"
            :disabled="editingNode?.isBuiltin === true && drawerMode === 'edit'"
            @update:model-value="handleFormTypeChange"
          />
        </el-form-item>

        <el-form-item :label="t('menu.form.code')">
          <el-input
            v-model="form.code"
            data-testid="menu-form-code"
            :readonly="drawerMode === 'edit'"
            :placeholder="t('menu.form.codePlaceholder')"
          />
        </el-form-item>

        <el-form-item :label="t('menu.form.i18nKey')">
          <el-select-v2 v-model="form.i18nKey" data-testid="menu-form-i18n-key" :options="titleSelectOptions" />
        </el-form-item>

        <el-form-item v-if="form.menuType === 'page'" :label="t('menu.form.path')">
          <el-input v-model="form.path" data-testid="menu-form-path" :placeholder="t('menu.form.pathPlaceholder')" />
        </el-form-item>

        <el-form-item v-if="form.menuType === 'page'" :label="t('menu.form.viewKey')">
          <el-select-v2 v-model="form.viewKey" data-testid="menu-form-view-key" :options="viewSelectOptions" />
        </el-form-item>

        <el-form-item v-if="form.menuType !== 'action'" :label="t('menu.form.icon')">
          <div class="menu-icon-picker">
            <el-button data-testid="menu-form-icon" @click="openIconSelect">
              <el-icon v-if="form.icon !== null"><component :is="menuIcons[form.icon]" /></el-icon>
              {{ form.icon ?? t('menu.form.noIcon') }}
            </el-button>
            <el-button v-if="form.icon !== null" text type="danger" @click="clearMenuIcon">{{ t('menu.form.noIcon') }}</el-button>
          </div>
        </el-form-item>

        <el-form-item :label="t('menu.form.sortOrder')">
          <el-input-number v-model="form.sortOrder" data-testid="menu-form-sort-order" :min="0" :step="10" />
        </el-form-item>

        <el-form-item v-if="drawerMode === 'create'" :label="t('menu.form.isEnabled')">
          <el-switch v-model="form.isEnabled" :active-value="YesNo.Yes" :inactive-value="YesNo.No" data-testid="menu-form-enabled" />
        </el-form-item>

        <div class="menu-form-actions">
          <el-button data-testid="menu-form-cancel" @click="closeDrawer">{{ t('menu.form.cancel') }}</el-button>
          <el-button data-testid="menu-form-submit" type="primary" :disabled="!canSubmitForm" @click="submitForm">
            {{ t('menu.form.submit') }}
          </el-button>
        </div>
      </el-form>
    </el-drawer>

    <IconSelect v-model="iconSelectVisible" :icons="menuIconOptions" :title="t('menu.form.icon')" :empty-text="t('menu.form.noIcon')" @select-icon="selectMenuIcon" />
  </section>
</template>

<style scoped>
.menu-management-page {
  min-width: 0;
}

.menu-management__toolbar-actions,
.menu-row-actions,
.menu-icon-cell {
  display: flex;
  align-items: center;
}

.menu-management__toolbar-actions,
.menu-row-actions {
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
  display: grid;
  min-width: 0;
  gap: 2px;
}

.menu-route-cell code,
.menu-icon-cell span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.menu-route-cell code {
  color: var(--admin-text);
  font-family: Consolas, 'SFMono-Regular', monospace;
  font-size: 12px;
}

.menu-route-cell span,
.menu-cell-empty {
  color: var(--admin-text-soft);
  font-size: 12px;
}

.menu-icon-cell {
  min-width: 0;
  gap: 6px;
}

.menu-row-actions {
  justify-content: flex-end;
  min-height: 32px;
}

.menu-row-actions .el-button { margin: 0; }

.menu-management__empty {
  color: var(--admin-text-soft);
}

.menu-form-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 12px;
  gap: 8px;
}

.menu-icon-picker {
  display: inline-flex;
  align-items: center;
  gap: 6px;
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
