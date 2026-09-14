<script setup lang="ts">
import { onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import type { IDomEditor } from '@wangeditor-next/editor'
import { Editor, Toolbar } from '@wangeditor-next/editor-for-vue'
import '@wangeditor-next/editor/dist/css/style.css'
import { assertSafeMailHtml, readMailBody, updateMailBody } from './mailHtmlDocument'

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const mode = ref<'rich' | 'source'>('rich')
const source = ref(props.modelValue)
const body = ref(readMailBody(props.modelValue))
const editor = shallowRef<IDomEditor>()
const userEditing = ref(false)
const editorConfig = { placeholder: '编辑邮件正文', MENU_CONF: {} }
const toolbarConfig = { excludeKeys: ['group-video', 'insertImage', 'insertVideo'] }
watch(
  () => props.modelValue,
  (next) => {
    source.value = next
    if (mode.value === 'rich') body.value = readMailBody(next)
  },
)
onBeforeUnmount(() => editor.value?.destroy())

function markUserEditing(): void {
  userEditing.value = true
}
function updateRichText(currentEditor: IDomEditor): void {
  if (!userEditing.value) return
  const nextBody = currentEditor.getHtml()
  if (nextBody === body.value) return
  body.value = nextBody
  const next = updateMailBody(source.value, nextBody)
  source.value = next
  emit('update:modelValue', next)
}
function switchMode(next: 'rich' | 'source'): void {
  if (next === 'source') {
    source.value = props.modelValue
    mode.value = next
    return
  }
  assertSafeMailHtml(source.value)
  userEditing.value = false
  body.value = readMailBody(source.value)
  emit('update:modelValue', source.value)
  mode.value = next
}
function updateSource(next: string): void {
  source.value = next
  emit('update:modelValue', next)
}
</script>
<template>
  <div class="mail-html-editor">
    <div class="mail-html-editor__tabs">
      <el-radio-group :model-value="mode" @update:model-value="switchMode">
        <el-radio-button label="rich">富文本</el-radio-button>
        <el-radio-button label="source">HTML 源码</el-radio-button>
      </el-radio-group>
      <span>富文本仅修改邮件正文，完整结构请使用 HTML 源码</span>
    </div>
    <div
      v-if="mode === 'rich'"
      class="mail-html-editor__canvas"
      @pointerdown="markUserEditing"
      @keydown="markUserEditing"
    >
      <Toolbar
        class="mail-html-editor__toolbar"
        :editor="editor"
        :default-config="toolbarConfig"
        mode="default"
      />
      <Editor
        class="mail-html-editor__content"
        :model-value="body"
        :default-config="editorConfig"
        mode="default"
        @on-created="editor = $event"
        @on-change="updateRichText"
      />
    </div>
    <el-input
      v-else
      class="mail-html-editor__source"
      :model-value="source"
      type="textarea"
      :rows="18"
      @update:model-value="updateSource"
    />
  </div>
</template>
<style scoped>
.mail-html-editor {
  display: grid;
  gap: 12px;
  min-width: 0;
}
.mail-html-editor__tabs {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.mail-html-editor__tabs span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.mail-html-editor__canvas {
  overflow: hidden;
  border: 1px solid var(--el-border-color);
  border-radius: var(--el-border-radius-base);
  background: var(--el-bg-color);
}
.mail-html-editor__toolbar {
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.mail-html-editor__content {
  min-height: 280px;
  max-height: 360px;
  overflow-y: auto;
}
.mail-html-editor__source {
  width: 100%;
}
.mail-html-editor__source :deep(textarea) {
  min-height: 360px !important;
  font-family: Consolas, 'SFMono-Regular', monospace;
  font-size: 13px;
  line-height: 1.65;
}
</style>
