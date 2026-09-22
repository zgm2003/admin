<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CirclePlus, Delete, Edit, Switch } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { useI18n } from 'vue-i18n'

import {
  createSetting,
  deleteSetting,
  getSettings,
  updateSetting,
  updateBrandSettings,
  updateSettingStatus,
  isRetentionSettingKey,
  type SettingValueType,
  type SystemSetting,
  type BrandSettings,
} from '@/api/system/setting'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'
import { useBrandStore } from '@/store/brand'
import SettingDialog from './components/SettingDialog/index.vue'
import BrandSettingsPanel from './components/BrandSettingsPanel/index.vue'

const { t } = useI18n()
const access = usePermissionStore()
const brand = useBrandStore()
const activeTab = ref<'brand' | 'advanced'>('brand')
const rows = ref<SystemSetting[]>([])
const loading = ref(false)
const loadError = ref('')
const keyword = ref('')
const status = ref<'' | YesNo>('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const dialogVisible = ref(false)
const submitting = ref(false)
const brandLoading = ref(false)
const brandSaving = ref(false)
const brandError = ref('')
const brandForm = ref<BrandSettings>({ titleZhCN: '', titleEnUS: '', defaultAvatar: '' })
const editing = ref<SystemSetting | null>(null)
const form = ref<{ key: string; value: string; valueType: SettingValueType; description: string }>({
  key: '',
  value: '',
  valueType: 1,
  description: '',
})
interface SettingSearchModel {
  keyword: string
  status: '' | YesNo
}

const canList = computed(() => access.hasPermission('system:setting:list'))
const canCreate = computed(() => access.hasPermission('system:setting:create'))
const canUpdate = computed(() => access.hasPermission('system:setting:update'))
const canStatus = computed(() => access.hasPermission('system:setting:status'))
const canDelete = computed(() => access.hasPermission('system:setting:delete'))
const canUpload = computed(() => access.hasPermission('storage:object:upload'))
const searchModel = computed<SearchFormModel<SettingSearchModel>>({
  get: () => ({ keyword: keyword.value, status: status.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    status.value = value.status === YesNo.Yes || value.status === YesNo.No ? value.status : ''
  },
})
const searchFields = computed<SearchField<SettingSearchModel>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    resetValue: '',
    label: t('setting.key'),
    placeholder: t('setting.searchPlaceholder'),
    clearable: true,
    testId: 'setting-keyword',
  },
  {
    key: 'status',
    type: 'select-v2',
    resetValue: '',
    label: t('setting.status'),
    options: [
      { label: t('setting.enabled'), value: YesNo.Yes },
      { label: t('setting.disabled'), value: YesNo.No },
    ],
    clearable: true,
    testId: 'setting-status-filter',
  },
])
const valueTypeOptions = computed<Array<{ label: string; value: SettingValueType }>>(() => [
  { label: t('setting.typeString'), value: 1 },
  { label: t('setting.typeNumber'), value: 2 },
  { label: t('setting.typeBoolean'), value: 3 },
  { label: t('setting.typeJson'), value: 4 },
])

const state = computed<'loading' | 'error' | 'empty' | 'success'>(() =>
  loading.value
    ? 'loading'
    : loadError.value !== ''
      ? 'error'
      : rows.value.length === 0
        ? 'empty'
        : 'success',
)
const columns = computed<TableColumn<SystemSetting>[]>(() => [
  { prop: 'key', label: t('setting.key'), minWidth: 240 },
  { prop: 'value', label: t('setting.value'), minWidth: 180, overflowTooltip: true },
  { key: 'type', prop: 'valueType', label: t('setting.type'), width: 120 },
  {
    prop: 'description',
    label: t('setting.descriptionField'),
    minWidth: 220,
    overflowTooltip: true,
  },
  { key: 'status', prop: 'isEnabled', label: t('setting.status'), width: 110 },
  { key: 'actions', prop: 'key', label: t('setting.actions'), width: 230, fixed: 'right' },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))

