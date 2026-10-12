<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { usePermissionStore } from '@/store/permission'
import {
  listFiles,
  getFileOptions,
  getFileURL,
  deleteFile,
  type StorageFile,
  type FileOptions,
} from '@/api/storage/file'
import type { CompletedFile } from '@/api/storage/file'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField } from '@/components/AppSearch'
import { formatTime } from '@/utils/datetime'
const props = withDefaults(
  defineProps<{ purpose?: string; selection?: boolean; active?: boolean }>(),
  { purpose: '', selection: false, active: true },
)
const emit = defineEmits<{ select: [file: StorageFile] }>()
const { t, locale } = useI18n()
const access = usePermissionStore()
const canList = computed(() => access.hasPermission('storage:file:list'))
const canDetail = computed(() => access.hasPermission('storage:file:detail'))
const canDelete = computed(() => !props.selection && access.hasPermission('storage:file:delete'))
const canUpload = computed(() => access.hasPermission('storage:object:upload'))
const rows = ref<StorageFile[]>([])
const options = ref<FileOptions | null>(null)
const filters = ref({ keyword: '', category: '' })
const query = ref({ keyword: '', category: '' })
const page = ref(1),
  pageSize = ref(20),
  total = ref(0)
const loading = ref(false),
  error = ref(''),
  ruleCode = ref('')
const uploadedKeys = ref<string[]>([])
const uploadBusy = ref(false)
const previewVisible = ref(false),
  previewURL = ref(''),
  previewName = ref(''),
  previewCategory = ref(''),
  previewLoading = ref(false)
let sequence = 0,
  previewSequence = 0
let controller: AbortController | null = null,
  previewController: AbortController | null = null
