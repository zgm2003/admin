<script lang="ts">
export const notificationToolbarKeys = [
  'bold',
  'italic',
  'underline',
  'headerSelect',
  'bulletedList',
  'numberedList',
  'insertLink',
  'undo',
  'redo',
  'clearStyle',
] as const

export const notificationEditorConfig = {
  MENU_CONF: {
    headerSelect: {
      title: 'header',
      options: [
        { value: 'paragraph', text: 'paragraph' },
        { value: 'h2', text: 'H2' },
        { value: 'h3', text: 'H3' },
      ],
    },
    insertLink: {
      checkLink: (link: string): boolean => {
        try {
          return new URL(link).protocol === 'https:'
        } catch {
          return false
        }
      },
    },
  },
}
</script>

<script setup lang="ts">
import { computed, onBeforeUnmount, shallowRef } from 'vue'
import type { IDomEditor } from '@wangeditor-next/editor'
import { Editor, Toolbar } from '@wangeditor-next/editor-for-vue'
import '@wangeditor-next/editor/dist/css/style.css'

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const editor = shallowRef<IDomEditor>()
const value = computed({
  get: () => props.modelValue,
  set: (next: string) => {
    if (next !== props.modelValue) emit('update:modelValue', next)
  },
})
const editorConfig = { ...notificationEditorConfig, placeholder: '' }
const toolbarConfig = { toolbarKeys: [...notificationToolbarKeys] }

onBeforeUnmount(() => editor.value?.destroy())
</script>

<template>
  <div class="notification-editor">
    <Toolbar :editor="editor" :default-config="toolbarConfig" mode="default" />
    <Editor
      v-model="value"
      class="notification-editor__body"
      :default-config="editorConfig"
      mode="default"
      @on-created="editor = $event"
    />
  </div>
</template>

<style scoped>
.notification-editor {
  width: 100%;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--el-border-color);
  border-radius: var(--el-border-radius-base);
  background: var(--el-bg-color);
}
.notification-editor :deep(.w-e-toolbar) {
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.notification-editor__body {
  min-height: 220px;
  max-height: 320px;
  overflow-y: auto;
}
</style>