async function load(): Promise<void> {
  if (!canList.value) return
  loading.value = true
  loadError.value = ''
  try {
    const result = await getSettings({
      page: page.value,
      pageSize: pageSize.value,
      ...(keyword.value.trim() ? { keyword: keyword.value.trim() } : {}),
      ...(status.value === '' ? {} : { isEnabled: status.value }),
    })
    rows.value = result.list
    total.value = result.total
  } catch {
    loadError.value = t('setting.loadFailed')
  } finally {
    loading.value = false
  }
}
async function loadBrand(): Promise<void> {
  brandLoading.value = true
  brandError.value = ''
  try {
    await brand.load()
    brandForm.value = { ...brand.settings }
  } catch {
    brandError.value = t('setting.brandLoadFailed')
  } finally {
    brandLoading.value = false
  }
}
async function saveBrand(): Promise<void> {
  if (!canUpdate.value || brandSaving.value) return
  const next = {
    titleZhCN: brandForm.value.titleZhCN.trim(),
    titleEnUS: brandForm.value.titleEnUS.trim(),
    defaultAvatar: brandForm.value.defaultAvatar.trim(),
  }
  if (next.titleZhCN === '' || next.titleEnUS === '') {
    brandError.value = t('setting.brandTitleRequired')
    return
  }
  brandSaving.value = true
  brandError.value = ''
  try {
    await updateBrandSettings(next)
    brand.apply(next)
    brandForm.value = next
    ElNotification.success({ title: t('setting.saved') })
  } catch {
    brandError.value = t('setting.brandSaveFailed')
  } finally {
    brandSaving.value = false
  }
}
function search(): void {
  page.value = 1
  void load()
}
function reset(): void {
  keyword.value = ''
  status.value = ''
  page.value = 1
  void load()
}
function updatePagination(next: TablePaginationState): void {
  page.value = next.currentPage
  pageSize.value = next.pageSize
  void load()
}
function openCreate(): void {
  editing.value = null
  form.value = { key: '', value: '', valueType: 1, description: '' }
  dialogVisible.value = true
}
function openEdit(row: SystemSetting): void {
  editing.value = row
  form.value = {
    key: row.key,
    value: row.value,
    valueType: row.valueType,
    description: row.description,
  }
  dialogVisible.value = true
}
async function save(): Promise<void> {
  if (
    submitting.value ||
    form.value.value.trim() === '' ||
    (editing.value === null && form.value.key.trim() === '')
  )
    return
  if (
    editing.value !== null &&
    isRetentionSettingKey(editing.value.key) &&
    Number(form.value.value) < Number(editing.value.value)
  ) {
    try {
      await ElMessageBox.confirm(
        t('setting.retentionDecreaseConfirm'),
        t('setting.retentionDecreaseTitle'),
        { type: 'warning' },
      )
    } catch (error: unknown) {
      if (error === 'cancel' || error === 'close') return
      throw error
    }
  }
  submitting.value = true
  try {
    if (editing.value === null) await createSetting(form.value)
    else
      await updateSetting(editing.value.key, {
        value: form.value.value,
        valueType: form.value.valueType,
        description: form.value.description,
      })
    dialogVisible.value = false
    await load()
    ElNotification.success({ title: t('setting.saved') })
  } finally {
    submitting.value = false
  }
}
async function toggle(row: SystemSetting): Promise<void> {
  if (!canStatus.value) return
  await updateSettingStatus(row.key, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
  await load()
}
async function remove(row: SystemSetting): Promise<void> {
  if (row.isBuiltin === YesNo.Yes || !canDelete.value) return
  await ElMessageBox.confirm(t('setting.deleteConfirm'), t('setting.deleteTitle'), {
    type: 'warning',
  })
  await deleteSetting(row.key)
  await load()
}
function typeLabel(value: SettingValueType): string {
  return valueTypeOptions.value.find((item) => item.value === value)?.label ?? String(value)
}

onMounted(() => {
  void load()
  void loadBrand()
})
</script>

<template>
  <AppPage class="setting-page">
    <el-tabs v-model="activeTab" class="setting-page__tabs">
      <el-tab-pane name="brand" :label="t('setting.brandTitle')">
        <BrandSettingsPanel
          v-model:form="brandForm"
          :loading="brandLoading"
          :saving="brandSaving"
          :error="brandError"
          :can-update="canUpdate"
          :can-upload="canUpload"
          @save="saveBrand"
        />
      </el-tab-pane>
      <el-tab-pane name="advanced" :label="t('setting.advancedTitle')">
        <AppSearch
          v-model="searchModel"
          class="management-page__filters"
          :fields="searchFields"
          :query-label="t('search.query')"
          :reset-label="t('search.reset')"
          query-test-id="setting-search"
          reset-test-id="setting-reset"
          @query="search"
          @reset="reset"
        />

        <AppTable
          :columns="columns"
          :data="rows"
          :loading="loading"
          :result-state="state"
          :status-message="loadError"
          row-key="id"
          :pagination="pagination"
          :aria-label="t('setting.title')"
          :refresh-label="t('appTable.refresh')"
          @refresh="load"
          @update:pagination="updatePagination"
        >
          <template #toolbar-left>
            <el-button
              v-if="canCreate"
              data-testid="setting-create"
              type="primary"
              :icon="CirclePlus"
              @click="openCreate"
              >{{ t('setting.create') }}</el-button
            >
          </template>
          <template #cell-type="{ row }: { row: SystemSetting }">{{
            typeLabel(row.valueType)
          }}</template>
          <template #cell-status="{ row }: { row: SystemSetting }">
            <el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'">
              {{ row.isEnabled === YesNo.Yes ? t('setting.enabled') : t('setting.disabled') }}
            </el-tag>
          </template>
          <template #cell-actions="{ row }: { row: SystemSetting }">
            <el-button
              v-if="canUpdate"
              data-testid="setting-update"
              text
              type="primary"
              :icon="Edit"
              @click="openEdit(row)"
              >{{ t('setting.edit') }}</el-button
            >
            <el-button
              v-if="canStatus && !isRetentionSettingKey(row.key)"
              data-testid="setting-status-toggle"
              text
              :icon="Switch"
              @click="toggle(row)"
              >{{
                row.isEnabled === YesNo.Yes ? t('setting.disable') : t('setting.enable')
              }}</el-button
            >
            <el-button
              v-if="canDelete && row.isBuiltin === YesNo.No"
              data-testid="setting-delete"
              text
              type="danger"
              :icon="Delete"
              @click="remove(row)"
              >{{ t('setting.delete') }}</el-button
            >
          </template>
          <template #empty
            ><el-empty data-testid="setting-empty" :description="t('setting.empty')"
          /></template>
        </AppTable>
      </el-tab-pane>
    </el-tabs>

    <SettingDialog
      v-model="dialogVisible"
      v-model:form="form"
      :editing="editing"
      :submitting="submitting"
      :value-type-options="valueTypeOptions"
      @save="save"
    />
  </AppPage>
</template>

<style scoped>
.setting-page__tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
}

.setting-page__tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--el-border-color-lighter);
}

.setting-page__tabs :deep(.el-tabs__item) {
  height: 44px;
  padding: 0 22px;
  font-size: 14px;
}

.setting-page__tabs :deep(.el-tabs__item.is-active) {
  font-weight: 600;
}

.setting-page__tabs :deep(.el-tabs__content) {
  padding-top: 24px;
  overflow: visible;
}
</style>
