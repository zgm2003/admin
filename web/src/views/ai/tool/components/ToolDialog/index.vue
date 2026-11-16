<!-- 组件：AI 工具新增/编辑弹窗 -->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules } from 'element-plus'

import type { AiTool } from '@/api/ai/tool'
import AppDialog from '@/components/AppDialog/index.vue'
import type { ToolForm, ToolFormPayload } from './types'

defineOptions({ name: 'AIToolDialog' })

const props = defineProps<{
  model: AiTool | null
  submitting: boolean
  error?: string
}>()
const emit = defineEmits<{ save: [payload: ToolFormPayload] }>()
const visible = defineModel<boolean>({ required: true })
const { t } = useI18n()

const formRef = ref<FormInstance>()
const localError = ref('')
const form = ref<ToolForm>(emptyForm())
const isBuiltin = computed(() => props.model?.kind === 1)
const title = computed(() =>
  props.model === null ? t('ai.tool.createTitle') : t('ai.tool.editTitle'),
)
const methodOptions = [
  { value: 'GET', label: 'GET' },
  { value: 'POST', label: 'POST' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'DELETE', label: 'DELETE' },
]
const rules = computed<FormRules<ToolForm>>(() => ({
  code: [
    { required: true, message: t('ai.tool.codeRequired'), trigger: 'blur' },
    {
      pattern: /^[a-z][a-z0-9_.-]{0,63}$/,
      message: t('ai.tool.codePattern'),
      trigger: 'blur',
    },
  ],
  name: [
    { required: true, message: t('ai.tool.nameRequired'), trigger: 'blur' },
    { max: 128, message: t('ai.tool.nameMax'), trigger: 'blur' },
  ],
  url: [
    {
      validator: (_rule: unknown, value: string, callback: (error?: Error) => void) => {
        if (isBuiltin.value || /^https?:\/\/[^\s]+$/i.test(value.trim())) {
          callback()
          return
        }
        callback(new Error(t('ai.tool.urlInvalid')))
      },
      trigger: 'blur',
    },
  ],
}))

function emptyForm(): ToolForm {
  return {
    code: '',
    name: '',
    description: '',
    category: 'internal',
    method: 'POST',
    url: '',
    timeoutMs: 10000,
    status: 1,
    sortOrder: 0,
    parameters: JSON.stringify({ type: 'object', properties: {} }, null, 2),
  }
}

watch(
  () => [visible.value, props.model?.id] as const,
  () => {
    if (!visible.value) return
    localError.value = ''
    form.value = emptyForm()
    if (props.model !== null) {
      form.value = {
        code: props.model.code,
        name: props.model.name,
        description: props.model.description,
        category: props.model.category,
        method: props.model.method || 'POST',
        url: props.model.url,
        timeoutMs: props.model.timeoutMs || 10000,
        status: props.model.status,
        sortOrder: props.model.sortOrder,
        parameters: JSON.stringify(
          props.model.parameters ?? { type: 'object', properties: {} },
          null,
          2,
        ),
      }
    }
    void nextTick(() => formRef.value?.clearValidate())
  },
  { immediate: true },
)

