<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch, type Component } from 'vue'
import {
  Document,
  Download,
  FullScreen,
  Headset,
  Picture,
  Refresh,
  RefreshLeft,
  RefreshRight,
  VideoCamera,
  ZoomIn,
  ZoomOut,
} from '@element-plus/icons-vue'
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
  error = ref('')
const previewVisible = ref(false),
  previewURL = ref(''),
  previewName = ref(''),
  previewCategory = ref(''),
  previewRow = ref<StorageFile | null>(null),
  previewLoading = ref(false)
let sequence = 0,
  previewSequence = 0
let controller: AbortController | null = null,
  previewController: AbortController | null = null
const thumbnailURLs = ref<Record<number, string>>({})
const thumbnailLoading = ref<Record<number, boolean>>({})
const thumbnailControllers = new Map<number, AbortController>()
const thumbnailQueue: StorageFile[] = []
const fields = computed<SearchField<{ keyword: string; category: string }>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('storage.file.fileName'),
    placeholder: t('storage.file.fileNamePlaceholder'),
    resetValue: '',
    testId: 'file-keyword',
  },
  {
    key: 'category',
    type: 'select-v2',
    label: t('storage.file.category'),
    placeholder: t('storage.file.categoryPlaceholder'),
    options: options.value?.categories ?? [],
    resetValue: '',
    testId: 'file-category',
  },
])
const uploadRule = computed(() => options.value?.uploadRules[0])
const uploadLimit = computed(() => {
  if (!uploadRule.value) return ''
  const bytes = uploadRule.value.maxFileSizeBytes
  const [divisor, unit]: [number, string] =
    bytes >= 1024 * 1024 ? [1024 * 1024, 'MB'] : bytes >= 1024 ? [1024, 'KB'] : [1, 'B']
  const size = new Intl.NumberFormat(locale.value, { maximumFractionDigits: 2 }).format(
    bytes / divisor,
  )
  return t('storage.file.uploadLimit', { size: `${size} ${unit}` })
})
async function load(): Promise<void> {
  const current = ++sequence
  controller?.abort()
  clearThumbnails()
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
function paginate(currentPage: number): void {
  page.value = currentPage
  void load()
}
function changePageSize(nextPageSize: number): void {
  page.value = 1
  pageSize.value = nextPageSize
  void load()
}
function thumbnailURL(row: StorageFile): string {
  return thumbnailURLs.value[row.id] ?? ''
}
function thumbnailKind(row: StorageFile): 'image' | 'video' | 'file' {
  if (row.category === 'image') return 'image'
  if (row.category === 'video') return 'video'
  return 'file'
}
function categoryIcon(category: string): Component {
  if (category === 'image') return Picture
  if (category === 'video') return VideoCamera
  if (category === 'audio') return Headset
  return Document
}
function categoryLabel(category: string): string {
  return options.value?.categories.find((item) => item.value === category)?.label ?? category
}
function canPreview(row: StorageFile): boolean {
  return row.category === 'image' || row.category === 'video' || row.category === 'audio'
}
function invalidateThumbnail(id: number): void {
  const next = { ...thumbnailURLs.value }
  delete next[id]
  thumbnailURLs.value = next
}
function clearThumbnails(): void {
  thumbnailControllers.forEach((request) => request.abort())
  thumbnailControllers.clear()
  thumbnailQueue.splice(0)
  thumbnailURLs.value = {}
  thumbnailLoading.value = {}
}
async function ensureThumbnail(row: StorageFile): Promise<void> {
  if (
    !props.active ||
    !canList.value ||
    !canDetail.value ||
    loading.value ||
    (row.category !== 'image' && row.category !== 'video')
  )
    return
  if (thumbnailURL(row) || thumbnailLoading.value[row.id]) return
  if (thumbnailControllers.size >= 4) {
    if (!thumbnailQueue.some((item) => item.id === row.id)) thumbnailQueue.push(row)
    return
  }
  const request = new AbortController()
  thumbnailControllers.set(row.id, request)
  thumbnailLoading.value = { ...thumbnailLoading.value, [row.id]: true }
  try {
    const result = await getFileURL(row.id, 'inline', request.signal)
    if (thumbnailControllers.get(row.id) === request && !request.signal.aborted) {
      thumbnailURLs.value = { ...thumbnailURLs.value, [row.id]: result.url }
    }
  } catch {
    /* request layer owns error notification */
  } finally {
    if (thumbnailControllers.get(row.id) === request) {
      thumbnailControllers.delete(row.id)
      const next = { ...thumbnailLoading.value }
      delete next[row.id]
      thumbnailLoading.value = next
      const queued = thumbnailQueue.shift()
      if (queued !== undefined && !request.signal.aborted) void ensureThumbnail(queued)
    }
  }
}
async function preview(row: StorageFile): Promise<void> {
  if (!canDetail.value || !canPreview(row)) return
  const current = ++previewSequence
  previewController?.abort()
  previewController = new AbortController()
  previewVisible.value = true
  previewURL.value = ''
  previewName.value = row.fileName
  previewCategory.value = row.category
  previewRow.value = row
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
function closePreview(): void {
  previewVisible.value = false
}
function handlePreviewError(): void {
  previewURL.value = ''
}
async function downloadPreview(): Promise<void> {
  const row = previewRow.value
  if (row) await download(row)
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
}
watch(
  rows,
  (items) => {
    clearThumbnails()
    items.forEach((item) => void ensureThumbnail(item))
  },
  { flush: 'post' },
)
watch(canDetail, (allowed) => {
  if (!allowed) {
    previewVisible.value = false
    clearThumbnails()
  } else {
    rows.value.forEach((item) => void ensureThumbnail(item))
  }
})
watch(previewVisible, (visible) => {
  if (!visible) {
    previewSequence++
    previewController?.abort()
    previewLoading.value = false
    previewURL.value = ''
    previewRow.value = null
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
  clearThumbnails()
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
    <div v-if="canList" class="file-browser__toolbar">
      <div class="file-browser__toolbar-left">
        <template v-if="canUpload">
          <UpMedia
            v-if="uploadRule"
            :rule-code="uploadRule.value"
            variant="button"
            :accept="uploadRule.accept"
            multiple
            :clearable="false"
            @uploaded="uploaded"
          />
          <span v-if="uploadRule" class="file-browser__hint" data-testid="file-upload-limit">{{
            uploadLimit
          }}</span>
          <span v-else-if="options" class="file-browser__hint">{{
            t('storage.file.noUploadRule')
          }}</span>
        </template>
      </div>
      <el-button
        type="primary"
        :icon="Refresh"
        :loading="loading"
        data-testid="file-refresh"
        @click="load"
        >{{ t('appTable.refresh') }}</el-button
      >
    </div>
    <div class="file-browser__surface" :aria-busy="loading">
      <el-skeleton v-if="loading" :rows="4" animated />
      <div v-else-if="error" class="file-browser__error" role="alert">{{ error }}</div>
      <el-empty v-else-if="!rows.length" :description="t('appTable.empty')" />
      <div v-else class="file-browser__grid">
        <article
          v-for="row in rows"
          :key="row.id"
          class="file-browser__card"
          :class="{ 'is-selectable': selection && canDetail }"
          :data-testid="'file-card-' + row.id"
          :tabindex="selection && canDetail ? 0 : undefined"
          @click="selection && canDetail ? emit('select', row) : undefined"
          @keydown.enter.self="selection && canDetail ? emit('select', row) : undefined"
          @keydown.space.self.prevent="selection && canDetail ? emit('select', row) : undefined"
        >
          <div
            class="file-browser__thumbnail"
            :data-kind="thumbnailKind(row)"
            :class="{ 'is-loading': thumbnailLoading[row.id] }"
          >
            <img
              v-if="thumbnailURL(row) && row.category === 'image'"
              :src="thumbnailURL(row)"
              :alt="row.fileName"
              loading="lazy"
              @error="invalidateThumbnail(row.id)"
            />
            <video
              v-else-if="thumbnailURL(row) && row.category === 'video'"
              :src="thumbnailURL(row)"
              muted
              playsinline
              preload="metadata"
            />
            <el-icon v-else :size="42"><component :is="categoryIcon(row.category)" /></el-icon>
          </div>
          <div class="file-browser__card-body">
            <p class="file-browser__name" :title="row.fileName">{{ row.fileName }}</p>
            <p class="file-browser__meta">
              <span>{{ categoryLabel(row.category) }}</span>
              <span>{{ new Intl.NumberFormat(locale).format(row.sizeBytes) }} B</span>
            </p>
            <p class="file-browser__time">{{ formatTime(row.createdAt) }}</p>
          </div>
          <div class="file-browser__actions" @click.stop>
            <el-button
              v-if="selection && canDetail"
              type="primary"
              link
              :data-testid="'file-select-' + row.id"
              @click="emit('select', row)"
              >{{ t('storage.file.select') }}</el-button
            >
            <el-button
              v-if="canDetail && canPreview(row)"
              type="primary"
              link
              :data-testid="'file-preview-' + row.id"
              @click="preview(row)"
              >{{ t('storage.file.preview') }}</el-button
            >
            <el-button
              v-if="canDetail"
              type="primary"
              link
              :data-testid="'file-download-' + row.id"
              @click="download(row)"
              >{{ t('storage.file.download') }}</el-button
            >
            <el-button
              v-if="canDelete"
              type="danger"
              link
              :data-testid="'file-delete-' + row.id"
              @click="remove(row)"
              >{{ t('storage.file.delete') }}</el-button
            >
          </div>
        </article>
      </div>
    </div>
    <div v-if="total > 0" class="file-browser__pagination">
      <el-pagination
        background
        layout="total, sizes, prev, pager, next"
        :current-page="page"
        :page-size="pageSize"
        :page-sizes="[20, 50, 100]"
        :total="total"
        @current-change="paginate"
        @size-change="changePageSize"
      />
    </div>
    <el-image-viewer
      v-if="previewVisible && previewCategory === 'image' && previewURL"
      :url-list="[previewURL]"
      :initial-index="0"
      teleported
      @close="closePreview"
      @error="handlePreviewError"
    >
      <template #toolbar="{ actions, reset }">
        <div class="file-browser__viewer-toolbar" data-testid="file-preview-image">
          <button
            type="button"
            :aria-label="t('components.upMedia.zoomOut')"
            :title="t('components.upMedia.zoomOut')"
            @click="actions('zoomOut')"
          >
            <ZoomOut />
          </button>
          <button
            type="button"
            :aria-label="t('components.upMedia.zoomIn')"
            :title="t('components.upMedia.zoomIn')"
            @click="actions('zoomIn')"
          >
            <ZoomIn />
          </button>
          <button
            type="button"
            :aria-label="t('components.upMedia.fit')"
            :title="t('components.upMedia.fit')"
            @click="reset"
          >
            <FullScreen />
          </button>
          <button
            type="button"
            :aria-label="t('components.upMedia.rotateLeft')"
            :title="t('components.upMedia.rotateLeft')"
            @click="actions('anticlockwise')"
          >
            <RefreshLeft />
          </button>
          <button
            type="button"
            :aria-label="t('components.upMedia.rotateRight')"
            :title="t('components.upMedia.rotateRight')"
            @click="actions('clockwise')"
          >
            <RefreshRight />
          </button>
          <button
            type="button"
            data-testid="file-preview-download"
            :aria-label="t('components.upMedia.download')"
            :title="t('components.upMedia.download')"
            @click="downloadPreview"
          >
            <Download />
          </button>
        </div>
      </template>
      <template #viewer-error
        ><el-empty :description="t('storage.file.previewUnavailable')"
      /></template>
    </el-image-viewer>
    <AppDialog
      v-if="previewVisible && (previewCategory === 'video' || previewCategory === 'audio')"
      v-model="previewVisible"
      :title="previewName"
      width="min(1100px, 90vw)"
      height="min(760px, 82vh)"
      body-padding="0"
    >
      <div v-loading="previewLoading" class="file-browser__preview">
        <video v-if="previewURL && previewCategory === 'video'" :src="previewURL" controls />
        <audio v-else-if="previewURL && previewCategory === 'audio'" :src="previewURL" controls />
        <el-empty v-else-if="!previewLoading" :description="t('storage.file.previewUnavailable')" />
      </div>
    </AppDialog>
  </section>
</template>
<style scoped src="./FileBrowser.scss" lang="scss"></style>
