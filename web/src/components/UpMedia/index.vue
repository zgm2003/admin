<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { CircleCloseFilled, Picture, Plus } from '@element-plus/icons-vue'
import type { UploadRequestOptions } from 'element-plus'
import { useI18n } from 'vue-i18n'

import { requestObjectURL } from '@/api/storage/upload'
import { uploadMediaFiles, DirectUploadError, sameMediaValue } from './upload'
import type { UpMediaProps, PreviewState, MediaFileItem } from './types'
import FileList from './components/FileList/index.vue'

defineOptions({ name: 'UpMedia' })

const props = withDefaults(defineProps<UpMediaProps>(), {
  multiple: false,
  variant: 'default',
  fileLabel: '',
  uploadDisabled: false,
  accept: 'image/*',
  disabled: false,
  clearable: true,
  width: '112px',
})

const emit = defineEmits<{
  'update:modelValue': [value: string | string[]]
  'preview-change': [value: string]
  'uploading-change': [value: boolean]
}>()
const { t } = useI18n()
const inputRef = ref<HTMLInputElement>()
const loading = ref(false)
const uploadError = ref('')
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

async function onAvatarUpload(options: UploadRequestOptions): Promise<void> {
  await uploadSelected([options.file])
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
  try {
    const uploaded = await uploadMediaFiles(
      props.ruleCode,
      selected,
      props.accept,
      controller.signal,
      current,
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
    emit('update:modelValue', next)
    for (const item of uploaded) {
      if (!item.publicUrl) void resolvePreview(item.objectKey, true)
    }
    return true
  } catch (error: unknown) {
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
    v-if="variant === 'avatar'"
    v-loading="loading"
    class="up-media up-media--avatar"
    :class="{ 'is-disabled': disabled, 'is-loading': loading }"
    :style="{ '--up-media-avatar-size': width }"
    data-testid="up-media-avatar"
  >
    <el-upload
      class="avatar-uploader"
      :show-file-list="false"
      :accept="accept"
      :disabled="disabled || uploadDisabled || loading"
      :http-request="onAvatarUpload"
    >
      <img
        v-if="avatarItem?.previewUrl"
        :src="avatarItem.previewUrl"
        class="avatar"
        alt=""
        @error="handlePreviewError(avatarItem.objectKey)"
      />
      <el-icon v-else class="avatar-uploader-icon"><Plus /></el-icon>
    </el-upload>
    <button
      v-if="clearable && avatarItem && !disabled && !loading"
      type="button"
      class="up-media__avatar-clear"
      :aria-label="t('components.upMedia.clear')"
      @click="clearAt(0)"
    >
      <CircleCloseFilled />
    </button>
    <p v-if="uploadError" class="up-media__error" role="alert">{{ uploadError }}</p>
  </div>
  <div
    v-else
    v-loading="loading"
    class="up-media"
    :class="{ 'is-disabled': disabled, 'is-loading': loading }"
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
    <FileList
      v-if="variant === 'file'"
      :items="displayItems"
      :label="fileLabel"
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
    <p v-if="uploadError" class="up-media__error" role="alert">{{ uploadError }}</p>
  </div>
</template>

<style scoped src="./UpMedia.css"></style>
