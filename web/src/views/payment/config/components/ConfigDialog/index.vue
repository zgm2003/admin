<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules, UploadFile, UploadUserFile } from 'element-plus'
import type { PaymentConfigOptions } from '@/api/payment/config'
import type { ConfigDialogMode, ConfigForm } from './types'
const props = defineProps<{
  mode: ConfigDialogMode
  options: PaymentConfigOptions
  submitting: boolean
  error: string
}>()
const visible = defineModel<boolean>({ required: true })
const form = defineModel<ConfigForm>('form', { required: true })
const emit = defineEmits<{ save: []; cancel: [] }>()
const { t } = useI18n()
const formRef = ref<FormInstance>()
const credentialFields = ['privateKey', 'appCertPEM', 'alipayCertPEM', 'rootCertPEM'] as const
type CertificateField = 'appCertPEM' | 'alipayCertPEM' | 'rootCertPEM'
const importFiles = ref<Record<CertificateField, UploadUserFile[]>>({
  appCertPEM: [],
  alipayCertPEM: [],
  rootCertPEM: [],
})
const importErrors = ref<Record<CertificateField, string>>({
  appCertPEM: '',
  alipayCertPEM: '',
  rootCertPEM: '',
})
const importNames = ref<Record<CertificateField, string>>({
  appCertPEM: '',
  alipayCertPEM: '',
  rootCertPEM: '',
})
const importing = ref<Record<CertificateField, boolean>>({
  appCertPEM: false,
  alipayCertPEM: false,
  rootCertPEM: false,
})
const importSequence: Record<CertificateField, number> = {
  appCertPEM: 0,
  alipayCertPEM: 0,
  rootCertPEM: 0,
}
const importBusy = computed(() => Object.values(importing.value).some(Boolean))
let importEpoch = 0
function clearImportError(field: (typeof credentialFields)[number]) {
  if (field !== 'privateKey') importErrors.value[field] = ''
}
async function importCertificate(field: CertificateField, uploadFile: UploadFile) {
  const file = uploadFile.raw
  importFiles.value[field] = []
  if (file === undefined || props.submitting || !visible.value) return
  const sequence = ++importSequence[field]
  const epoch = importEpoch
  const accepted = () =>
    epoch === importEpoch && sequence === importSequence[field] && visible.value
  importErrors.value[field] = ''
  if (
    file.size < 1 ||
    file.size > props.options.limits.certificateBytes ||
    !/\.(crt|pem)$/i.test(file.name)
  ) {
    importErrors.value[field] = t('payment.config.importInvalid', {
      max: props.options.limits.certificateBytes,
    })
    return
  }
  importing.value[field] = true
  try {
    const raw = (await file.text()).replace(/^\uFEFF/, '').trim()
    if (!accepted()) return
    if (
      new TextEncoder().encode(raw).length > props.options.limits.certificateBytes ||
      !/^(?:-----BEGIN CERTIFICATE-----\s+[A-Za-z0-9+/=\s]+-----END CERTIFICATE-----\s*)+$/.test(
        raw,
      )
    ) {
      importErrors.value[field] = t('payment.config.importInvalid', {
        max: props.options.limits.certificateBytes,
      })
      return
    }
    form.value[field] = raw
    importNames.value[field] = file.name
    await formRef.value?.validateField(field).catch(() => undefined)
  } catch {
    if (accepted()) importErrors.value[field] = t('payment.config.importFailed')
  } finally {
    if (accepted()) importing.value[field] = false
  }
}
const rules = computed<FormRules<ConfigForm>>(() => {
  const required = { required: true, message: t('payment.config.required'), trigger: 'blur' }
  const result: FormRules<ConfigForm> = {}
  if (props.mode !== 'version') {
    result.name = [
      required,
      { max: props.options.limits.name, message: t('payment.config.tooLong'), trigger: 'blur' },
    ]
  }
  if (props.mode === 'create') {
    result.appId = [
      required,
      {
        pattern: /^[0-9]+$/,
        max: props.options.limits.identity,
        message: t('payment.config.identityRule'),
        trigger: 'blur',
      },
    ]
    result.sellerId = result.appId
  }
  if (props.mode !== 'edit') {
    result.notifyUrl = [
      required,
      {
        validator: (_rule, value: unknown, callback) => {
          const template = props.options.notifyPathTemplate
          const message = () => new Error(t('payment.config.notifyRule', { template }))
          if (typeof value !== 'string' || value.trim() !== value || !value.endsWith(template)) {
            callback(message())
            return
          }
          try {
            const path = template.replace('{configId}', '1').replace('{version}', '1')
            const url = new URL(value.replace('{configId}', '1').replace('{version}', '1'))
            if (
              !value.startsWith('https://') ||
              url.hostname === '' ||
              url.hostname.toLowerCase() === 'host' ||
              url.username !== '' ||
              url.password !== '' ||
              value.includes('?') ||
              value.includes('#') ||
              url.pathname !== path
            ) {
              callback(message())
              return
            }
            callback()
          } catch {
            callback(message())
          }
        },
        trigger: 'blur',
      },
    ]
    result.payMethods = [
      {
        type: 'array',
        required: true,
        min: 1,
        message: t('payment.config.required'),
        trigger: 'change',
      },
    ]
    for (const key of credentialFields) {
      result[key] = props.mode === 'create' ? [required] : []
    }
  }
  return result
})
watch(visible, async (open) => {
  importEpoch++
  importFiles.value = { appCertPEM: [], alipayCertPEM: [], rootCertPEM: [] }
  importErrors.value = { appCertPEM: '', alipayCertPEM: '', rootCertPEM: '' }
  importNames.value = { appCertPEM: '', alipayCertPEM: '', rootCertPEM: '' }
  importing.value = { appCertPEM: false, alipayCertPEM: false, rootCertPEM: false }
  if (open) {
    await nextTick()
    formRef.value?.clearValidate()
  }
})
onBeforeUnmount(() => {
  importEpoch++
})
async function save() {
  if (
    props.submitting ||
    importBusy.value ||
    Object.values(importErrors.value).some(Boolean) ||
    formRef.value === undefined
  )
    return
  const valid = await formRef.value.validate().catch(() => false)
  if (valid) emit('save')
}
</script>
<template>
  <AppDialog
    v-model="visible"
    :title="t(`payment.config.${mode}Title`)"
    width="780px"
    :append-to-body="false"
    :close-on-click-modal="false"
    :close-on-press-escape="!submitting"
    :show-close="!submitting"
  >
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-alert
      v-if="mode === 'version'"
      :title="t('payment.config.retainHint')"
      type="info"
      :closable="false"
    />
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      :disabled="submitting"
      label-position="top"
      class="payment-config-form"
      data-testid="payment-config-form"
      @submit.prevent="save"
    >
      <div v-if="mode === 'create'" class="payment-config-form__grid">
        <el-form-item :label="t('payment.config.provider')"
          ><el-select-v2
            v-model="form.provider"
            :options="options.providers"
            data-testid="payment-config-provider"
        /></el-form-item>
        <el-form-item :label="t('payment.config.environment')"
          ><el-select-v2
            v-model="form.environment"
            :options="options.environments"
            data-testid="payment-config-environment"
        /></el-form-item>
        <el-form-item :label="t('payment.config.status')"
          ><el-select-v2 v-model="form.isEnabled" :options="options.statuses"
        /></el-form-item>
        <el-form-item prop="appId" :label="t('payment.config.appId')"
          ><el-input
            v-model="form.appId"
            :maxlength="options.limits.identity"
            :placeholder="t('payment.config.appIdPlaceholder')"
            data-testid="payment-config-app-id"
        /></el-form-item>
        <el-form-item prop="sellerId" :label="t('payment.config.sellerId')"
          ><el-input
            v-model="form.sellerId"
            :maxlength="options.limits.identity"
            :placeholder="t('payment.config.sellerIdPlaceholder')"
            data-testid="payment-config-seller-id"
        /></el-form-item>
      </div>
      <div v-if="mode !== 'version'" class="payment-config-form__grid">
        <el-form-item prop="name" :label="t('payment.config.name')"
          ><el-input
            v-model="form.name"
            :maxlength="options.limits.name"
            :placeholder="t('payment.config.namePlaceholder')"
            data-testid="payment-config-name"
        /></el-form-item>
        <el-form-item prop="sortOrder" :label="t('payment.config.sortOrder')"
          ><el-input-number
            v-model="form.sortOrder"
            :min="options.limits.sortMinimum"
            :max="options.limits.sortMaximum"
            :precision="0"
        /></el-form-item>
      </div>
      <template v-if="mode !== 'edit'">
        <el-form-item prop="payMethods" :label="t('payment.config.payMethods')"
          ><el-select-v2
            v-model="form.payMethods"
            :options="options.payMethods"
            multiple
            data-testid="payment-config-pay-methods"
        /></el-form-item>
        <el-form-item prop="notifyUrl" :label="t('payment.config.notifyUrl')"
          ><el-input
            v-model="form.notifyUrl"
            :maxlength="options.limits.notifyUrl"
            :placeholder="options.defaults.notifyUrl"
            data-testid="payment-config-notify-url"
        /></el-form-item>
        <p class="payment-config-form__hint">{{ t('payment.config.notifyHint') }}</p>
        <el-form-item
          v-for="field in credentialFields"
          :key="field"
          :prop="field"
          class="payment-config-form__credential-item"
        >
          <template #label>
            <span class="payment-config-form__credential-label">
              <span
                class="payment-config-form__credential-label-text"
                :title="t(`payment.config.${field}`)"
              >
                {{ t(`payment.config.${field}`) }}
              </span>
              <el-upload
                v-if="field !== 'privateKey'"
                v-model:file-list="importFiles[field]"
                class="payment-config-form__import-control"
                accept=".crt,.pem"
                :auto-upload="false"
                :show-file-list="false"
                :disabled="submitting || importing[field]"
                :data-testid="`payment-config-import-${field}`"
                :on-change="(file: UploadFile) => importCertificate(field, file)"
              >
                <el-button
                  size="small"
                  :disabled="submitting || importing[field]"
                  :loading="importing[field]"
                >
                  {{ t('payment.config.importCertificate') }}
                </el-button>
              </el-upload>
              <span
                v-if="field !== 'privateKey' && (importing[field] || importNames[field])"
                class="payment-config-form__import-name"
                :title="importNames[field] || undefined"
              >
                {{ importing[field] ? t('payment.config.importing') : importNames[field] }}
              </span>
            </span>
          </template>
          <div class="payment-config-form__credential-entry">
            <el-input
              v-model="form[field]"
              type="textarea"
              :autosize="{ minRows: 4, maxRows: 8 }"
              clearable
              :placeholder="
                field === 'privateKey'
                  ? props.mode === 'create'
                    ? t('payment.config.privateKeyPlaceholder')
                    : t('payment.config.privateKeyRetainPlaceholder')
                  : t(`payment.config.${field}Placeholder`)
              "
              autocomplete="off"
              spellcheck="false"
              :data-testid="
                field === 'privateKey' ? 'payment-config-private-key' : `payment-config-${field}`
              "
              :maxlength="
                field === 'privateKey'
                  ? options.limits.privateKeyBytes
                  : options.limits.certificateBytes
              "
              @input="clearImportError(field)"
            />
            <div v-if="field !== 'privateKey'" class="payment-config-form__import">
              <span
                v-if="importErrors[field]"
                role="alert"
                class="payment-config-form__import-error"
                >{{ importErrors[field] }}</span
              >
            </div>
          </div>
        </el-form-item>
      </template>
    </el-form>
    <template #footer
      ><el-button :disabled="submitting" @click="emit('cancel')">{{
        t('payment.config.cancel')
      }}</el-button
      ><el-button
        type="primary"
        :loading="submitting"
        :disabled="importBusy || Object.values(importErrors).some(Boolean)"
        data-testid="payment-config-save"
        @click="save"
        >{{ t('payment.config.save') }}</el-button
      ></template
    >
  </AppDialog>
