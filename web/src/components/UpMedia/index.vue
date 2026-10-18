<!-- 组件：媒体上传 -->
<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, ref, watch } from 'vue'
import {
  Delete,
  Download,
  FullScreen,
  Picture,
  Plus,
  RefreshLeft,
  RefreshRight,
  VideoCamera,
  ZoomIn,
  ZoomOut,
} from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { CompletedFile, StorageFile } from '@/api/storage/file'
import { requestObjectURL } from '@/api/storage/upload'
import { uploadMediaFiles, DirectUploadError, sameMediaValue, acceptsMediaFile } from './upload'
import type { UpMediaProps, PreviewState, MediaFileItem } from './types'
import FileList from './components/FileList/index.vue'

defineOptions({ name: 'UpMedia' })

const FileSelectDialog = defineAsyncComponent(
  () => import('@/components/FileSelectDialog/index.vue'),
)

const props = withDefaults(defineProps<UpMediaProps>(), {
  modelValue: '',
  multiple: false,
  variant: 'image',
  fileLabel: '',
  uploadDisabled: false,
  allowFileLibrary: false,
  fileLibraryPurpose: '',
  disabled: false,
  clearable: true,
  width: '112px',
})

const emit = defineEmits<{
  'update:modelValue': [value: string | string[]]
  'preview-change': [value: string]
  'uploading-change': [value: boolean]
  uploaded: [value: CompletedFile[]]
  selected: [file: StorageFile]
}>()
const { t } = useI18n()
const inputRef = ref<HTMLInputElement>()
const loading = ref(false)
const uploadError = ref('')
const libraryVisible = ref(false)
const fileNames = ref<Record<string, string>>({})
const square = computed(() => props.variant === 'image' || props.variant === 'video')
const acceptedTypes = computed(() => props.accept ?? (square.value ? `${props.variant}/*` : ''))
const previewKey = ref('')
const videoVisible = computed({
  get: () => previewItem.value?.kind === 'video' && !!previewKey.value,
  set: (visible: boolean) => {
    if (!visible) previewKey.value = ''
  },
})
const imageItems = computed(() =>
  displayItems.value.filter((item) => item.kind === 'image' && !!item.previewUrl),
)
const imageIndex = computed(() =>
  Math.max(
    0,
    imageItems.value.findIndex((item) => item.objectKey === previewKey.value),
  ),
)
const previewItem = computed(() =>
  displayItems.value.find((item) => item.objectKey === previewKey.value),
)
const downloading = ref(false)
const downloadError = ref('')
let downloadController: AbortController | null = null
let emittedModel: string | string[] | null = null
const previews = ref<Record<string, PreviewState>>({})
let nextRequestId = 0
let active = true
let uploadSequence = 0
let uploadController: AbortController | null = null
const refreshedImages = new Set<string>()
watch(loading, (value) => emit('uploading-change', value), { flush: 'sync' })
watch(
  [
    () => props.disabled,
    () => props.uploadDisabled,
    () => props.ruleCode,
    () => props.modelValue,
    () => props.variant,
    acceptedTypes,
  ] as const,
  (next, previous) => {
    if (
      emittedModel !== null &&
      sameMediaValue(next[3], emittedModel) &&
      next[0] === previous[0] &&
      next[1] === previous[1] &&
      next[2] === previous[2] &&
      next[4] === previous[4] &&
      next[5] === previous[5]
    ) {
      emittedModel = null
      return
    }
    emittedModel = null
    uploadSequence++
    uploadController?.abort()
    loading.value = false
    uploadError.value = ''
    downloadError.value = ''
    downloadController?.abort()
    previewKey.value = ''
  },
  { deep: true, flush: 'sync' },
)

const values = computed(() =>
  Array.isArray(props.modelValue) ? props.modelValue : props.modelValue ? [props.modelValue] : [],
)
const displayItems = computed<MediaFileItem[]>(() =>
  values.value.map((objectKey) => ({
    objectKey,
    kind: mediaKind(objectKey),
    fileName:
      props.fileLabel || fileNames.value[objectKey] || objectKey.split('/').at(-1) || objectKey,
    previewUrl: previews.value[objectKey]?.url ?? '',
    pending: previews.value[objectKey]?.pending ?? false,
    failed:
      previews.value[objectKey] !== undefined &&
      !previews.value[objectKey]?.pending &&
      !previews.value[objectKey]?.url,
  })),
)
const firstItem = computed(() => displayItems.value[0])

function mediaKind(objectKey: string): MediaFileItem['kind'] {
  if (!square.value) return 'file'
  // Only chooses the HTML preview element; upload and business rules remain server-owned.
  if (/\.(mp4|webm|mov|m4v|ogv)$/i.test(objectKey)) return 'video'
  if (/\.(png|jpe?g|gif|webp|avif|bmp|svg)$/i.test(objectKey)) return 'image'
  return props.variant === 'video' ? 'video' : 'image'
}

