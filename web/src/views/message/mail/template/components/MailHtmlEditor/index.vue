<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
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
const editorConfig = { placeholder: '编辑邮件正文', MENU_CONF: {} }
const toolbarConfig = { excludeKeys: ['group-video', 'insertImage', 'insertVideo'] }
const value = computed({
  get: () => body.value,
  set: (next: string) => {
    body.value = next
    emitBody()
  },
})
watch(
  () => props.modelValue,
  (next) => {
    source.value = next
    if (mode.value === 'rich') body.value = readMailBody(next)
  },
)
onBeforeUnmount(() => editor.value?.destroy())

function emitBody(): void {
  const next = updateMailBody(source.value, body.value)
  emit('update:modelValue', next)
}
function switchMode(next: 'rich' | 'source'): void {
  if (next === 'source') {
    source.value = props.modelValue
    mode.value = next
    return
  }
  assertSafeMailHtml(source.value)
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
    <el-radio-group :model-value="mode" @update:model-value="switchMode">
      <el-radio-button label="rich">富文本</el-radio-button>
      <el-radio-button label="source">HTML 源码</el-radio-button>
    </el-radio-group>
    <template v-if="mode === 'rich'">
      <Toolbar :editor="editor" :default-config="toolbarConfig" mode="default" />
      <Editor
        v-model="value"
        :default-config="editorConfig"
        mode="default"
        @on-created="editor = $event"
      />
    </template>
    <el-input
      v-else
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
}
</style>
