<!-- 组件：我的文件选择弹窗 -->
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import FileBrowser from '@/components/FileBrowser/index.vue'
import type { StorageFile } from '@/api/storage/file'
const visible = defineModel<boolean>({ required: true })
withDefaults(defineProps<{ purpose?: string }>(), { purpose: '' })
const emit = defineEmits<{ select: [file: StorageFile] }>()
const { t } = useI18n()
function select(file: StorageFile): void {
  emit('select', file)
  visible.value = false
}
</script>
<template>
  <AppDialog v-model="visible" :title="t('storage.file.selectTitle')" width="1000px" height="650px"
    ><FileBrowser v-if="visible" :purpose="purpose" selection @select="select"
  /></AppDialog>
</template>
