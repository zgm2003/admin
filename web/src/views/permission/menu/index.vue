<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import {
  createMenu,
  deleteMenu,
  getMenus,
  updateMenu,
  updateMenuStatus,
  rebuildAccessCache,
} from '@/api/permission/menu'
import type { ManagedMenuNode, ManagedMenuType, MenuPlatformOption } from '@/api/permission/menu'
import { YesNo } from '@/enums/yes-no'
import { usePermissionStore } from '@/store/permission'
import MenuFormDialog from './components/MenuFormDialog/index.vue'
import MenuTreeTable from './components/MenuTreeTable/index.vue'
import type { MenuFormState } from './components/types'
import {
  changeMenuFormType,
  createMenuForm,
  createMenuInput,
  editMenuForm,
  isMenuFormSubmittable,
  menuCodeError,
  updateMenuInput,
} from './menu-form'
import { menuParentOptions } from './menu-tree'

const { t } = useI18n()
const access = usePermissionStore()

const menus = ref<ManagedMenuNode[]>([])
const platforms = ref<MenuPlatformOption[]>([])
const activePlatformID = ref<number | null>(null)
const loading = ref(false)
const rebuildingAccessCache = ref(false)
const loadError = ref('')
const mutationError = ref('')
const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingID = ref<number | null>(null)

const form = ref<MenuFormState>(createMenuForm())

const canCreate = computed(() => access.hasPermission('permission:menu:create'))
const canUpdate = computed(() => access.hasPermission('permission:menu:update'))
const canDelete = computed(() => access.hasPermission('permission:menu:delete'))
const canRebuildAccessCache = computed(() =>
  access.hasPermission('permission:access-cache:rebuild'),
)
const activePlatform = computed(() => {
  if (activePlatformID.value === null) return null
  return platforms.value.find((platform) => platform.id === activePlatformID.value) ?? null
})

function publicErrorMessage(error: unknown): string {
  return error instanceof Error && error.message !== '' ? error.message : t('menu.loadFailed')
}

const editingProtected = computed(
  () => dialogMode.value === 'edit' && form.value.isProtected === YesNo.Yes,
)

const parentOptions = computed(() =>
  menuParentOptions(menus.value, form.value.menuType, editingID.value),
)
const canSubmitForm = computed(() => isMenuFormSubmittable(form.value))

const rootParentValue = '__root__' as const

const parentSelectOptions = computed<
  Array<{ label: string; value: number | typeof rootParentValue }>
>(() => [
  { label: t('menu.form.root'), value: rootParentValue },
  ...parentOptions.value.map((node) => ({
    label: parentLabel(node),
    value: node.id,
  })),
])
const menuTypeOptions = computed<Array<{ label: string; value: ManagedMenuType }>>(() => [
  { label: t('menu.type.directory'), value: 'directory' },
  { label: t('menu.type.page'), value: 'page' },
  { label: t('menu.type.action'), value: 'action' },
])
const parentSelection = computed<number | typeof rootParentValue>({
  get: () => form.value.parentId ?? rootParentValue,
  set: (value) => {
    form.value.parentId = value === rootParentValue ? null : value
  },
})

function parentLabel(node: ManagedMenuNode): string {
  return `${node.name} (${node.code})`
}

function handleFormTypeChange(nextType: ManagedMenuType): void {
  const validParentIDs = new Set(
    menuParentOptions(menus.value, nextType, editingID.value).map((node) => node.id),
  )
  form.value = changeMenuFormType(form.value, nextType, validParentIDs)
}

function openCreate(parent: ManagedMenuNode | null = null): void {
  dialogMode.value = 'create'
  editingID.value = null
  mutationError.value = ''
  form.value = createMenuForm(parent)
  dialogVisible.value = true
}

function openEdit(node: ManagedMenuNode): void {
  dialogMode.value = 'edit'
  editingID.value = node.id
  mutationError.value = ''
  form.value = editMenuForm(node)
  dialogVisible.value = true
}

function closeDialog(): void {
  dialogVisible.value = false
  editingID.value = null
}

async function submitForm(): Promise<void> {
  const codeError = menuCodeError(form.value)
  if (codeError !== null) {
    mutationError.value = t(
      codeError === 'page-code-suffix'
        ? 'menu.form.pageCodeSuffixError'
        : 'menu.form.actionCodeSuffixError',
    )
    return
  }
  if (!canSubmitForm.value) return
  mutationError.value = ''
  try {
    if (dialogMode.value === 'create') {
      if (activePlatformID.value === null) {
        mutationError.value = t('menu.platform.unavailable')
        return
      }
      await createMenu(createMenuInput(activePlatformID.value, form.value))
      await reloadMenus()
      closeDialog()
      notifyMutation('menu.success.created')
      return
    }
    if (editingID.value === null) return
    await updateMenu(editingID.value, updateMenuInput(form.value))
    await reloadMenus()
    closeDialog()
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
    await reloadMenus()
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
    await reloadMenus()
    notifyMutation('menu.success.deleted')
  } catch (error: unknown) {
    if (error === 'cancel' || error === 'close') return
    mutationError.value = publicErrorMessage(error)
  }
}