watch(
  [
    () => props.disabled,
    () => props.allowFileLibrary,
    () => props.fileLibraryPurpose,
    () => props.ruleCode,
    acceptedTypes,
    () => props.variant,
    () => props.modelValue,
    loading,
  ],
  () => {
    libraryVisible.value = false
  },
  { flush: 'sync' },
)

watch(
  () => firstItem.value?.previewUrl ?? '',
  (previewUrl) => emit('preview-change', previewUrl),
  { immediate: true },
)

async function resolvePreview(objectKey: string, force = false): Promise<string> {
  if (!active || props.variant === 'button') return ''
  const current = previews.value[objectKey]
  if (current?.pending) return ''
  if (
    !force &&
    current?.url &&
    (current.expiresAt === null || Date.parse(current.expiresAt) > Date.now())
  ) {
    return current.url
  }
  const requestId = ++nextRequestId
  previews.value = {
    ...previews.value,
    [objectKey]: {
      url: current?.url ?? '',
      expiresAt: current?.expiresAt ?? null,
      requestId,
      pending: true,
    },
  }
  try {
    const result = await requestObjectURL(objectKey)
    if (!active || previews.value[objectKey]?.requestId !== requestId) return ''
    previews.value = {
      ...previews.value,
      [objectKey]: { url: result.url, expiresAt: result.expiresAt, requestId, pending: false },
    }
    return result.url
  } catch {
    if (!active || previews.value[objectKey]?.requestId !== requestId) return ''
    previews.value = {
      ...previews.value,
      [objectKey]: { url: '', expiresAt: null, requestId, pending: false },
    }
    return ''
  }
}

watch(
  [values, () => props.variant],
  ([next]) => {
    previewKey.value = ''
    if (props.variant === 'button') {
      previews.value = {}
      fileNames.value = {}
      return
    }
    fileNames.value = Object.fromEntries(
      Object.entries(fileNames.value).filter(([key]) => next.includes(key)),
    )
    for (const key of refreshedImages) if (!next.includes(key)) refreshedImages.delete(key)
    const retained: Record<string, PreviewState> = {}
    for (const objectKey of next) {
      retained[objectKey] = previews.value[objectKey] ?? {
        url: '',
        expiresAt: null,
        requestId: ++nextRequestId,
        pending: false,
      }
    }
    previews.value = retained
    for (const objectKey of next) void resolvePreview(objectKey)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  active = false
  uploadSequence++
  uploadController?.abort()
  downloadController?.abort()
  nextRequestId += 1
  previews.value = {}
})

function openPicker(): void {
  if (!props.disabled && !props.uploadDisabled && !loading.value) inputRef.value?.click()
}

async function onFileChange(event: Event): Promise<void> {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  target.value = ''
  if (files.length === 0) return
  const selected = props.multiple ? files : files.slice(0, 1)
  await uploadSelected(selected)
}

function openLibrary(): void {
  if (props.allowFileLibrary && !props.disabled && !loading.value) libraryVisible.value = true
}

function selectFromLibrary(file: StorageFile): void {
  if (
    !active ||
    !libraryVisible.value ||
    !props.allowFileLibrary ||
    props.disabled ||
    loading.value
  )
    return
  libraryVisible.value = false
  uploadError.value = ''
  if (!acceptsMediaFile({ name: file.fileName, type: file.mimeType }, acceptedTypes.value)) {
    uploadError.value = t('components.upMedia.invalidType')
    return
  }
  if (props.multiple && values.value.includes(file.objectKey)) return
  const next = props.multiple ? [...values.value, file.objectKey] : file.objectKey
  if (props.variant !== 'button') fileNames.value[file.objectKey] = file.fileName
  emittedModel = next
  emit('selected', file)
  emit('update:modelValue', next)
}