</template>
<style scoped lang="scss">
.payment-config-form {
  padding-top: 16px;
  &__grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0 20px;
  }
  &__hint {
    color: var(--el-text-color-secondary);
    margin: -8px 0 16px;
    font-size: 12px;
  }
  &__import {
    display: flex;
    gap: 8px;
    overflow-wrap: anywhere;
  }
  &__credential-entry {
    width: 100%;
  }
  &__credential-label {
    display: flex;
    align-items: center;
    width: 100%;
    min-width: 0;
    gap: 8px;
  }
  &__credential-label-text {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  &__import-control {
    flex: 0 0 auto;
  }
  &__import-name {
    flex: 1 1 0;
    min-width: 40px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  :deep(.payment-config-form__credential-item.el-form-item--label-top > .el-form-item__label) {
    display: flex;
    align-items: center;
    width: 100%;
    max-width: 100%;
    box-sizing: border-box;
    overflow: hidden;
  }
  &__import-error {
    color: var(--el-color-danger);
    font-size: 12px;
  }
  :deep(.el-input-number),
  :deep(.el-select-v2) {
    width: 100%;
  }
  :deep(textarea) {
    font-family: monospace;
    font-size: 12px;
  }
}
@media (max-width: 640px) {
  .payment-config-form__grid {
    grid-template-columns: 1fr;
  }
}
</style>
