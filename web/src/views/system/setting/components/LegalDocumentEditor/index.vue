<script lang="ts">
export const legalDocumentToolbarKeys = [
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

export const legalDocumentEditorConfig = {
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

defineOptions({ name: 'LegalDocumentEditor' })

const props = defineProps<{ modelValue: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const editor = shallowRef<IDomEditor>()
const value = computed({
  get: () => props.modelValue,
  set: (next: string) => {
    if (next !== props.modelValue) emit('update:modelValue', next)
  },
})
const editorConfig = { ...legalDocumentEditorConfig, placeholder: '', readOnly: props.disabled }
const toolbarConfig = { toolbarKeys: [...legalDocumentToolbarKeys] }

onBeforeUnmount(() => editor.value?.destroy())
</script>

<template>
  <div class="legal-document-editor" :class="{ 'is-disabled': disabled }">
    <Toolbar v-if="!disabled" :editor="editor" :default-config="toolbarConfig" mode="default" />
    <Editor
      v-model="value"
      class="legal-document-editor__body"
      :default-config="editorConfig"
      mode="default"
      @on-created="editor = $event"
    />
  </div>
</template>

<style scoped>
.legal-document-editor {
  display: flex;
  width: 100%;
  min-width: 0;
  min-height: 320px;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--el-border-color);
  border-radius: var(--el-border-radius-base);
  background: var(--el-bg-color);
}

.legal-document-editor :deep(.w-e-toolbar) {
  flex: 0 0 auto;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.legal-document-editor__body {
  min-height: 0;
  flex: 1 1 auto;
  overflow-y: auto;
}

.legal-document-editor.is-disabled {
  background: var(--el-disabled-bg-color);
}
</style>
