<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CirclePlus, Delete, Edit, Switch } from '@element-plus/icons-vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  createSetting,
  deleteSetting,
  getSettings,
  updateSetting,
  updateSettingStatus,
  type SettingValueType,
  type SystemSetting,
} from '@/api/system/setting'
import { AppDialog } from '@/components/AppDialog'
import { AppSearch } from '@/components/AppSearch'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { AppTable } from '@/components/AppTable'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'

const { t } = useI18n()
const access = usePermissionStore()
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
const editing = ref<SystemSetting | null>(null)
const form = ref<{ key: string; value: string; valueType: SettingValueType; description: string }>({
  key: '',
  value: '',
  valueType: 1,
  description: '',
})

const canList = computed(() => access.hasPermission('system:setting:list'))
const canCreate = computed(() => access.hasPermission('system:setting:create'))
const canUpdate = computed(() => access.hasPermission('system:setting:update'))
const canStatus = computed(() => access.hasPermission('system:setting:status'))
const canDelete = computed(() => access.hasPermission('system:setting:delete'))
const searchModel = computed<SearchFormModel>({
  get: () => ({ keyword: keyword.value, status: status.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    status.value = value.status === YesNo.Yes || value.status === YesNo.No ? value.status : ''
  },
})
const searchFields = computed<SearchField[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('setting.key'),
    placeholder: t('setting.searchPlaceholder'),
    clearable: true,
    testId: 'setting-keyword',
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('setting.status'),
    options: [
      { label: t('setting.enabled'), value: YesNo.Yes },
      { label: t('setting.disabled'), value: YesNo.No },
    ],
    clearable: true,
    testId: 'setting-status-filter',
  },
])
const valueTypeOptions = computed(() => [
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
})
</script>

<template>
  <section class="setting-page management-page">
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
          v-if="canStatus"
          data-testid="setting-status-toggle"
          text
          :icon="Switch"
          @click="toggle(row)"
          >{{ row.isEnabled === YesNo.Yes ? t('setting.disable') : t('setting.enable') }}</el-button
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

    <AppDialog
      v-model="dialogVisible"
      :title="editing === null ? t('setting.create') : t('setting.edit')"
      width="min(560px, 94vw)"
    >
      <el-form label-position="top" @submit.prevent="save">
        <el-form-item :label="t('setting.key')">
          <el-input
            v-model="form.key"
            data-testid="setting-form-key"
            :maxlength="128"
            :disabled="editing !== null"
            :placeholder="t('setting.keyPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('setting.type')">
          <el-select-v2
            v-model="form.valueType"
            data-testid="setting-form-type"
            :options="valueTypeOptions"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item :label="t('setting.value')">
          <el-input
            v-model="form.value"
            data-testid="setting-form-value"
            type="textarea"
            :rows="4"
            :placeholder="t('setting.valuePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('setting.descriptionField')">
          <el-input
            v-model="form.description"
            data-testid="setting-form-description"
            :maxlength="512"
            :placeholder="t('setting.descriptionPlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('setting.cancel') }}</el-button>
        <el-button data-testid="setting-save" type="primary" :loading="submitting" @click="save">
          {{ t('setting.save') }}
        </el-button>
      </template>
    </AppDialog>
  </section>
</template>
