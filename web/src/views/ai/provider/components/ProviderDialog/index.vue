<!-- 组件：AI 供应商新增/编辑弹窗 -->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules } from 'element-plus'

import type { AiProvider, ProviderOptions } from '@/api/ai/provider'
import AppDialog from '@/components/AppDialog/index.vue'
import type { ProviderForm } from './types'

defineOptions({ name: 'AIProviderDialog' })

const props = defineProps<{
  model: AiProvider | null
  options: ProviderOptions
  submitting: boolean
  error?: string
}>()
const emit = defineEmits<{ save: [payload: ProviderForm] }>()
const visible = defineModel<boolean>({ required: true })
const { t } = useI18n()

const formRef = ref<FormInstance>()
const form = ref<ProviderForm>(emptyForm())
const title = computed(() =>
  props.model === null ? t('ai.provider.createTitle') : t('ai.provider.editTitle'),
)
const rules = computed<FormRules<ProviderForm>>(() => ({
  name: [{ required: true, message: t('ai.provider.nameRequired'), trigger: 'blur' }],
  baseUrl: [
    { required: true, message: t('ai.provider.baseUrlRequired'), trigger: 'blur' },
    { pattern: /^https:\/\/[^\s]+$/, message: t('ai.provider.baseUrlRule'), trigger: 'blur' },
  ],
  apiKey:
    props.model === null
      ? [{ required: true, message: t('ai.provider.apiKeyRequired'), trigger: 'blur' }]
      : [],
  tokenRateMultiplier: [
    { required: true, message: t('ai.provider.tokenRateRequired'), trigger: 'blur' },
    {
      pattern: /^(?:100(?:\.0{1,6})?|\d{1,2}(?:\.\d{1,6})?)$/,
      message: t('ai.provider.tokenRateRule'),
      trigger: 'blur',
    },
  ],
}))

function emptyForm(): ProviderForm {
  return {
    name: '',
    vendor: 1,
    protocol: 1,
    baseUrl: 'https://',
    apiKey: '',
    timeoutMs: 60000,
    sortOrder: 0,
    tokenRateMultiplier: '1.0',
    status: 0,
  }
}

watch(
  () => [visible.value, props.model?.id] as const,
  () => {
    if (!visible.value) return
    if (props.model === null) {
      form.value = emptyForm()
    } else {
      form.value = {
        name: props.model.name,
        vendor: props.model.vendor,
        protocol: props.model.protocol,
        baseUrl: props.model.baseUrl,
        apiKey: '',
        timeoutMs: props.model.timeoutMs,
        sortOrder: props.model.sortOrder,
        tokenRateMultiplier: props.model.tokenRateMultiplier,
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
  emit('save', {
    ...form.value,
    name: form.value.name.trim(),
    baseUrl: form.value.baseUrl.trim(),
    apiKey: form.value.apiKey.trim(),
    tokenRateMultiplier: form.value.tokenRateMultiplier.trim(),
  })
}
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="title"
    width="660px"
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
      class="ai-provider-dialog"
      @submit.prevent="submit"
    >
      <div class="ai-provider-dialog__grid">
        <el-form-item prop="name" :label="t('ai.provider.name')">
          <el-input
            v-model="form.name"
            maxlength="128"
            :placeholder="t('ai.provider.namePlaceholder')"
            data-testid="ai-provider-name"
          />
        </el-form-item>
        <el-form-item prop="vendor" :label="t('ai.provider.vendor')">
          <el-select v-model="form.vendor" class="ai-provider-dialog__select">
            <el-option
              v-for="item in options.vendors"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="protocol" :label="t('ai.provider.protocol')">
          <el-select v-model="form.protocol" class="ai-provider-dialog__select">
            <el-option
              v-for="item in options.protocols"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="status" :label="t('ai.provider.status')">
          <el-select v-model="form.status" class="ai-provider-dialog__select">
            <el-option
              v-for="item in options.statuses"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
      </div>

      <el-form-item prop="tokenRateMultiplier" :label="t('ai.provider.tokenRateMultiplier')">
        <el-input
          v-model="form.tokenRateMultiplier"
          maxlength="12"
          :placeholder="t('ai.provider.tokenRatePlaceholder')"
          data-testid="ai-provider-token-rate"
        >
          <template #append>x</template>
        </el-input>
        <p class="ai-provider-dialog__hint">
          {{ t('ai.provider.tokenRateHint') }}
        </p>
      </el-form-item>

      <el-form-item prop="baseUrl" :label="t('ai.provider.baseUrl')">
        <el-input
          v-model="form.baseUrl"
          maxlength="2048"
          placeholder="https://api.example.com/v1"
          data-testid="ai-provider-base-url"
        />
      </el-form-item>

      <el-form-item prop="apiKey" :label="t('ai.provider.apiKey')">
        <el-input
          v-model="form.apiKey"
          type="password"
          show-password
          autocomplete="new-password"
          :placeholder="
            model === null
              ? t('ai.provider.apiKeyPlaceholder')
              : t('ai.provider.apiKeyEditPlaceholder')
          "
          data-testid="ai-provider-api-key"
        />
        <p class="ai-provider-dialog__hint">
          {{ model === null ? t('ai.provider.apiKeyHint') : t('ai.provider.apiKeyEditHint') }}
        </p>
      </el-form-item>

      <div class="ai-provider-dialog__grid">
        <el-form-item prop="timeoutMs" :label="t('ai.provider.timeoutMs')">
          <el-input-number
            v-model="form.timeoutMs"
            :min="1000"
            :max="300000"
            :step="1000"
            controls-position="right"
            class="ai-provider-dialog__number"
          />
        </el-form-item>
        <el-form-item prop="sortOrder" :label="t('ai.provider.sortOrder')">
          <el-input-number
            v-model="form.sortOrder"
            :min="0"
            :max="1000000"
            :step="10"
            controls-position="right"
            class="ai-provider-dialog__number"
          />
        </el-form-item>
      </div>

      <el-alert v-if="error" :title="error" type="error" :closable="false" />
    </el-form>
    <template #footer>
      <el-button :disabled="submitting" @click="visible = false">
        {{ t('common.cancel') }}
      </el-button>
      <el-button
        type="primary"
        :loading="submitting"
        data-testid="ai-provider-save"
        @click="submit"
      >
        {{ t('common.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.ai-provider-dialog {
  display: flex;
  flex-direction: column;
  gap: 2px;

  &__grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 16px;
  }

  &__select,
  &__number {
    width: 100%;
  }

  &__hint {
    margin: 4px 0 0;
    color: var(--admin-text-muted);
    font-size: 12px;
    line-height: 1.6;
  }
}

@media (max-width: 680px) {
  .ai-provider-dialog__grid {
    grid-template-columns: 1fr;
  }
}
</style>
