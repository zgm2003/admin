<!-- 组件：官方模型人民币价编辑弹窗 -->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules } from 'element-plus'

import type { OfficialModel } from '@/api/ai/officialModel'
import AppDialog from '@/components/AppDialog/index.vue'
import type { PriceForm } from './types'

defineOptions({ name: 'OfficialModelPriceDialog' })

const props = defineProps<{
  model: OfficialModel | null
  submitting: boolean
  error?: string
}>()
const emit = defineEmits<{ save: [payload: PriceForm] }>()
const visible = defineModel<boolean>({ required: true })
const { t, locale } = useI18n()

const formRef = ref<FormInstance>()
const form = ref<PriceForm>({ inputCny: '', outputCny: '', cacheReadCny: '', cacheWriteCny: '' })
const rules = computed<FormRules<PriceForm>>(() => ({
  inputCny: [
    { required: true, message: t('ai.officialModel.priceRequired'), trigger: 'blur' },
    { pattern: /^\d+(\.\d{1,6})?$/, message: t('ai.officialModel.priceRule'), trigger: 'blur' },
  ],
  outputCny: [
    { required: true, message: t('ai.officialModel.priceRequired'), trigger: 'blur' },
    { pattern: /^\d+(\.\d{1,6})?$/, message: t('ai.officialModel.priceRule'), trigger: 'blur' },
  ],
  cacheReadCny: [
    {
      pattern: /^\d+(\.\d{1,6})?$/,
      message: t('ai.officialModel.priceRule'),
      trigger: 'blur',
    },
  ],
  cacheWriteCny: [
    {
      pattern: /^\d+(\.\d{1,6})?$/,
      message: t('ai.officialModel.priceRule'),
      trigger: 'blur',
    },
  ],
}))

watch(
  () => [visible.value, props.model?.id] as const,
  () => {
    if (!visible.value || props.model === null) return
    form.value = {
      inputCny: props.model.inputCnyPerMillion,
      outputCny: props.model.outputCnyPerMillion,
      cacheReadCny: props.model.cacheReadCnyPerMillion ?? '',
      cacheWriteCny: props.model.cacheWriteCnyPerMillion ?? '',
    }
    void nextTick(() => formRef.value?.clearValidate())
  },
  { immediate: true },
)

function formatVerifiedAt(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium' }).format(date)
}

function price(value: string, currency: 'USD' | 'CNY'): string {
  return `${currency === 'USD' ? '$' : '¥'}${value}/M`
}

