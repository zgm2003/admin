<script setup lang="ts">
import { AppPage } from '@/components/AppPage'
import { computed, onMounted, ref } from 'vue'
import { CirclePlus, Delete, Edit, Switch } from '@element-plus/icons-vue'
import { ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  createDictionary,
  createDictionaryItem,
  deleteDictionary,
  deleteDictionaryItem,
  getDictionaries,
  getDictionary,
  updateDictionary,
  updateDictionaryItem,
  updateDictionaryItemStatus,
  updateDictionaryStatus,
} from '@/api/system/dictionary'
import type { Dictionary, DictionaryItem } from '@/api/system/dictionary'
import { AppTable } from '@/components/AppTable'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'
import { useSystemDictionaryStore } from '@/store/systemDictionary'
import { AppSearch } from '@/components/AppSearch'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import DictionaryDetailDialog from './components/DictionaryDetailDialog/index.vue'
import DictionaryFormDialog from './components/DictionaryFormDialog/index.vue'
import DictionaryItemDialog from './components/DictionaryItemDialog/index.vue'

const { t } = useI18n()
const access = usePermissionStore()
const dictionaries = useSystemDictionaryStore()
const rows = ref<Dictionary[]>([])
const total = ref(0)
const query = ref<{ page: number; pageSize: number; keyword?: string; isEnabled?: YesNo }>({
  page: 1,
  pageSize: 20,
})
const keyword = ref('')
const statusFilter = ref<'' | YesNo>('')
const loading = ref(false)
const loadError = ref('')
const dictionaryVisible = ref(false)
const editingDictionary = ref<Dictionary | null>(null)
const dictionaryForm = ref({ code: '', nameZh: '', nameEn: '', description: '' })
const detailVisible = ref(false)
const selectedDictionary = ref<Dictionary | null>(null)
const items = ref<DictionaryItem[]>([])
const itemVisible = ref(false)
const editingItem = ref<DictionaryItem | null>(null)
const itemForm = ref({ value: '', labelZh: '', labelEn: '', sort: 0 })
const submitting = ref(false)

const canDetail = computed(() => access.hasPermission('system:dictionary:detail'))
const canList = computed(() => access.hasPermission('system:dictionary:list'))
const canCreate = computed(() => access.hasPermission('system:dictionary:create'))
const canUpdate = computed(() => access.hasPermission('system:dictionary:update'))
const canStatus = computed(() => access.hasPermission('system:dictionary:status'))
const canDelete = computed(() => access.hasPermission('system:dictionary:delete'))
const searchModel = computed<SearchFormModel>({
  get: () => ({ keyword: keyword.value, status: statusFilter.value }),
  set: (value) => {
    keyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    statusFilter.value = value.status === YesNo.Yes || value.status === YesNo.No ? value.status : ''
  },
})
const searchFields = computed<SearchField[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('dictionary.code'),
    placeholder: t('dictionary.searchPlaceholder'),
    clearable: true,
    testId: 'dictionary-keyword',
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('dictionary.status'),
    options: [
      { label: t('dictionary.enabled'), value: YesNo.Yes },
      { label: t('dictionary.disabled'), value: YesNo.No },
    ],
    clearable: true,
    testId: 'dictionary-status-filter',
  },
])
const columns = computed<TableColumn<Dictionary>[]>(() => [
  { prop: 'code', label: t('dictionary.code'), minWidth: 180 },
  { prop: 'nameZh', label: t('dictionary.nameZh'), minWidth: 140 },
  { prop: 'nameEn', label: t('dictionary.nameEn'), minWidth: 140 },
  { prop: 'itemCount', label: t('dictionary.itemCount'), width: 100 },
  { key: 'status', prop: 'id', label: t('dictionary.status'), width: 100 },
  { key: 'actions', prop: 'id', label: t('dictionary.actions'), width: 230 },
])
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const listState = computed<'loading' | 'error' | 'empty' | 'success'>(() => {
  if (loading.value) return 'loading'
  if (loadError.value !== '') return 'error'
  return rows.value.length === 0 ? 'empty' : 'success'
})

