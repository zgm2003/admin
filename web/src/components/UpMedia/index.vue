<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, ref, watch } from 'vue'
import { CircleCloseFilled, Picture, Plus } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import type { CompletedFile, StorageFile } from '@/api/storage/file'
import { requestObjectURL } from '@/api/storage/upload'
import { uploadMediaFiles, DirectUploadError, sameMediaValue, acceptsMediaFile } from './upload'
import type { UpMediaProps, PreviewState, MediaFileItem } from './types'
import FileList from './components/FileList/index.vue'

defineOptions({ name: 'UpMedia' })

const FileSelectDialog = defineAsyncComponent(() => import('@/components/FileSelectDialog/index.vue'))

const props = withDefaults(defineProps<UpMediaProps>(), {
  multiple: false,
  variant: 'default',
  fileLabel: '',
  uploadDisabled: false,
  allowFileLibrary: false,
  fileLibraryPurpose: '',
  accept: 'image/*',
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
const libraryFile = ref<StorageFile | null>(null)
const displayFileLabel = computed(() => props.fileLabel || libraryFile.value?.fileName || '')
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
  ] as const,
  (next, previous) => {
    if (
      emittedModel !== null &&
      sameMediaValue(next[3], emittedModel) &&
      next[0] === previous[0] &&
      next[1] === previous[1] &&
      next[2] === previous[2]
    ) {
      emittedModel = null
      return
    }
    emittedModel = null
    uploadSequence++
    uploadController?.abort()
    loading.value = false
    uploadError.value = ''
  },
  { deep: true, flush: 'sync' },
)

const values = computed(() =>
  Array.isArray(props.modelValue) ? props.modelValue : props.modelValue ? [props.modelValue] : [],
)
const displayItems = computed<MediaFileItem[]>(() =>
  values.value.map((objectKey) => ({
    objectKey,
    previewUrl: previews.value[objectKey]?.url ?? '',
    pending: previews.value[objectKey]?.pending ?? false,
    failed:
      previews.value[objectKey] !== undefined &&
      !previews.value[objectKey]?.pending &&
      !previews.value[objectKey]?.url,
  })),
)
const avatarItem = computed(() => displayItems.value[0])

watch(
  [() => props.disabled, () => props.allowFileLibrary, () => props.fileLibraryPurpose, loading],
  () => { libraryVisible.value = false },
  { flush: 'sync' },
)

watch(
  () => avatarItem.value?.previewUrl ?? '',
  (previewUrl) => emit('preview-change', previewUrl),
  { immediate: true },
)