async function submit(): Promise<void> {
  if (props.submitting || props.model === null || formRef.value === undefined) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  emit('save', {
    inputCny: form.value.inputCny.trim(),
    outputCny: form.value.outputCny.trim(),
    cacheReadCny: form.value.cacheReadCny.trim(),
    cacheWriteCny: form.value.cacheWriteCny.trim(),
  })
}
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="t('ai.officialModel.editTitle')"
    width="600px"
    :close-on-click-modal="false"
    :close-on-press-escape="!submitting"
    :show-close="!submitting"
  >
    <div v-if="model" class="official-model-price-dialog__edit">
      <div class="official-model-price-dialog__edit-head">
        <div>
          <span class="official-model-price-dialog__edit-vendor">{{ model.vendorName }}</span>
          <strong>{{ model.displayName }}</strong>
          <code v-if="model.displayName !== model.modelKey">{{ model.modelKey }}</code>
        </div>
        <span class="official-model-price-dialog__edit-note">
          {{ t('ai.officialModel.editHint') }}
          <br />
          {{ t('ai.officialModel.verifiedAt') }}: {{ formatVerifiedAt(model.verifiedAt) }}
        </span>
      </div>
      <div class="official-model-price-dialog__official-grid">
        <div>
          <span>{{ t('ai.officialModel.officialInput') }}</span>
          <strong>{{ price(model.inputUsdPerMillion, 'USD') }}</strong>
        </div>
        <div>
          <span>{{ t('ai.officialModel.officialOutput') }}</span>
          <strong>{{ price(model.outputUsdPerMillion, 'USD') }}</strong>
        </div>
      </div>
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        :disabled="submitting"
        label-position="top"
        class="official-model-price-dialog__form"
        @submit.prevent="submit"
      >
        <div class="official-model-price-dialog__form-grid">
          <el-form-item prop="inputCny" :label="t('ai.officialModel.inputCny')">
            <el-input
              v-model="form.inputCny"
              inputmode="decimal"
              :placeholder="t('ai.officialModel.pricePlaceholder')"
              data-testid="ai-official-model-input-cny"
            >
              <template #append>¥ / M</template>
            </el-input>
          </el-form-item>
          <el-form-item prop="outputCny" :label="t('ai.officialModel.outputCny')">
            <el-input
              v-model="form.outputCny"
              inputmode="decimal"
              :placeholder="t('ai.officialModel.pricePlaceholder')"
              data-testid="ai-official-model-output-cny"
            >
              <template #append>¥ / M</template>
            </el-input>
          </el-form-item>
          <el-form-item prop="cacheReadCny" :label="t('ai.officialModel.cacheReadCny')">
            <el-input
              v-model="form.cacheReadCny"
              inputmode="decimal"
              :placeholder="t('ai.officialModel.cachePlaceholder')"
              data-testid="ai-official-model-cache-read-cny"
            >
              <template #append>¥ / M</template>
            </el-input>
          </el-form-item>
          <el-form-item prop="cacheWriteCny" :label="t('ai.officialModel.cacheWriteCny')">
            <el-input
              v-model="form.cacheWriteCny"
              inputmode="decimal"
              :placeholder="t('ai.officialModel.cachePlaceholder')"
              data-testid="ai-official-model-cache-write-cny"
            >
              <template #append>¥ / M</template>
            </el-input>
          </el-form-item>
        </div>
        <p class="official-model-price-dialog__cache-note">
          {{ t('ai.officialModel.cacheHint') }}
        </p>
        <p class="official-model-price-dialog__ratio-note">
          {{ t('ai.officialModel.ratioNote') }}
        </p>
        <el-alert v-if="error" :title="error" type="error" :closable="false" />
      </el-form>
    </div>
    <template #footer>
      <el-button :disabled="submitting" @click="visible = false">
        {{ t('common.cancel') }}
      </el-button>
      <el-button
        type="primary"
        :loading="submitting"
        data-testid="ai-official-model-save"
        @click="submit"
      >
        {{ t('common.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.official-model-price-dialog {
  &__edit {
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  &__edit-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 20px;

    > div {
      display: flex;
      min-width: 0;
      flex-direction: column;
      gap: 3px;
    }

    strong {
      color: var(--admin-text);
      font-size: 16px;
      font-weight: 700;
    }

    code {
      overflow: hidden;
      color: var(--admin-text-muted);
      font-family: var(--admin-font-data, ui-monospace, monospace);
      font-size: 11px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  &__edit-vendor {
    color: var(--el-color-primary);
    font-size: 10px;
    font-weight: 750;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  &__edit-note {
    max-width: 240px;
    color: var(--admin-text-muted);
    font-size: 12px;
    line-height: 1.7;
    text-align: right;
  }

  &__official-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;

    > div {
      display: flex;
      flex-direction: column;
      gap: 4px;
      padding: 12px 14px;
      background: var(--admin-surface-soft);
      border: 1px solid var(--admin-border);
      border-radius: 8px;
    }

    span {
      color: var(--admin-text-muted);
      font-size: 11px;
    }

    strong {
      color: var(--admin-text);
      font-family: var(--admin-font-data, ui-monospace, monospace);
      font-size: 14px;
      font-weight: 650;
    }
  }

  &__form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 16px;
  }

  &__cache-note {
    margin: -8px 0 0;
    color: var(--admin-text-muted);
    font-size: 12px;
    line-height: 1.6;
  }

  &__ratio-note {
    margin: -2px 0 0;
    color: var(--admin-text-muted);
    font-size: 12px;
    line-height: 1.7;
  }
}

@media (max-width: 680px) {
  .official-model-price-dialog {
    &__edit-head {
      flex-direction: column;
      gap: 8px;
    }

    &__edit-note {
      max-width: none;
      text-align: left;
    }

    &__form-grid,
    &__official-grid {
      grid-template-columns: 1fr;
    }
  }
}
</style>