async function submit(): Promise<void> {
  if (props.submitting || formRef.value === undefined) return
  localError.value = ''
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  let parameters: Record<string, unknown>
  try {
    const parsed: unknown = JSON.parse(form.value.parameters || '{}')
    if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') {
      throw new Error('parameters must be an object')
    }
    parameters = parsed as Record<string, unknown>
  } catch {
    localError.value = t('ai.tool.parametersInvalid')
    return
  }
  emit('save', {
    code: form.value.code.trim(),
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    category: form.value.category.trim() || 'internal',
    method: form.value.method,
    url: form.value.url.trim(),
    timeoutMs: form.value.timeoutMs,
    status: form.value.status,
    sortOrder: form.value.sortOrder,
    parameters,
  })
}
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="title"
    width="720px"
    :close-on-click-modal="false"
    :close-on-press-escape="!submitting"
    :show-close="!submitting"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      :disabled="submitting"
      label-position="top"
      @submit.prevent
    >
      <el-alert
        v-if="error || localError"
        :title="error || localError"
        type="error"
        :closable="false"
      />
      <el-alert
        v-if="model === null"
        :title="t('ai.tool.httpHint')"
        type="info"
        :closable="false"
        show-icon
      />
      <el-alert
        v-if="isBuiltin"
        :title="t('ai.tool.builtinEditHint')"
        type="info"
        :closable="false"
        show-icon
      />

      <div class="ai-tool-dialog__grid">
        <el-form-item :label="t('ai.tool.code')" prop="code">
          <el-input
            v-model="form.code"
            :disabled="model !== null"
            maxlength="64"
            show-word-limit
            :placeholder="t('ai.tool.codePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('ai.tool.category')" prop="category">
          <el-input
            v-model="form.category"
            maxlength="32"
            :placeholder="t('ai.tool.categoryPlaceholder')"
          />
        </el-form-item>
      </div>

      <el-form-item :label="t('ai.tool.name')" prop="name">
        <el-input
          v-model="form.name"
          maxlength="128"
          show-word-limit
          :placeholder="t('ai.tool.namePlaceholder')"
        />
      </el-form-item>

      <el-form-item :label="t('ai.tool.description')" prop="description">
        <el-input
          v-model="form.description"
          type="textarea"
          maxlength="500"
          show-word-limit
          :autosize="{ minRows: 2, maxRows: 4 }"
          :placeholder="t('ai.tool.descriptionPlaceholder')"
        />
      </el-form-item>

      <template v-if="!isBuiltin">
        <div class="ai-tool-dialog__grid ai-tool-dialog__grid--request">
          <el-form-item :label="t('ai.tool.method')" prop="method">
            <el-select-v2 v-model="form.method" :options="methodOptions" />
          </el-form-item>
          <el-form-item :label="t('ai.tool.url')" prop="url">
            <el-input v-model="form.url" :placeholder="t('ai.tool.urlPlaceholder')" />
          </el-form-item>
        </div>

        <el-form-item :label="t('ai.tool.parameters')" prop="parameters">
          <el-input
            v-model="form.parameters"
            type="textarea"
            :autosize="{ minRows: 6, maxRows: 14 }"
            spellcheck="false"
            class="ai-tool-dialog__json"
            :placeholder="t('ai.tool.parametersPlaceholder')"
          />
          <span class="ai-tool-dialog__hint">{{ t('ai.tool.parametersHint') }}</span>
        </el-form-item>
      </template>

      <div
        class="ai-tool-dialog__grid ai-tool-dialog__grid--meta"
        :class="{ 'ai-tool-dialog__grid--meta-builtin': isBuiltin }"
      >
        <el-form-item v-if="!isBuiltin" :label="t('ai.tool.timeout')" prop="timeoutMs">
          <el-input-number
            v-model="form.timeoutMs"
            :min="1000"
            :max="60000"
            :step="1000"
            controls-position="right"
            class="ai-tool-dialog__number"
          />
        </el-form-item>
        <el-form-item :label="t('ai.tool.sortOrder')" prop="sortOrder">
          <el-input-number
            v-model="form.sortOrder"
            :min="0"
            :max="9999"
            controls-position="right"
            class="ai-tool-dialog__number"
          />
        </el-form-item>
        <el-form-item :label="t('ai.tool.status')" prop="status">
          <el-switch
            v-model="form.status"
            :active-value="1"
            :inactive-value="0"
            :active-text="t('ai.tool.enable')"
            :inactive-text="t('ai.tool.disable')"
          />
        </el-form-item>
      </div>
    </el-form>

    <template #footer>
      <el-button :disabled="submitting" @click="visible = false">
        {{ t('common.cancel') }}
      </el-button>
      <el-button type="primary" :loading="submitting" data-testid="ai-tool-save" @click="submit">
        {{ t('common.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.ai-tool-dialog {
  &__grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 16px;
  }

  &__grid--request {
    grid-template-columns: 160px minmax(0, 1fr);
  }

  &__grid--meta {
    grid-template-columns: 180px 180px minmax(0, 1fr);
    align-items: end;
  }

  &__grid--meta-builtin {
    grid-template-columns: 180px minmax(0, 1fr);
  }

  &__number {
    width: 100%;
  }

  &__hint {
    display: block;
    margin-top: 5px;
    color: var(--admin-text-muted);
    font-size: 12px;
    line-height: 1.5;
  }

  &__json :deep(textarea) {
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px;
    line-height: 1.6;
  }
}

@media (max-width: 680px) {
  .ai-tool-dialog__grid,
  .ai-tool-dialog__grid--request,
  .ai-tool-dialog__grid--meta {
    grid-template-columns: 1fr;
  }
}
</style>