async function resolvePreview(objectKey: string, force = false): Promise<string> {
  if (!active) return ''
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
      url: force ? '' : (current?.url ?? ''),
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
  values,
  (next) => {
    if (libraryFile.value && !next.includes(libraryFile.value.objectKey)) libraryFile.value = null
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
  if (!active || !libraryVisible.value || !props.allowFileLibrary || props.disabled || loading.value) return
  libraryVisible.value = false
  uploadError.value = ''
  if (!acceptsMediaFile({ name: file.fileName, type: file.mimeType }, props.accept)) {
    uploadError.value = t('components.upMedia.invalidType')
    return
  }
  if (props.multiple && values.value.includes(file.objectKey)) return
  const next = props.multiple ? [...values.value, file.objectKey] : file.objectKey
  libraryFile.value = file
  emittedModel = next
  emit('selected', file)
  emit('update:modelValue', next)
}

async function uploadSelected(selected: File[]): Promise<boolean> {
  if (props.disabled || props.uploadDisabled || loading.value || selected.length === 0) return false
  emittedModel = null
  uploadError.value = ''
  libraryFile.value = null
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
      props.accept,
      controller.signal,
      current,
      (item) => confirmed.push(item),
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
    for (const item of uploaded) {
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
    for (const item of uploaded) {
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
  if (props.disabled || loading.value) return
  uploadError.value = ''
  libraryFile.value = null
  const next = values.value.filter((_value, itemIndex) => itemIndex !== index)
  const removed = values.value[index]
  if (removed) {
    const nextPreviews = { ...previews.value }
    delete nextPreviews[removed]
    previews.value = nextPreviews
  }
  emit('update:modelValue', props.multiple ? next : '')
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
  const url = await resolvePreview(objectKey, true)
  if (!url || !active || !values.value.includes(objectKey)) return
  const link = document.createElement('a')
  link.href = url
  link.download = props.fileLabel || objectKey.split('/').at(-1) || 'download'
  link.rel = 'noopener noreferrer'
  document.body.appendChild(link)
  link.click()
  link.remove()
}
</script>

<template>
  <div
    v-loading="loading"
    class="up-media"
    :class="{ 'up-media--avatar': variant === 'avatar', 'is-disabled': disabled, 'is-loading': loading }"
    :style="{ '--up-media-avatar-size': width }"
    :data-testid="variant === 'avatar' ? 'up-media-avatar' : undefined"
  >
    <input
      ref="inputRef"
      data-testid="up-media-input"
      class="up-media__input"
      type="file"
      :accept="accept"
      :multiple="multiple"
      :disabled="disabled || uploadDisabled || loading"
      @change="onFileChange"
    />
    <template v-if="variant === 'avatar'">
      <div v-loading="avatarItem?.pending" class="up-media__avatar-preview">
        <img
          v-if="avatarItem?.previewUrl"
          :src="avatarItem.previewUrl"
          class="avatar"
          :alt="t('components.upMedia.avatarPreview')"
          @error="handlePreviewError(avatarItem.objectKey)"
        />
        <Picture v-else class="up-media__placeholder" aria-hidden="true" />
      </div>
      <div class="up-media__avatar-content">
        <div class="up-media__actions">
          <el-button
            data-testid="up-media-avatar-upload"
            type="primary"
            plain
            :disabled="disabled || uploadDisabled || loading"
            @click="openPicker"
          >{{ t(avatarItem ? 'components.upMedia.replaceAvatar' : 'components.upMedia.uploadAvatar') }}</el-button>
          <el-button
            v-if="allowFileLibrary"
            data-testid="up-media-library"
            type="primary"
            plain
            :disabled="disabled || loading"
            @click="openLibrary"
          >{{ t('components.upMedia.chooseFromFiles') }}</el-button>
          <el-button
            v-if="clearable && avatarItem"
            data-testid="up-media-avatar-clear"
            type="danger"
            link
            :disabled="disabled || loading"
            @click="clearAt(0)"
          >{{ t('components.upMedia.clearFile') }}</el-button>
        </div>
        <p v-if="displayFileLabel" class="up-media__name" :title="displayFileLabel">{{ displayFileLabel }}</p>
        <div v-if="$slots.tip" class="up-media__tip"><slot name="tip" /></div>
        <p v-if="avatarItem?.failed" class="up-media__preview-error" role="status">
          {{ t('components.upMedia.resolveFailed') }}
          <el-button link type="primary" @click="resolvePreview(avatarItem.objectKey, true)">{{ t('components.upMedia.retry') }}</el-button>
        </p>
      </div>
    </template>
    <FileList
      v-else-if="variant === 'file'"
      :items="displayItems"
      :label="displayFileLabel"
      :disabled="disabled"
      :upload-disabled="uploadDisabled"
      :loading="loading"
      :clearable="clearable"
      :multiple="multiple"
      @select="openPicker"
      @clear="clearAt"
      @download="downloadFile"
      @retry="(key) => resolvePreview(key, true)"
    />
    <template v-else>
      <div
        v-for="(item, index) in displayItems"
        :key="item.objectKey"
        class="up-media__item"
        :style="{ width, height: width }"
        :title="item.objectKey"
      >
        <button
          type="button"
          class="up-media__preview"
          :disabled="disabled || uploadDisabled || loading || multiple"
          @click="openPicker"
        >
          <img
            v-if="item.previewUrl"
            :src="item.previewUrl"
            alt=""
            @error="handlePreviewError(item.objectKey)"
          />
          <Picture v-else class="up-media__placeholder" />
        </button>
        <button
          v-if="clearable && !disabled && !loading"
          type="button"
          class="up-media__clear"
          :aria-label="t('components.upMedia.clear')"
          @click="clearAt(index)"
        >
          <CircleCloseFilled />
        </button>
      </div>
      <button
        v-if="multiple || displayItems.length === 0"
        type="button"
        class="up-media__trigger"
        :style="{ width, height: width }"
        :disabled="disabled || uploadDisabled || loading"
        :aria-label="t('components.upMedia.select')"
        @click="openPicker"
      >
        <Plus />
      </button>
    </template>
    <div v-if="variant !== 'avatar' && allowFileLibrary" class="up-media__actions">
      <el-button
        data-testid="up-media-library"
        type="primary"
        plain
        :disabled="disabled || loading"
        @click="openLibrary"
      >{{ t('components.upMedia.chooseFromFiles') }}</el-button>
    </div>
    <p v-if="uploadError" class="up-media__error" role="alert">{{ uploadError }}</p>
    <FileSelectDialog
      v-if="libraryVisible"
      v-model="libraryVisible"
      :purpose="fileLibraryPurpose"
      @select="selectFromLibrary"
    />
  </div>
</template>

<style scoped src="./UpMedia.scss" lang="scss"></style>
