<!-- 组件：供应商模型配置弹窗 -->

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules } from 'element-plus'

import type {
  AiProviderModel,
  ProviderModelOptions,
  ProviderModelPriceInput,
  UpdateProviderModelInput,
} from '@/api/ai/providerModel'
import AppDialog from '@/components/AppDialog/index.vue'

defineOptions({ name: 'AIProviderModelEditDialog' })

const props = defineProps<{
  model: AiProviderModel
  options: ProviderModelOptions
  submitting: boolean
  error?: string
}>()
const emit = defineEmits<{ save: [payload: UpdateProviderModelInput] }>()
const visible = defineModel<boolean>({ required: true })
const { t } = useI18n()

const formRef = ref<FormInstance>()
const localError = ref('')
const form = ref({
  displayName: '',
  modelKind: 1,
  pricingMode: 1,
  priceUnit: 1,
  officialModelId: null as number | null,
  status: 0,
})
const prices = ref<ProviderModelPriceInput[]>([])

const isTokenMode = computed(() => form.value.pricingMode === 1)
const officialOptions = computed(() =>
  props.options.officialModels.filter((item) => item.modelKind === form.value.modelKind),
)
const rules = computed<FormRules<typeof form.value>>(() => ({
  displayName: [
    { required: true, message: t('ai.providerModel.displayNameRequired'), trigger: 'blur' },
  ],
}))

watch(
  () => [visible.value, props.model.id] as const,
  () => {
    if (!visible.value) return
    form.value = {
      displayName: props.model.displayName,
      modelKind: props.model.modelKind,
      pricingMode: props.model.pricingMode,
      priceUnit: props.model.priceUnit,
      officialModelId: props.model.officialModel?.id ?? null,
      status: props.model.status,
    }
    prices.value = props.model.prices.map((price) => ({
      tierKey: price.tierKey,
      tierLabel: price.tierLabel,
      priceCny: price.priceCny,
    }))
    if (form.value.pricingMode === 2 && prices.value.length === 0) defaultTiers()
    localError.value = ''
    void nextTick(() => formRef.value?.clearValidate())
  },
  { immediate: true },
)

watch(
  () => form.value.pricingMode,
  (mode) => {
    if (mode === 1) {
      form.value.priceUnit = 1
      prices.value = []
      return
    }
    form.value.officialModelId = null
    form.value.priceUnit = unitForKind(form.value.modelKind)
    if (prices.value.length === 0) defaultTiers()
  },
)

watch(
  () => form.value.modelKind,
  (kind) => {
    const kindChanged = props.model.modelKind !== kind
    if (form.value.pricingMode === 1) {
      form.value.priceUnit = 1
      const current = form.value.officialModelId
      if (current !== null && !officialOptions.value.some((item) => item.id === current)) {
        form.value.officialModelId = null
      }
      return
    }
    form.value.priceUnit = unitForKind(kind)
    if (kindChanged) {
      prices.value = []
      defaultTiers()
    }
  },
)

function unitForKind(kind: number): number {
  if (kind === 2) return 2
  if (kind === 3) return 3
  if (kind === 4) return 5
  return 2
}

function defaultTiers(): void {
  if (form.value.modelKind === 2) {
    prices.value = [
      { tierKey: 'quality:low', tierLabel: t('ai.providerModel.qualityLow'), priceCny: '0.00' },
      {
        tierKey: 'quality:medium',
        tierLabel: t('ai.providerModel.qualityMedium'),
        priceCny: '0.00',
      },
      { tierKey: 'quality:high', tierLabel: t('ai.providerModel.qualityHigh'), priceCny: '0.00' },
    ]
    return
  }
  if (form.value.modelKind === 3) {
    prices.value = [
      { tierKey: 'resolution:480p', tierLabel: '480p', priceCny: '0.00' },
      { tierKey: 'resolution:720p', tierLabel: '720p', priceCny: '0.00' },
      { tierKey: 'resolution:1080p', tierLabel: '1080p', priceCny: '0.00' },
    ]
    return
  }
  prices.value = [
    { tierKey: 'default', tierLabel: t('ai.providerModel.defaultTier'), priceCny: '0.00' },
  ]
}

function addTier(): void {
  prices.value.push({ tierKey: '', tierLabel: '', priceCny: '0.00' })
}

function removeTier(index: number): void {
  prices.value.splice(index, 1)
}