async function load(): Promise<void> {
  if (!canList.value) return
  loading.value = true
  loadError.value = ''
  try {
    const result = await getDictionaries(query.value)
    rows.value = result.list
    total.value = result.total
  } catch {
    loadError.value = t('dictionary.loadFailed')
  } finally {
    loading.value = false
  }
}
function openCreate(): void {
  editingDictionary.value = null
  dictionaryForm.value = { code: '', nameZh: '', nameEn: '', description: '' }
  dictionaryVisible.value = true
}
function search(): void {
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(keyword.value.trim() === '' ? {} : { keyword: keyword.value.trim() }),
    ...(statusFilter.value === '' ? {} : { isEnabled: statusFilter.value }),
  }
  void load()
}
function reset(): void {
  keyword.value = ''
  statusFilter.value = ''
  query.value = { page: 1, pageSize: query.value.pageSize }
  void load()
}
function openEdit(row: Dictionary): void {
  editingDictionary.value = row
  dictionaryForm.value = {
    code: row.code,
    nameZh: row.nameZh,
    nameEn: row.nameEn,
    description: row.description,
  }
  dictionaryVisible.value = true
}
async function submitDictionary(): Promise<void> {
  if (
    submitting.value ||
    dictionaryForm.value.code.trim() === '' ||
    dictionaryForm.value.nameZh.trim() === '' ||
    dictionaryForm.value.nameEn.trim() === ''
  )
    return
  submitting.value = true
  try {
    if (editingDictionary.value === null) await createDictionary(dictionaryForm.value)
    else await updateDictionary(editingDictionary.value.id, dictionaryForm.value)
    dictionaries.reset()
    dictionaryVisible.value = false
    await load()
    ElNotification.success({ title: t('dictionary.saved') })
  } finally {
    submitting.value = false
  }
}
async function openDetail(row: Dictionary): Promise<void> {
  if (!canDetail.value) return
  const detail = await getDictionary(row.id)
  selectedDictionary.value = detail.dictionary
  items.value = detail.items
  detailVisible.value = true
}
async function reloadDetail(): Promise<void> {
  if (selectedDictionary.value === null) return
  const detail = await getDictionary(selectedDictionary.value.id)
  selectedDictionary.value = detail.dictionary
  items.value = detail.items
  await load()
}
function openCreateItem(): void {
  editingItem.value = null
  itemForm.value = { value: '', labelZh: '', labelEn: '', sort: 0 }
  itemVisible.value = true
}
function openEditItem(item: DictionaryItem): void {
  editingItem.value = item
  itemForm.value = {
    value: item.value,
    labelZh: item.labelZh,
    labelEn: item.labelEn,
    sort: item.sort,
  }
  itemVisible.value = true
}
async function submitItem(): Promise<void> {
  if (
    selectedDictionary.value === null ||
    submitting.value ||
    itemForm.value.value.trim() === '' ||
    itemForm.value.labelZh.trim() === '' ||
    itemForm.value.labelEn.trim() === ''
  )
    return
  submitting.value = true
  try {
    if (editingItem.value === null)
      await createDictionaryItem(selectedDictionary.value.id, itemForm.value)
    else
      await updateDictionaryItem(selectedDictionary.value.id, editingItem.value.id, itemForm.value)
    dictionaries.reset()
    itemVisible.value = false
    await reloadDetail()
  } finally {
    submitting.value = false
  }
}
async function toggleDictionary(row: Dictionary): Promise<void> {
  await updateDictionaryStatus(row.id, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
  dictionaries.reset()
  await load()
}
async function toggleItem(item: DictionaryItem): Promise<void> {
  if (selectedDictionary.value === null) return
  await updateDictionaryItemStatus(
    selectedDictionary.value.id,
    item.id,
    item.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes,
  )
  dictionaries.reset()
  await reloadDetail()
}
async function removeDictionary(row: Dictionary): Promise<void> {
  if (row.isBuiltin === YesNo.Yes) return
  await ElMessageBox.confirm(t('dictionary.deleteConfirm'), t('dictionary.deleteTitle'), {
    type: 'warning',
  })
  await deleteDictionary(row.id)
  dictionaries.reset()
  await load()
}
async function removeItem(item: DictionaryItem): Promise<void> {
  if (selectedDictionary.value === null || item.isBuiltin === YesNo.Yes) return
  await ElMessageBox.confirm(t('dictionary.itemDeleteConfirm'), t('dictionary.deleteTitle'), {
    type: 'warning',
  })
  await deleteDictionaryItem(selectedDictionary.value.id, item.id)
  dictionaries.reset()
  await reloadDetail()
}
function updatePagination(value: TablePaginationState): void {
  query.value = {
    ...query.value,
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}
onMounted(() => void load())
</script>

<template>
  <AppPage class="dictionary-page">
    <AppSearch
      v-model="searchModel"
      class="management-page__filters"
      :fields="searchFields"
      :query-label="t('search.query')"
      :reset-label="t('search.reset')"
      query-test-id="dictionary-search"
      reset-test-id="dictionary-reset"
      @query="search"
      @reset="reset"
    />
    <AppTable
      :data="rows"
      :columns="columns"
      :loading="loading"
      :result-state="listState"
      :status-message="loadError"
      row-key="id"
      :pagination="pagination"
      @refresh="load"
      @row-click="openDetail"
      @update:pagination="updatePagination"
    >
      <template #toolbar-left
        ><el-button
          v-if="canCreate"
          data-testid="dictionary-create"
          type="primary"
          :icon="CirclePlus"
          @click="openCreate"
          >{{ t('dictionary.create') }}</el-button
        ></template
      >
      <template #cell-status="{ row }"
        ><el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'">{{
          row.isEnabled === YesNo.Yes ? t('dictionary.enabled') : t('dictionary.disabled')
        }}</el-tag></template
      >
      <template #cell-actions="{ row }"
        ><el-button
          v-if="canUpdate"
          data-testid="dictionary-update"
          text
          :icon="Edit"
          @click.stop="openEdit(row)"
          >{{ t('dictionary.edit') }}</el-button
        ><el-button
          v-if="canStatus"
          data-testid="dictionary-status-toggle"
          text
          :icon="Switch"
          @click.stop="toggleDictionary(row)"
          >{{
            row.isEnabled === YesNo.Yes ? t('dictionary.disable') : t('dictionary.enable')
          }}</el-button
        ><el-button
          v-if="canDelete && row.isBuiltin === YesNo.No"
          data-testid="dictionary-delete"
          text
          type="danger"
          :icon="Delete"
          @click.stop="removeDictionary(row)"
          >{{ t('dictionary.delete') }}</el-button
        ></template
      >
    </AppTable>
    <DictionaryFormDialog
      v-model="dictionaryVisible"
      v-model:form="dictionaryForm"
      :editing="editingDictionary"
      :submitting="submitting"
      @save="submitDictionary"
    />
    <DictionaryDetailDialog
      v-model="detailVisible"
      :dictionary="selectedDictionary"
      :items="items"
      :can-create="canCreate"
      :can-update="canUpdate"
      :can-status="canStatus"
      :can-delete="canDelete"
      @refresh="reloadDetail"
      @create-item="openCreateItem"
      @edit-item="openEditItem"
      @toggle-item="toggleItem"
      @remove-item="removeItem"
    />
    <DictionaryItemDialog
      v-model="itemVisible"
      v-model:form="itemForm"
      :editing="editingItem"
      :submitting="submitting"
      @save="submitItem"
    />
  </AppPage>
</template>
