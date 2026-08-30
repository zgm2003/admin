<script setup lang="ts">
import { computed, ref } from 'vue'
import { CircleCloseFilled, Picture, Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'

import { requestUploadCredentials, type UploadCredentialItem } from '../../../api/storage/upload'

const props = withDefaults(defineProps<{
  modelValue: string | string[]
  ruleCode: string
  multiple?: boolean
  accept?: string
  disabled?: boolean
  clearable?: boolean
  width?: string
}>(), { multiple: false, accept: 'image/*', disabled: false, clearable: true, width: '112px' })

const emit = defineEmits<{ 'update:modelValue': [value: string | string[]] }>()
const { t } = useI18n()
const inputRef = ref<HTMLInputElement>()
const loading = ref(false)
const previewUrls = ref<Record<string, string>>({})

const values = computed(() => Array.isArray(props.modelValue) ? props.modelValue : props.modelValue ? [props.modelValue] : [])
const displayItems = computed(() => values.value.map((objectKey) => ({ objectKey, previewUrl: previewUrls.value[objectKey] ?? '' })))

function openPicker(): void {
  if (!props.disabled && !loading.value) inputRef.value?.click()
}

async function onFileChange(event: Event): Promise<void> {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  target.value = ''
  if (files.length === 0) return
  const selected = props.multiple ? files : files.slice(0, 1)
  loading.value = true
  try {
    const credentials = await requestUploadCredentials(props.ruleCode, selected.map((file) => ({ fileName: file.name, contentType: file.type, fileSizeBytes: file.size })))
    if (credentials.items.length !== selected.length) throw new DirectUploadError(t('components.upMedia.uploadFailed'))
    const uploaded: UploadCredentialItem[] = []
    for (const [index, item] of credentials.items.entries()) {
      const file = selected[index]
      if (!file) continue
      let response: Response
      try {
        response = await fetch(item.uploadUrl, { method: item.method, headers: item.headers, body: file })
      } catch {
        throw new DirectUploadError(t('components.upMedia.uploadFailed'))
      }
      if (!response.ok) throw new DirectUploadError(t('components.upMedia.uploadFailed'))
      uploaded.push(item)
    }
    const next = props.multiple ? [...values.value, ...uploaded.map((item) => item.objectKey)] : (uploaded[0]?.objectKey ?? '')
    const nextPreviews = props.multiple ? { ...previewUrls.value } : {}
    for (const item of uploaded) nextPreviews[item.objectKey] = item.publicUrl ?? ''
    previewUrls.value = nextPreviews
    emit('update:modelValue', next)
  } catch (error: unknown) {
    if (error instanceof DirectUploadError) ElMessage.error(error.message)
  } finally {
    loading.value = false
  }
}

function clearAt(index: number): void {
  const next = values.value.filter((_value, itemIndex) => itemIndex !== index)
  const removed = values.value[index]
  if (removed) {
    const nextPreviews = { ...previewUrls.value }
    delete nextPreviews[removed]
    previewUrls.value = nextPreviews
  }
  emit('update:modelValue', props.multiple ? next : '')
}

class DirectUploadError extends Error {}
</script>

<template>
  <div v-loading="loading" class="up-media" :class="{ 'is-disabled': disabled, 'is-loading': loading }">
    <input ref="inputRef" data-testid="up-media-input" class="up-media__input" type="file" :accept="accept" :multiple="multiple" :disabled="disabled || loading" @change="onFileChange">
    <div v-for="(item, index) in displayItems" :key="item.objectKey" class="up-media__item" :style="{ width, height: width }" :title="item.objectKey">
      <button type="button" class="up-media__preview" :disabled="disabled || loading || multiple" @click="openPicker">
        <img v-if="item.previewUrl" :src="item.previewUrl" alt="">
        <Picture v-else class="up-media__placeholder" />
      </button>
      <button v-if="clearable && !disabled" type="button" class="up-media__clear" :aria-label="t('components.upMedia.clear')" @click="clearAt(index)"><CircleCloseFilled /></button>
    </div>
    <button v-if="multiple || displayItems.length === 0" type="button" class="up-media__trigger" :style="{ width, height: width }" :disabled="disabled || loading" :aria-label="t('components.upMedia.select')" @click="openPicker"><Plus /></button>
  </div>
</template>

<style scoped>
.up-media { display: flex; flex-wrap: wrap; gap: 10px; }
.up-media__input { display: none; }
.up-media__item { position: relative; flex: 0 0 auto; }
.up-media__trigger, .up-media__preview { display: inline-flex; width: 100%; height: 100%; align-items: center; justify-content: center; overflow: hidden; padding: 0; color: var(--el-text-color-secondary); background: var(--el-fill-color-lighter); border: 1px dashed var(--el-border-color); border-radius: 8px; cursor: pointer; }
.up-media__trigger:hover { color: var(--el-color-primary); border-color: var(--el-color-primary); }
.up-media__preview { border-style: solid; }
.up-media__preview img { width: 100%; height: 100%; object-fit: cover; }
.up-media__placeholder { width: 28px; color: var(--el-text-color-placeholder); }
.up-media__clear { position: absolute; top: -8px; right: -8px; z-index: 1; display: inline-flex; width: 20px; height: 20px; align-items: center; justify-content: center; padding: 0; color: var(--el-color-danger); background: var(--el-bg-color); border: 0; border-radius: 50%; cursor: pointer; }
.is-disabled { opacity: .55; }
</style>
