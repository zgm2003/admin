<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import AppDialog from '@/components/AppDialog/index.vue'
import type {
  GenerateRedeemCodeInput,
  GenerateRedeemCodeResult,
  RedeemCodeOptions,
} from '@/api/payment/redeemCode'

const props = defineProps<{
  modelValue: boolean
  options: RedeemCodeOptions | null
  submitting: boolean
  result: GenerateRedeemCodeResult | null
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  submit: [value: GenerateRedeemCodeInput]
  close: []
}>()
const { t } = useI18n()

interface FormState {
  amount: string
  quantity: number
  expiresAt: string | null
  remark: string
}

const form = ref<FormState>({ amount: '', quantity: 1, expiresAt: null, remark: '' })
const error = ref('')
const codeText = computed(() => props.result?.codes.join('\n') ?? '')

function reset(): void {
  form.value = {
    amount: props.options?.limits.minimumAmount ?? '',
    quantity: props.options?.defaults.quantity ?? 1,
    expiresAt: null,
    remark: '',
  }
  error.value = ''
}
function validate(): string {
  const limits = props.options?.limits
  if (limits === undefined) return t('payment.redeemCode.optionsUnavailable')
  if (!/^\d+(\.\d{1,2})?$/.test(form.value.amount.trim())) {
    return t('payment.redeemCode.amountRule')
  }
  const amount = Number(form.value.amount)
  if (
    !Number.isFinite(amount) ||
    amount <= 0 ||
    amount < Number(limits.minimumAmount) ||
    amount > Number(limits.maximumAmount)
  ) {
    return t('payment.redeemCode.amountRule')
  }
  if (
    !Number.isInteger(form.value.quantity) ||
    form.value.quantity < 1 ||
    form.value.quantity > limits.maxBatchQuantity
  ) {
    return t('payment.redeemCode.quantityRule')
  }
  if (form.value.remark.trim().length > limits.remark) return t('payment.redeemCode.remarkRule')
  return ''
}
function requestID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function')
    return crypto.randomUUID()
  const tail = `${Date.now()}${Math.floor(Math.random() * 1_000_000)}`.padStart(12, '0').slice(-12)
  return `00000000-0000-4000-8000-${tail}`
}
function submit(): void {
  if (props.submitting || props.result !== null) return
  const validation = validate()
  if (validation !== '') {
    error.value = validation
    return
  }
  error.value = ''
  emit('submit', {
    requestId: requestID(),
    amount: form.value.amount.trim(),
    quantity: form.value.quantity,
    expiresAt: form.value.expiresAt,
    remark: form.value.remark.trim(),
  })
}
function close(): void {
  if (props.submitting) return
  emit('update:modelValue', false)
  emit('close')
}

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) reset()
  },
)
</script>

<template>
  <AppDialog
    :model-value="modelValue"
    :title="t('payment.redeemCode.generateTitle')"
    width="560px"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <template v-if="result === null">
      <el-form
        label-position="top"
        class="payment-redeem-code-generate-form"
        @submit.prevent="submit"
      >
        <div class="payment-redeem-code-generate-form__grid">
          <el-form-item :label="t('payment.redeemCode.amount')">
            <el-input
              v-model="form.amount"
              inputmode="decimal"
              :placeholder="t('payment.redeemCode.amountPlaceholder')"
              data-testid="payment-redeem-code-amount"
            />
          </el-form-item>
          <el-form-item :label="t('payment.redeemCode.quantity')">
            <el-input-number
              v-model="form.quantity"
              :min="1"
              :max="options?.limits.maxBatchQuantity"
              controls-position="right"
              data-testid="payment-redeem-code-quantity"
            />
          </el-form-item>
        </div>
        <el-form-item :label="t('payment.redeemCode.expiresAt')">
          <el-date-picker
            v-model="form.expiresAt"
            type="datetime"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            :placeholder="t('payment.redeemCode.expiresAtPlaceholder')"
            class="payment-redeem-code-generate-form__date"
            data-testid="payment-redeem-code-expires-at"
          />
        </el-form-item>
        <el-form-item :label="t('payment.redeemCode.remark')">
          <el-input
            v-model="form.remark"
            type="textarea"
            :maxlength="options?.limits.remark"
            show-word-limit
            :placeholder="t('payment.redeemCode.remarkPlaceholder')"
            data-testid="payment-redeem-code-remark"
          />
        </el-form-item>
        <el-alert v-if="error" :title="error" type="error" :closable="false" />
      </el-form>
    </template>
    <template v-else>
      <el-alert
        :title="t('payment.redeemCode.generateSuccess', { batchNo: result.batchNo })"
        type="success"
        :closable="false"
      />
      <p class="payment-redeem-code-generate__hint">{{ t('payment.redeemCode.secretHint') }}</p>
      <el-input
        :model-value="codeText"
        type="textarea"
        readonly
        :rows="Math.min(12, Math.max(3, result.codes.length))"
        data-testid="payment-redeem-code-result"
      />
    </template>
    <template #footer>
      <el-button :disabled="submitting" @click="close">{{
        t('payment.redeemCode.close')
      }}</el-button>
      <el-button
        v-if="result === null"
        type="primary"
        :loading="submitting"
        data-testid="payment-redeem-code-submit"
        @click="submit"
        >{{ t('payment.redeemCode.generate') }}</el-button
      >
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.payment-redeem-code-generate-form__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
.payment-redeem-code-generate-form__date {
  width: 100%;
}
.payment-redeem-code-generate__hint {
  margin: 16px 0 8px;
  color: var(--el-text-color-secondary);
}
@media (max-width: 680px) {
  .payment-redeem-code-generate-form__grid {
    grid-template-columns: 1fr;
  }
}
</style>
