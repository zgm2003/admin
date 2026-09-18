<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { CircleCloseFilled, Picture, Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import type { UploadRequestOptions } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  requestObjectURL,
  requestUploadCredentials,
  type UploadCredentialItem,
} from '@/api/storage/upload'

defineOptions({ name: 'UpMedia' })

const props = withDefaults(
  defineProps<{
    modelValue: string | string[]
    ruleCode: string
    multiple?: boolean
    variant?: 'default' | 'avatar'
    accept?: string
    disabled?: boolean
    clearable?: boolean
    width?: string
  }>(),
  {
    multiple: false,
    variant: 'default',
    accept: 'image/*',
    disabled: false,
    clearable: true,
    width: '112px',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string | string[]]
  'preview-change': [value: string]
}>()
const { t } = useI18n()
const inputRef = ref<HTMLInputElement>()
const loading = ref(false)
interface PreviewState {
  url: string
  expiresAt: string | null
  requestId: number
  pending: boolean
}
const previews = ref<Record<string, PreviewState>>({})
let nextRequestId = 0
let active = true

const values = computed(() =>
  Array.isArray(props.modelValue) ? props.modelValue : props.modelValue ? [props.modelValue] : [],
)
const displayItems = computed(() =>
	values.value.map((objectKey) => ({
		objectKey,
		previewUrl: previews.value[objectKey]?.url ?? '',
	})),
)
const avatarItem = computed(() => displayItems.value[0])

watch(
  () => avatarItem.value?.previewUrl ?? '',
  (previewUrl) => emit('preview-change', previewUrl),
  { immediate: true },
)

async function resolvePreview(objectKey: string, force = false): Promise<void> {
	if (!active) return
	const current = previews.value[objectKey]
	if (current?.pending) return
	if (
		!force &&
		current?.url &&
		(current.expiresAt === null || Date.parse(current.expiresAt) > Date.now())
	) {
		return
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
		if (!active || previews.value[objectKey]?.requestId !== requestId) return
		previews.value = {
			...previews.value,
			[objectKey]: { url: result.url, expiresAt: result.expiresAt, requestId, pending: false },
		}
	} catch {
		if (!active || previews.value[objectKey]?.requestId !== requestId) return
		previews.value = {
			...previews.value,
			[objectKey]: { url: '', expiresAt: null, requestId, pending: false },
		}
	}
}

watch(
  values,
  (next) => {
		const retained: Record<string, PreviewState> = {}
		for (const objectKey of next) {
			retained[objectKey] =
				previews.value[objectKey] ?? {
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
	nextRequestId += 1
	previews.value = {}
})

function openPicker(): void {
  if (!props.disabled && !loading.value) inputRef.value?.click()
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
  loading.value = true
  try {
    const credentials = await requestUploadCredentials(
      props.ruleCode,
      selected.map((file) => ({
        fileName: file.name,
        contentType: file.type,
        fileSizeBytes: file.size,
      })),
    )
    if (credentials.items.length !== selected.length)
      throw new DirectUploadError(t('components.upMedia.uploadFailed'))
    const uploaded: UploadCredentialItem[] = []
    for (const [index, item] of credentials.items.entries()) {
      const file = selected[index]
      if (!file) continue
      let response: Response
      try {
        response = await fetch(item.uploadUrl, {
          method: item.method,
          headers: item.headers,
          body: file,
        })
      } catch {
        throw new DirectUploadError(t('components.upMedia.uploadFailed'))
      }
      if (!response.ok) throw new DirectUploadError(t('components.upMedia.uploadFailed'))
      uploaded.push(item)
    }
    const next = props.multiple
      ? [...values.value, ...uploaded.map((item) => item.objectKey)]
      : (uploaded[0]?.objectKey ?? '')
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
    emit('update:modelValue', next)
		for (const item of uploaded) {
			if (!item.publicUrl) void resolvePreview(item.objectKey, true)
		}
    return true
  } catch (error: unknown) {
    if (error instanceof DirectUploadError) ElMessage.error(error.message)
    return false
  } finally {
    loading.value = false
  }
}

function clearAt(index: number): void {
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
	void resolvePreview(objectKey, true)
}

class DirectUploadError extends Error {}
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
      :disabled="disabled || loading"
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
      v-if="clearable && avatarItem && !disabled"
      type="button"
      class="up-media__avatar-clear"
      :aria-label="t('components.upMedia.clear')"
      @click="clearAt(0)"
    >
      <CircleCloseFilled />
    </button>
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
      :disabled="disabled || loading"
      @change="onFileChange"
    />
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
        :disabled="disabled || loading || multiple"
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
        v-if="clearable && !disabled"
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
      :disabled="disabled || loading"
      :aria-label="t('components.upMedia.select')"
      @click="openPicker"
    >
      <Plus />
    </button>
  </div>
</template>

<style scoped>
.up-media {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.up-media__input {
  display: none;
}
.up-media__item {
  position: relative;
  flex: 0 0 auto;
}
.up-media__trigger,
.up-media__preview {
  display: inline-flex;
  width: 100%;
  height: 100%;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 0;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-lighter);
  border: 1px dashed var(--el-border-color);
  border-radius: 8px;
  cursor: pointer;
}
.up-media__trigger:hover {
  color: var(--el-color-primary);
  border-color: var(--el-color-primary);
}
.up-media__preview {
  border-style: solid;
}
.up-media__preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.up-media__placeholder {
  width: 28px;
  color: var(--el-text-color-placeholder);
}
.up-media__clear {
  position: absolute;
  top: -8px;
  right: -8px;
  z-index: 1;
  display: inline-flex;
  width: 20px;
  height: 20px;
  align-items: center;
  justify-content: center;
  padding: 0;
  color: var(--el-color-danger);
  background: var(--el-bg-color);
  border: 0;
  border-radius: 50%;
  cursor: pointer;
}
.is-disabled {
  opacity: 0.55;
}

.up-media--avatar {
  position: relative;
  display: inline-flex;
  width: var(--up-media-avatar-size);
}
.avatar-uploader .avatar {
  display: block;
  width: var(--up-media-avatar-size);
  height: var(--up-media-avatar-size);
  object-fit: cover;
}
:deep(.avatar-uploader .el-upload) {
  position: relative;
  overflow: hidden;
  border: 1px dashed var(--el-border-color);
  border-radius: 6px;
  cursor: pointer;
  transition: var(--el-transition-duration-fast);
}
:deep(.avatar-uploader .el-upload:hover) {
  border-color: var(--el-color-primary);
}
.avatar-uploader-icon {
  width: var(--up-media-avatar-size);
  height: var(--up-media-avatar-size);
  color: #8c939d;
  font-size: 28px;
  text-align: center;
}
.up-media__avatar-clear {
  position: absolute;
  top: -8px;
  right: -8px;
  z-index: 2;
  display: inline-flex;
  width: 20px;
  height: 20px;
  align-items: center;
  justify-content: center;
  padding: 0;
  color: var(--el-color-danger);
  background: var(--el-bg-color);
  border: 0;
  border-radius: 50%;
  cursor: pointer;
}
</style>
