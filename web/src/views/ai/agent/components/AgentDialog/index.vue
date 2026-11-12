<!-- 组件：AI 智能体新增/编辑弹窗 -->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CascaderProps, FormInstance, FormRules } from 'element-plus'

import type { AgentOptions, AgentProviderModelOption, AiAgent } from '@/api/ai/agent'
import AppDialog from '@/components/AppDialog/index.vue'
import { emptyAgentForm, type AgentForm } from './types'

defineOptions({ name: 'AIAgentDialog' })

const props = defineProps<{
  model: AiAgent | null
  options: AgentOptions
  modelOptions: AgentProviderModelOption[]
  submitting: boolean
  error?: string
}>()
const emit = defineEmits<{ save: [payload: AgentForm] }>()
const visible = defineModel<boolean>({ required: true })
const { t } = useI18n()

const formRef = ref<FormInstance>()
const form = ref<AgentForm>(emptyAgentForm())
const cascaderProps: CascaderProps = {
  value: 'value',
  label: 'label',
  children: 'children',
  emitPath: false,
  expandTrigger: 'hover',
}
const title = computed(() =>
  props.model === null ? t('ai.agent.createTitle') : t('ai.agent.editTitle'),
)
function availableIDsFor(type: number): Set<number> {
  return new Set(
    props.modelOptions.flatMap((provider) =>
      provider.children.filter((item) => item.modelKind === type).map((item) => item.value),
    ),
  )
}
const filteredModelOptions = computed(() =>
  props.modelOptions
    .map((provider) => ({
      ...provider,
      children: provider.children.filter((item) => item.modelKind === form.value.type),
    }))
    .filter((provider) => provider.children.length > 0),
)
const availableModelIDs = computed(() => availableIDsFor(form.value.type))
const currentModelUnavailable = computed(
  () =>
    props.model !== null &&
    !props.model.modelAvailable &&
    !availableModelIDs.value.has(props.model.providerModelId),
)
const rules = computed<FormRules<AgentForm>>(() => ({
  name: [
    { required: true, message: t('ai.agent.nameRequired'), trigger: 'blur' },
    { max: 128, message: t('ai.agent.nameMax'), trigger: 'blur' },
  ],
  type: [{ required: true, message: t('ai.agent.typeRequired'), trigger: 'change' }],
  scene: [{ required: true, message: t('ai.agent.sceneRequired'), trigger: 'change' }],
  providerModelId: [{ required: true, message: t('ai.agent.modelRequired'), trigger: 'change' }],
  systemPrompt: [
    { required: true, message: t('ai.agent.systemPromptRequired'), trigger: 'blur' },
    { max: 20000, message: t('ai.agent.systemPromptMax'), trigger: 'blur' },
  ],
}))

watch(
  () => form.value.type,
  (type) => {
    if (
      form.value.providerModelId !== null &&
      !availableIDsFor(type).has(form.value.providerModelId)
    ) {
      form.value.providerModelId = null
    }
    void nextTick(() => formRef.value?.clearValidate('providerModelId'))
  },
)

watch(
  () => [visible.value, props.model?.id] as const,
  () => {
    if (!visible.value) return
    form.value = emptyAgentForm()
    if (props.model !== null) {
      const boundModelAvailable = availableIDsFor(props.model.type).has(props.model.providerModelId)
      form.value = {
        name: props.model.name,
        description: props.model.description,
        systemPrompt: props.model.systemPrompt,
        scene: props.model.scene,
        type: props.model.type,
        providerModelId: boundModelAvailable ? props.model.providerModelId : null,
        status: props.model.status,
      }
    }
    void nextTick(() => formRef.value?.clearValidate())
  },
  { immediate: true },
)

async function submit(): Promise<void> {
  if (props.submitting || formRef.value === undefined) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  if (form.value.providerModelId === null) return
  emit('save', {
    name: form.value.name.trim(),
    description: form.value.description.trim(),
    systemPrompt: form.value.systemPrompt.trim(),
    scene: form.value.scene,
    type: form.value.type,
    providerModelId: form.value.providerModelId,
    status: form.value.status,
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
      <el-alert v-if="error" :title="error" type="error" :closable="false" />

      <div class="ai-agent-dialog__grid">
        <el-form-item :label="t('ai.agent.name')" prop="name">
          <el-input
            v-model="form.name"
            maxlength="128"
            show-word-limit
            :placeholder="t('ai.agent.namePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('ai.agent.type')" prop="type">
          <el-select v-model="form.type" :placeholder="t('ai.agent.typePlaceholder')">
            <el-option
              v-for="item in options.types"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
      </div>

      <div class="ai-agent-dialog__grid">
        <el-form-item :label="t('ai.agent.scene')" prop="scene">
          <el-select v-model="form.scene" :placeholder="t('ai.agent.scenePlaceholder')">
            <el-option
              v-for="item in options.scenes"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('ai.agent.status')" prop="status">
          <el-select v-model="form.status">
            <el-option
              v-for="item in options.statuses"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
      </div>

      <el-form-item :label="t('ai.agent.model')" prop="providerModelId">
        <div class="ai-agent-dialog__model">
          <el-cascader
            v-model="form.providerModelId"
            :options="filteredModelOptions"
            :props="cascaderProps"
            :placeholder="t('ai.agent.modelPlaceholder')"
            :no-data-text="t('ai.agent.noModels')"
            filterable
            clearable
          />
        </div>
      </el-form-item>

      <el-alert
        v-if="currentModelUnavailable"
        :title="t('ai.agent.modelUnavailable')"
        type="warning"
        :closable="false"
        show-icon
      />

      <el-form-item :label="t('ai.agent.description')" prop="description">
        <el-input
          v-model="form.description"
          type="textarea"
          maxlength="500"
          show-word-limit
          :autosize="{ minRows: 2, maxRows: 4 }"
          :placeholder="t('ai.agent.descriptionPlaceholder')"
        />
      </el-form-item>

      <el-form-item :label="t('ai.agent.systemPrompt')" prop="systemPrompt">
        <el-input
          v-model="form.systemPrompt"
          type="textarea"
          maxlength="20000"
          show-word-limit
          :autosize="{ minRows: 8, maxRows: 16 }"
          :placeholder="t('ai.agent.systemPromptPlaceholder')"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button :disabled="submitting" @click="visible = false">
        {{ t('common.cancel') }}
      </el-button>
      <el-button type="primary" :loading="submitting" data-testid="ai-agent-save" @click="submit">
        {{ t('common.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.ai-agent-dialog {
  &__grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 180px;
    gap: 16px;
  }

  &__model {
    width: 100%;
    min-width: 0;

    :deep(.el-cascader) {
      width: 100%;
    }
  }
}

@media (max-width: 680px) {
  .ai-agent-dialog__grid {
    grid-template-columns: 1fr;
  }
}
</style>