const pagination = computed<TablePaginationState>(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))
const resultState = computed(() =>
  loading.value ? 'loading' : error.value ? 'error' : rows.value.length ? 'success' : 'empty',
)
const fields = computed<SearchField<{ keyword: string; category: string }>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('storage.file.fileName'),
    resetValue: '',
    testId: 'file-keyword',
  },
  {
    key: 'category',
    type: 'select-v2',
    label: t('storage.file.category'),
    options: options.value?.categories ?? [],
    resetValue: '',
    testId: 'file-category',
  },
])
const columns = computed<TableColumn<StorageFile>[]>(() => [
  { prop: 'fileName', label: t('storage.file.fileName'), minWidth: 220 },
  { prop: 'category', label: t('storage.file.category'), width: 110 },
  { prop: 'sizeBytes', label: t('storage.file.size'), width: 120 },
  { prop: 'createdAt', label: t('storage.file.createdAt'), width: 180 },
  { key: 'actions', label: t('storage.file.actions'), width: 240 },
])
const uploadRule = computed(() =>
  options.value?.uploadRules.find((item) => item.value === ruleCode.value),
)
async function load(): Promise<void> {
  const current = ++sequence
  controller?.abort()
  controller = new AbortController()
  if (!props.active || !canList.value) {
    rows.value = []
    options.value = null
    total.value = 0
    loading.value = false
    return
  }
  loading.value = true
  error.value = ''
  const signal = controller.signal
  try {
    if (options.value === null) {
      const data = await getFileOptions(props.purpose, signal)
      if (current !== sequence) return
      options.value = data
      if (!data.uploadRules.some((item) => item.value === ruleCode.value))
        ruleCode.value = data.uploadRules[0]?.value ?? ''
    }
    const result = await listFiles(
      { page: page.value, pageSize: pageSize.value, ...query.value, purpose: props.purpose },
      signal,
    )
    if (current !== sequence) return
    rows.value = result.list
    total.value = result.total
    page.value = result.page
    pageSize.value = result.pageSize
  } catch (cause: unknown) {
    if (current === sequence && !signal.aborted) {
      rows.value = []
      error.value = cause instanceof Error ? cause.message : t('storage.file.loadFailed')
    }
  } finally {
    if (current === sequence) loading.value = false
  }
}
function search(value: { keyword: string; category: string }): void {
  filters.value = { ...value }
  query.value = { ...value }
  page.value = 1
  void load()
}
function paginate(value: TablePaginationState): void {
  page.value = value.pageSize === pageSize.value ? value.currentPage : 1
  pageSize.value = value.pageSize
  void load()
}
async function preview(row: StorageFile): Promise<void> {
  if (!canDetail.value) return
  const current = ++previewSequence
  previewController?.abort()
  previewController = new AbortController()
  previewVisible.value = true
  previewURL.value = ''
  previewName.value = row.fileName
  previewCategory.value = row.category
  previewLoading.value = true
  try {
    const result = await getFileURL(row.id, 'inline', previewController.signal)
    if (current === previewSequence && previewVisible.value) previewURL.value = result.url
  } catch {
    /* request layer owns error notification */
  } finally {
    if (current === previewSequence) previewLoading.value = false
  }
}
async function download(row: StorageFile): Promise<void> {
  if (!canDetail.value) return
  try {
    const result = await getFileURL(row.id, 'attachment')
    if (!props.active || !canDetail.value) return
    const link = document.createElement('a')
    link.href = result.url
    link.download = row.fileName
    link.rel = 'noopener noreferrer'
    document.body.appendChild(link)
    link.click()
    link.remove()
  } catch {
    /* request layer owns errors */
  }
}
async function remove(row: StorageFile): Promise<void> {
  if (!canDelete.value) return
  try {
    await ElMessageBox.confirm(
      t('storage.file.deleteConfirm', { name: row.fileName }),
      t('confirm.title'),
      { type: 'warning' },
    )
    if (!canDelete.value) return
    await deleteFile(row.id)
    ElNotification.success({ title: t('storage.file.deleteAccepted') })
    if (rows.value.length === 1 && page.value > 1) page.value--
    await load()
  } catch {
    /* canceled confirmation sends no request */
  }
}
function uploaded(items: CompletedFile[]): void {
  if (items.length) void load()
  void nextTick(() => {
    uploadedKeys.value = []
  })
}
watch(canDetail, (allowed) => {
  if (!allowed) previewVisible.value = false
})
watch(previewVisible, (visible) => {
  if (!visible) {
    previewSequence++
    previewController?.abort()
    previewLoading.value = false
    previewURL.value = ''
  }
})
watch(
  [locale, canList, () => props.purpose, () => props.active],
  () => {
    options.value = null
    page.value = 1
    void load()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  sequence++
  previewSequence++
  controller?.abort()
  previewController?.abort()
})
</script>
<template>
  <section class="file-browser">
    <AppSearch
      v-if="canList && options !== null"
      v-model="filters"
      :fields="fields"
      @query="search"
      @reset="search"
    />
    <AppTable
      :data="rows"
      :columns="columns"
      :pagination="pagination"
      :loading="loading"
      :result-state="resultState"
      :status-message="error"
      row-key="id"
      @update:pagination="paginate"
      @refresh="load"
    >
      <template #toolbar-left>
        <template v-if="canUpload && canList">
          <el-select-v2
            v-if="options && options.uploadRules.length"
            v-model="ruleCode"
            :options="options.uploadRules"
            :disabled="uploadBusy"
            class="file-browser__purpose"
            :placeholder="t('storage.file.uploadPurpose')"
          />
          <UpMedia
            v-if="uploadRule"
            v-model="uploadedKeys"
            :rule-code="ruleCode"
            variant="file"
            :accept="uploadRule.accept"
            multiple
            :clearable="false"
            @uploaded="uploaded"
            @uploading-change="uploadBusy = $event"
          />
          <span v-else-if="options" class="file-browser__hint">{{
            t('storage.file.noUploadRule')
          }}</span>
        </template>
      </template>
      <template #cell-category="{ row }: { row: StorageFile }">{{
        options?.categories.find((item) => item.value === row.category)?.label ?? row.category
      }}</template>
      <template #cell-sizeBytes="{ row }: { row: StorageFile }"
        >{{ new Intl.NumberFormat(locale).format(row.sizeBytes) }} B</template
      >
      <template #cell-createdAt="{ row }: { row: StorageFile }">{{
        formatTime(row.createdAt)
      }}</template>
      <template #cell-actions="{ row }: { row: StorageFile }">
        <el-button
          v-if="selection && canDetail"
          type="primary"
          link
          :data-testid="`file-select-${row.id}`"
          @click="emit('select', row)"
          >{{ t('storage.file.select') }}</el-button
        >
        <el-button
          v-if="canDetail"
          type="primary"
          link
          :data-testid="`file-preview-${row.id}`"
          @click="preview(row)"
          >{{ t('storage.file.preview') }}</el-button
        >
        <el-button v-if="canDetail" type="primary" link @click="download(row)">{{
          t('storage.file.download')
        }}</el-button>
        <el-button
          v-if="canDelete"
          type="danger"
          link
          :data-testid="`file-delete-${row.id}`"
          @click="remove(row)"
          >{{ t('storage.file.delete') }}</el-button
        >
      </template>
    </AppTable>
    <AppDialog v-model="previewVisible" :title="previewName" width="800px">
      <div v-loading="previewLoading" class="file-browser__preview">
        <img
          v-if="previewURL && previewCategory === 'image'"
          :src="previewURL"
          :alt="previewName"
          @error="previewURL = ''"
        />
        <video v-else-if="previewURL && previewCategory === 'video'" :src="previewURL" controls />
        <audio v-else-if="previewURL && previewCategory === 'audio'" :src="previewURL" controls />
        <el-empty v-else-if="!previewLoading" :description="t('storage.file.previewUnavailable')" />
      </div>
    </AppDialog>
  </section>
</template>
<style scoped src="./FileBrowser.scss" lang="scss"></style>
