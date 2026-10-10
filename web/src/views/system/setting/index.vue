<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { CirclePlus, Delete, Edit, Switch } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { useI18n } from 'vue-i18n'

import {
  createSetting,
  deleteSetting,
  getLegalDocument,
  getSettings,
  updateSetting,
  updateLegalDocument,
  updateSettingStatus,
  type SettingValueType,
  type SystemSetting,
  type LegalDocumentKind,
} from '@/api/system/setting'
import { getSettingOptions, type SettingOptions } from '@/api/system/settingOptions'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'
import { useBrandStore } from '@/store/brand'
import SettingDialog from './components/SettingDialog/index.vue'
import LegalSettingsPanel from './components/LegalSettingsPanel/index.vue'

const { t } = useI18n()
const access = usePermissionStore()
const brand = useBrandStore()
const {
  options: formOptions,
  loading: optionsLoading,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<SettingOptions | null>(getSettingOptions, () => null)
const activeTab = ref<'advanced' | 'legal'>('advanced')
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
const submitError = ref('')
const legalLoading = ref(false)
const legalSaving = ref(false)
const legalError = ref('')
const legalDocuments = ref<Record<LegalDocumentKind, string>>({
  userAgreement: '',
  privacyPolicy: '',
})
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
const canSubmit = computed(
  () =>
    formOptions.value !== null &&
    !optionsLoading.value &&
    optionsError.value === '' &&
    (editing.value === null ? canCreate.value : canUpdate.value),
)
let pageMounted = true
let loadSequence = 0
watch(canList, (allowed) => {
  loadSequence++
  rows.value = []
  total.value = 0
  loading.value = false
  loadError.value = ''
  if (allowed) void load()
})
onBeforeUnmount(() => {
  pageMounted = false
  loadSequence++
})
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
    placeholder: t('setting.allStatuses'),
    options: [
      { label: t('setting.enabled'), value: YesNo.Yes },
      { label: t('setting.disabled'), value: YesNo.No },
    ],
    clearable: true,
    testId: 'setting-status-filter',
  },
])
const valueTypeOptions = computed(() => formOptions.value?.valueTypes ?? [])

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
  const sequence = ++loadSequence
  const current = () => pageMounted && canList.value && sequence === loadSequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await getSettings({
      page: page.value,
      pageSize: pageSize.value,
      ...(keyword.value.trim() ? { keyword: keyword.value.trim() } : {}),
      ...(status.value === '' ? {} : { isEnabled: status.value }),
    })
    if (current()) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (current()) {
      rows.value = []
      total.value = 0
      loadError.value = t('setting.loadFailed')
    }
  } finally {
    if (current()) loading.value = false
  }
}
async function refreshBrand(): Promise<void> {
  brand.reset()
  try {
    await brand.load()
  } catch {
    /* The brand store records failure; request.ts owns notifications. */
  }
}
async function loadLegalDocuments(): Promise<void> {
  legalLoading.value = true
  legalError.value = ''
  try {
    const [userAgreement, privacyPolicy] = await Promise.all([
      getLegalDocument('userAgreement'),
      getLegalDocument('privacyPolicy'),
    ])
    legalDocuments.value = {
      userAgreement: userAgreement.contentHtml,
      privacyPolicy: privacyPolicy.contentHtml,
    }
  } catch {
    legalError.value = t('setting.legalLoadFailed')
  } finally {
    legalLoading.value = false
  }
}
async function saveLegalDocument(kind: LegalDocumentKind): Promise<void> {
  if (!canUpdate.value || legalSaving.value) return
  legalSaving.value = true
  legalError.value = ''
  try {
    await updateLegalDocument(kind, legalDocuments.value[kind])
    ElNotification.success({ title: t('setting.saved') })
  } catch {
    legalError.value = t('setting.legalSaveFailed')
  } finally {
    legalSaving.value = false
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
  if (
    !canCreate.value ||
    submitting.value ||
    formOptions.value === null ||
    optionsLoading.value ||
    optionsError.value !== ''
  )
    return
  submitError.value = ''
  editing.value = null
  form.value = {
    key: '',
    value: '',
    valueType: formOptions.value.defaultValueType,
    description: '',
  }
  dialogVisible.value = true
}
function openEdit(row: SystemSetting): void {
  if (
    !canUpdate.value ||
    submitting.value ||
    formOptions.value === null ||
    optionsLoading.value ||
    optionsError.value !== ''
  )
    return
  submitError.value = ''
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
    !canSubmit.value ||
    submitting.value ||
    (editing.value === null && form.value.key.trim() === '')
  )
    return
  submitting.value = true
  submitError.value = ''
  const target = editing.value
  const payload = { ...form.value }
  try {
    if (
      target !== null &&
      target.presentation.warnOnDecrease &&
      Number(payload.value) < Number(target.value)
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
    if (!pageMounted || !dialogVisible.value || !canSubmit.value) return
    let refresh = target?.presentation.refreshBrand === true
    if (target === null) {
      const created = await createSetting(payload)
      refresh = created.presentation.refreshBrand
    } else
      await updateSetting(target.key, {
        value: payload.value,
        valueType: payload.valueType,
        description: payload.description,
      })
    dialogVisible.value = false
    if (refresh) await refreshBrand()
    await load()
    ElNotification.success({ title: t('setting.saved') })
  } catch {
    submitError.value = t('setting.saveFailed')
  } finally {
    submitting.value = false
  }
}
async function toggle(row: SystemSetting): Promise<void> {
  if (!canStatus.value || !row.presentation.actions.status) return
  await updateSettingStatus(row.key, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
  await load()
}
async function remove(row: SystemSetting): Promise<void> {
  if (!row.presentation.actions.delete || !canDelete.value) return
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
  void loadLegalDocuments()
})
</script>

<template>
  <AppPage class="setting-page">
    <el-alert
      v-if="optionsError"
      data-testid="setting-options-error"
      :title="t('setting.loadFailed')"
      type="error"
      :closable="false"
      show-icon
    >
      <el-button data-testid="setting-options-retry" @click="reloadOptions">{{
        t('appTable.refresh')
      }}</el-button>
    </el-alert>
    <el-tabs v-model="activeTab" class="setting-page__tabs">
      <el-tab-pane name="advanced" :label="t('setting.advancedTitle')">
        <el-alert
          v-if="!canList"
          :title="t('setting.readDenied')"
          type="info"
          :closable="false"
          show-icon
        />
        <el-button
          v-if="!canList && canCreate"
          data-testid="setting-create"
          :disabled="formOptions === null || optionsLoading"
          type="primary"
          :icon="CirclePlus"
          @click="openCreate"
          >{{ t('setting.create') }}</el-button
        >
        <AppSearch
          v-if="canList"
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
          v-if="canList"
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
              :disabled="formOptions === null || optionsLoading"
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
              :disabled="formOptions === null || optionsLoading"
              text
              type="primary"
              :icon="Edit"
              @click="openEdit(row)"
              >{{ t('setting.edit') }}</el-button
            >
            <el-button
              v-if="canStatus && row.presentation.actions.status"
              data-testid="setting-status-toggle"
              text
              :icon="Switch"
              @click="toggle(row)"
              >{{
                row.isEnabled === YesNo.Yes ? t('setting.disable') : t('setting.enable')
              }}</el-button
            >
            <el-button
              v-if="canDelete && row.presentation.actions.delete"
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
      <el-tab-pane name="legal" :label="t('setting.legalTitle')">
        <LegalSettingsPanel
          v-model:documents="legalDocuments"
          :loading="legalLoading"
          :saving="legalSaving"
          :error="legalError"
          :can-update="canUpdate"
          @save="saveLegalDocument"
        />
      </el-tab-pane>
    </el-tabs>

    <SettingDialog
      v-if="formOptions !== null"
      :default-presentation="formOptions.defaultPresentation"
      v-model="dialogVisible"
      v-model:form="form"
      :editing="editing"
      :submitting="submitting"
      :error="submitError"
      :can-save="canSubmit"
      :can-upload="canUpload"
      :value-type-options="valueTypeOptions"
      @save="save"
    />
  </AppPage>
</template>

<style scoped lang="scss">
.setting-page {
  height: 100%;
  min-height: 0;

  &__tabs {
    display: flex;
    min-height: 0;
    flex: 1 1 auto;
    flex-direction: column;
  }

  &__tabs :deep(.el-tabs__header) {
    margin-bottom: 0;
  }

  &__tabs :deep(.el-tabs__nav-wrap::after) {
    height: 1px;
    background: var(--el-border-color-lighter);
  }

  &__tabs :deep(.el-tabs__item) {
    height: 44px;
    padding: 0 22px;
    font-size: 14px;
  }

  &__tabs :deep(.el-tabs__item.is-active) {
    font-weight: 600;
  }
}

/* 内容区自己滚动，页签保持可见；透明上边框不会像 padding 那样随滚动消失。 */
.setting-page {
  &__tabs :deep(.el-tabs__content) {
    min-height: 0;
    flex: 1 1 auto;
    overflow: auto;
    border-top: 24px solid transparent;
  }
}

/* 面板按内容区高度布局，让内部编辑器自己滚动而不是整体滚页。 */
.setting-page {
  &__tabs :deep(.el-tab-pane) {
    height: 100%;
  }
}
</style>