async function uploadSelected(selected: File[]): Promise<boolean> {
  if (props.disabled || props.uploadDisabled || loading.value || selected.length === 0) return false
  emittedModel = null
  uploadError.value = ''
  const sequence = ++uploadSequence
  const current = () =>
    active && sequence === uploadSequence && !props.disabled && !props.uploadDisabled
  const controller = new AbortController()
  uploadController = controller
  loading.value = true
  const confirmed: CompletedFile[] = []
  try {
    const uploaded = await uploadMediaFiles(
      props.ruleCode,
      selected,
      acceptedTypes.value,
      controller.signal,
      current,
      (item) => {
        if (props.variant !== 'button')
          fileNames.value[item.objectKey] = selected[confirmed.length]?.name ?? item.objectKey
        confirmed.push(item)
      },
    )
    if (!current()) return false
    const first = uploaded[0]
    if (!first) throw new DirectUploadError('uploadFailed')
    const next = props.multiple
      ? [...values.value, ...uploaded.map((item) => item.objectKey)]
      : first.objectKey
    const nextPreviews: Record<string, PreviewState> = {}
    if (props.multiple) {
      for (const objectKey of values.value) {
        const existing = previews.value[objectKey]
        if (existing) nextPreviews[objectKey] = existing
      }
    }
    for (const item of props.variant === 'button' ? [] : uploaded) {
      nextPreviews[item.objectKey] = {
        url: item.publicUrl ?? '',
        expiresAt: null,
        requestId: ++nextRequestId,
        pending: false,
      }
    }
    previews.value = nextPreviews
    emittedModel = next
    emit('uploaded', uploaded)
    emit('update:modelValue', next)
    for (const item of props.variant === 'button' ? [] : uploaded) {
      if (!item.publicUrl) void resolvePreview(item.objectKey, true)
    }
    return true
  } catch (error: unknown) {
    if (current() && confirmed.length > 0) {
      const next = props.multiple
        ? [...values.value, ...confirmed.map((item) => item.objectKey)]
        : confirmed[0]!.objectKey
      emittedModel = next
      emit('uploaded', confirmed)
      emit('update:modelValue', next)
    }
    if (current())
      uploadError.value = t(
        `components.upMedia.${error instanceof DirectUploadError ? error.reason : 'uploadFailed'}`,
      )
    return false
  } finally {
    if (sequence === uploadSequence) {
      loading.value = false
      uploadController = null
    }
  }
}

function clearAt(index: number): void {
  if (!active || !props.clearable || props.disabled || loading.value) return
  const removed = values.value[index]
  if (!removed) return
  uploadError.value = ''
  previewKey.value = ''
  const next = values.value.filter((_value, itemIndex) => itemIndex !== index)
  if (removed) {
    const nextPreviews = { ...previews.value }
    delete nextPreviews[removed]
    previews.value = nextPreviews
  }
  delete fileNames.value[removed]
  emit('update:modelValue', props.multiple ? next : '')
}

async function openPreview(objectKey: string): Promise<void> {
  if (loading.value || !square.value) return
  const variant = props.variant
  const url = await resolvePreview(objectKey)
  if (url && active && props.variant === variant && values.value.includes(objectKey))
    previewKey.value = objectKey
}

function handlePreviewError(objectKey: string): void {
  if (refreshedImages.has(objectKey)) {
    const preview = previews.value[objectKey]
    if (preview)
      previews.value = { ...previews.value, [objectKey]: { ...preview, url: '', pending: false } }
    return
  }
  refreshedImages.add(objectKey)
  void resolvePreview(objectKey, true)
}

async function downloadFile(objectKey: string): Promise<void> {
  if (downloading.value) return
  const controller = new AbortController()
  downloadController = controller
  downloading.value = true
  downloadError.value = ''
  let blobURL = ''
  try {
    const url = await resolvePreview(objectKey, true)
    if (!url || !active || controller.signal.aborted || !values.value.includes(objectKey)) return
    const response = await fetch(url, { signal: controller.signal })
    if (!response.ok) throw new Error('Download failed')
    const blob = await response.blob()
    if (!active || controller.signal.aborted || !values.value.includes(objectKey)) return
    // A download attribute on a cross-origin COS URL can open the image instead of downloading.
    blobURL = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = blobURL
    link.download =
      displayItems.value.find((item) => item.objectKey === objectKey)?.fileName || 'download'
    document.body.appendChild(link)
    link.click()
    link.remove()
  } catch {
    if (active && !controller.signal.aborted)
      downloadError.value = t('components.upMedia.downloadFailed')
  } finally {
    if (blobURL) URL.revokeObjectURL(blobURL)
    downloading.value = false
    downloadController = null
  }
}
</script>