async function rebuildAccessCacheNow(): Promise<void> {
  try {
    await ElMessageBox.confirm(
      t('menu.confirm.rebuildAccessCacheMessage'),
      t('menu.confirm.rebuildAccessCacheTitle'),
      {
        confirmButtonText: t('menu.confirm.confirm'),
        cancelButtonText: t('menu.confirm.cancel'),
        type: 'warning',
      },
    )
    rebuildingAccessCache.value = true
    const result = await rebuildAccessCache()
    await reloadMenus()
    ElNotification.success({
      title: t('menu.success.accessCacheRebuilt'),
      message: t('menu.success.accessCacheRebuiltCount', { count: result.rebuiltUsers }),
    })
  } catch (error: unknown) {
    if (error === 'cancel' || error === 'close') return
    mutationError.value = publicErrorMessage(error)
  } finally {
    rebuildingAccessCache.value = false
  }
}

async function loadMenus(platformID?: number): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const result =
      platformID === undefined ? await getMenus() : await getMenus({ platformId: platformID })
    const selectedPlatform =
      platformID === undefined
        ? (result.platforms.find((platform) => platform.code === 'admin') ?? result.platforms[0])
        : result.platforms.find((platform) => platform.id === platformID)
    if (selectedPlatform === undefined) {
      throw new Error(t('menu.platform.unavailable'))
    }
    platforms.value = result.platforms
    activePlatformID.value = selectedPlatform.id
    menus.value = result.menuTree
  } catch (error: unknown) {
    loadError.value = publicErrorMessage(error)
  } finally {
    loading.value = false
  }
}

async function reloadMenus(): Promise<void> {
  if (activePlatformID.value === null) {
    await loadMenus()
    return
  }
  await loadMenus(activePlatformID.value)
}

async function switchPlatform(value: string | number): Promise<void> {
  const platformID = typeof value === 'number' ? value : Number(value)
  if (!Number.isInteger(platformID) || platformID < 1) {
    loadError.value = t('menu.platform.unavailable')
    return
  }
  menus.value = []
  await loadMenus(platformID)
}

onMounted(() => loadMenus())
</script>

<template>
  <section class="menu-management-page management-page" :aria-label="t('menu.title')">
    <el-tabs
      v-if="platforms.length > 0"
      v-model="activePlatformID"
      data-testid="menu-platform-tabs"
      class="menu-platform-tabs"
      @tab-change="switchPlatform"
    >
      <el-tab-pane v-for="platform in platforms" :key="platform.id" :name="platform.id">
        <template #label>
          <span class="menu-platform-tab">
            <span>{{ platform.name }}</span>
            <code>{{ platform.code }}</code>
            <el-tag v-if="platform.isEnabled === YesNo.No" size="small" type="info" effect="plain">
              {{ t('menu.platform.disabled') }}
            </el-tag>
          </span>
        </template>
      </el-tab-pane>
    </el-tabs>
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

      <MenuTreeTable
        :menus="menus"
        :loading="loading"
        :show-table="loadError === ''"
        :can-create="canCreate"
        :can-update="canUpdate"
        :can-delete="canDelete"
        :can-rebuild-access-cache="canRebuildAccessCache"
        :active-platform-available="activePlatform !== null"
        :rebuilding-access-cache="rebuildingAccessCache"
        @create-root="openCreate()"
        @create-child="openCreate"
        @edit="openEdit"
        @status="changeStatus"
        @delete="removeNode"
        @refresh="reloadMenus"
        @rebuild-access-cache="rebuildAccessCacheNow"
      />
    </div>

    <MenuFormDialog
      v-model="dialogVisible"
      v-model:form="form"
      v-model:parent-selection="parentSelection"
      :dialog-mode="dialogMode"
      :mutation-error="mutationError"
      :editing-protected="editingProtected"
      :active-platform="activePlatform"
      :can-submit-form="canSubmitForm"
      :parent-select-options="parentSelectOptions"
      :menu-type-options="menuTypeOptions"
      :handle-form-type-change="handleFormTypeChange"
      @close="closeDialog"
      @save="submitForm"
    />
  </section>
</template>

<style scoped>
.menu-management-page {
  min-width: 0;
}

.menu-platform-tabs {
  min-width: 0;
  margin-bottom: 8px;
}

.menu-platform-tab {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.menu-platform-tab code {
  color: var(--admin-text-soft);
  font-family: Consolas, 'SFMono-Regular', monospace;
  font-size: 12px;
}

.menu-management__content {
  min-width: 0;
  padding: 0;
}
</style>