async function submit(): Promise<void> {
  if (props.submitting || formRef.value === undefined) return
  localError.value = ''
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  if (!form.value.displayName.trim()) {
    localError.value = t('ai.providerModel.displayNameRequired')
    return
  }
  if (isTokenMode.value && form.value.officialModelId === null) {
    localError.value = t('ai.providerModel.officialRequired')
    return
  }
  if (!isTokenMode.value) {
    if (prices.value.length === 0) {
      localError.value = t('ai.providerModel.priceRequired')
      return
    }
    if (prices.value.some((price) => !price.tierKey.trim() || !price.priceCny.trim())) {
      localError.value = t('ai.providerModel.priceRowRequired')
      return
    }
  }
  emit('save', {
    displayName: form.value.displayName.trim(),
    modelKind: form.value.modelKind,
    pricingMode: form.value.pricingMode,
    priceUnit: form.value.priceUnit,
    officialModelId: form.value.officialModelId,
    status: form.value.status,
    prices: isTokenMode.value
      ? []
      : prices.value.map((price) => ({
          tierKey: price.tierKey.trim(),
          tierLabel: price.tierLabel.trim(),
          priceCny: price.priceCny.trim(),
        })),
  })
}
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="t('ai.providerModel.editTitle', { name: model.displayName })"
    width="760px"
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
      class="ai-provider-model-edit"
      @submit.prevent="submit"
    >
      <div class="ai-provider-model-edit__grid">
        <el-form-item prop="displayName" :label="t('ai.providerModel.displayName')">
          <el-input v-model="form.displayName" maxlength="200" />
        </el-form-item>
        <el-form-item prop="modelKind" :label="t('ai.providerModel.modelKind')">
          <el-select v-model="form.modelKind" class="ai-provider-model-edit__select">
            <el-option
              v-for="item in options.modelKinds"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="pricingMode" :label="t('ai.providerModel.pricingMode')">
          <el-select v-model="form.pricingMode" class="ai-provider-model-edit__select">
            <el-option
              v-for="item in options.pricingModes"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="priceUnit" :label="t('ai.providerModel.priceUnit')">
          <el-select
            v-model="form.priceUnit"
            class="ai-provider-model-edit__select"
            :disabled="isTokenMode"
          >
            <el-option
              v-for="item in options.priceUnits"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="officialModelId" :label="t('ai.providerModel.officialModel')">
          <el-select
            v-model="form.officialModelId"
            clearable
            filterable
            class="ai-provider-model-edit__select"
            :placeholder="t('ai.providerModel.officialPlaceholder')"
          >
            <el-option
              v-for="item in officialOptions"
              :key="item.id"
              :label="`${item.displayName} · ${item.modelKey}`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item prop="status" :label="t('ai.providerModel.status')">
          <el-select v-model="form.status" class="ai-provider-model-edit__select">
            <el-option :label="t('ai.providerModel.enabled')" :value="1" />
            <el-option :label="t('ai.providerModel.disabled')" :value="0" />
          </el-select>
        </el-form-item>
      </div>

      <el-alert
        v-if="isTokenMode"
        :title="t('ai.providerModel.tokenHint')"
        type="info"
        :closable="false"
        class="ai-provider-model-edit__alert"
      />

      <div v-else class="ai-provider-model-edit__prices">
        <div class="ai-provider-model-edit__prices-head">
          <div>
            <strong>{{ t('ai.providerModel.prices') }}</strong>
            <p>{{ t('ai.providerModel.pricesHint') }}</p>
          </div>
          <el-button size="small" @click="addTier">{{ t('ai.providerModel.addTier') }}</el-button>
        </div>
        <div
          v-for="(price, index) in prices"
          :key="index"
          class="ai-provider-model-edit__price-row"
        >
          <el-input
            v-model="price.tierKey"
            maxlength="64"
            :placeholder="t('ai.providerModel.tierKey')"
          />
          <el-input
            v-model="price.tierLabel"
            maxlength="64"
            :placeholder="t('ai.providerModel.tierLabel')"
          />
          <el-input
            v-model="price.priceCny"
            maxlength="20"
            :placeholder="t('ai.providerModel.priceCny')"
          >
            <template #append>{{ t('ai.providerModel.cny') }}</template>
          </el-input>
          <el-button text type="danger" @click="removeTier(index)">
            {{ t('common.delete') }}
          </el-button>
        </div>
      </div>

      <el-alert
        v-if="localError || error"
        :title="localError || error"
        type="error"
        :closable="false"
      />
    </el-form>
    <template #footer>
      <el-button :disabled="submitting" @click="visible = false">
        {{ t('common.cancel') }}
      </el-button>
      <el-button type="primary" :loading="submitting" @click="submit">
        {{ t('common.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.ai-provider-model-edit {
  display: flex;
  flex-direction: column;
  gap: 4px;

  &__grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 16px;
  }

  &__select {
    width: 100%;
  }

  &__alert {
    margin-bottom: 12px;
  }

  &__prices {
    margin-top: 4px;
    padding: 12px;
    border: 1px solid var(--admin-border);
    border-radius: 8px;
  }

  &__prices-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 10px;

    p {
      margin: 4px 0 0;
      color: var(--admin-text-muted);
      font-size: 12px;
    }
  }

  &__price-row {
    display: grid;
    grid-template-columns: 1.1fr 1fr 1fr auto;
    gap: 8px;
    align-items: center;
    margin-bottom: 8px;
  }
}

@media (max-width: 680px) {
  .ai-provider-model-edit__grid,
  .ai-provider-model-edit__price-row {
    grid-template-columns: 1fr;
  }
}
</style>