<template>
  <div
    v-loading="loading"
    class="up-media"
    :class="{ 'is-disabled': disabled, 'is-loading': loading, 'up-media--square': square }"
    :style="{ '--up-media-size': width }"
  >
    <input
      ref="inputRef"
      data-testid="up-media-input"
      class="up-media__input"
      type="file"
      :accept="acceptedTypes"
      :multiple="multiple"
      :disabled="disabled || uploadDisabled || loading"
      @change="onFileChange"
    />
    <div v-if="square" class="up-media__uploads">
      <div v-for="(item, index) in displayItems" :key="item.objectKey" class="up-media__item">
        <div v-loading="item.pending" class="up-media__preview">
          <img
            v-if="item.previewUrl && item.kind === 'image'"
            :src="item.previewUrl"
            :alt="item.fileName"
            @error="handlePreviewError(item.objectKey)"
          />
          <video
            v-else-if="item.previewUrl"
            :src="item.previewUrl"
            muted
            playsinline
            preload="metadata"
            @error="handlePreviewError(item.objectKey)"
          />
          <component
            :is="item.kind === 'video' ? VideoCamera : Picture"
            v-else
            class="up-media__placeholder"
            aria-hidden="true"
          />
        </div>
        <div class="up-media__actions">
          <button
            type="button"
            data-testid="up-media-preview"
            :aria-label="t('components.upMedia.preview')"
            :title="t('components.upMedia.preview')"
            :disabled="loading || item.pending"
            @click="openPreview(item.objectKey)"
          >
            <ZoomIn aria-hidden="true" />
          </button>
          <button
            v-if="clearable && !disabled"
            type="button"
            class="up-media__clear"
            :aria-label="t('components.upMedia.clear')"
            :title="t('components.upMedia.clear')"
            :disabled="loading"
            @click="clearAt(index)"
          >
            <Delete aria-hidden="true" />
          </button>
        </div>
      </div>
      <button
        v-if="multiple || displayItems.length === 0"
        type="button"
        data-testid="up-media-upload"
        class="up-media__trigger"
        :disabled="disabled || uploadDisabled || loading"
        :aria-label="t('components.upMedia.select')"
        @click="openPicker"
      >
        <Plus aria-hidden="true" />
      </button>
    </div>
    <div class="up-media__controls">
      <el-button
        v-if="!square"
        data-testid="up-media-upload"
        type="primary"
        :disabled="disabled || uploadDisabled || loading"
        @click="openPicker"
        >{{ t('components.upMedia.upload') }}</el-button
      >
      <el-button
        v-if="allowFileLibrary"
        data-testid="up-media-library"
        class="up-media__library"
        type="primary"
        plain
        :disabled="disabled || loading"
        @click="openLibrary"
        >{{ t('components.upMedia.chooseFromFiles') }}</el-button
      >
    </div>
    <FileList
      v-if="variant === 'file' && displayItems.length > 0"
      :items="displayItems"
      :disabled="disabled"
      :loading="loading || downloading"
      :clearable="clearable"
      @clear="clearAt"
      @download="downloadFile"
      @retry="(key) => resolvePreview(key, true)"
    />
    <p v-if="uploadError" class="up-media__error" role="alert">{{ uploadError }}</p>
    <p v-if="downloadError" class="up-media__error" role="alert">{{ downloadError }}</p>
    <el-image-viewer
      v-if="previewItem?.kind === 'image' && previewKey && previewItem.previewUrl"
      :url-list="imageItems.map((item) => item.previewUrl)"
      :initial-index="imageIndex"
      :show-progress="multiple"
      teleported
      @close="previewKey = ''"
      @switch="(index: number) => (previewKey = imageItems[index]?.objectKey ?? '')"
      @error="handlePreviewError(previewKey)"
    >
      <template #toolbar="{ actions, reset }">
        <div class="up-media__viewer-toolbar" data-testid="up-media-preview-image">
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
            data-testid="up-media-viewer-download"
            :disabled="downloading"
            :aria-label="t('components.upMedia.download')"
            :title="t('components.upMedia.download')"
            @click="downloadFile(previewKey)"
          >
            <Download />
          </button>
        </div>
      </template>
      <template #viewer-error
        ><el-empty :description="t('components.upMedia.previewUnavailable')"
      /></template>
      <p v-if="downloadError" class="up-media__viewer-error" role="alert">{{ downloadError }}</p>
    </el-image-viewer>
    <AppDialog
      v-if="videoVisible"
      v-model="videoVisible"
      :title="t('components.upMedia.preview')"
      width="800px"
      height="auto"
    >
      <video
        v-if="previewItem?.previewUrl"
        data-testid="up-media-preview-video"
        class="up-media__player"
        :src="previewItem.previewUrl"
        controls
        playsinline
        preload="metadata"
        @error="handlePreviewError(previewKey)"
      />
      <el-empty v-else :description="t('components.upMedia.previewUnavailable')" />
      <p v-if="downloadError" class="up-media__error" role="alert">{{ downloadError }}</p>
      <template #footer
        ><el-button type="primary" :disabled="downloading" @click="downloadFile(previewKey)">{{
          t('components.upMedia.download')
        }}</el-button></template
      >
    </AppDialog>
    <FileSelectDialog
      v-if="libraryVisible"
      v-model="libraryVisible"
      :purpose="fileLibraryPurpose"
      @select="selectFromLibrary"
    />
  </div>
</template>

<style scoped src="./UpMedia.scss" lang="scss"></style>
