<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { Send } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import * as smsApi from '@/api/message/sms'
import { getSmsConfigOptions, type SmsConfigOptions } from '@/api/message/smsConfigOptions'
import { YesNo } from '@/enums/yesNo'

const props = defineProps<{
  config: smsApi.SmsConfig
  sceneOptions: Array<{ label: string; value: smsApi.SmsScene }>
  catalogReady: boolean
  loading: boolean
  canUpdate: boolean
  canDelete: boolean
  canTest: boolean
}>()
const emit = defineEmits<{ saved: []; deleted: [] }>()
type RegionOption = SmsConfigOptions['regions'][number]

const { t, locale } = useI18n()
const form = ref<smsApi.SmsConfigInput>(blankForm())
const testPhone = ref('')
const testScene = ref<smsApi.SmsScene>('')
watch(
  () => props.sceneOptions,
  (options) => {
    if (options.length > 0 && !options.some((option) => option.value === testScene.value)) {
      testScene.value = options[0]?.value ?? ''
    }
  },
  { immediate: true },
)
const saving = ref(false)
const testing = ref(false)
const regionOptions = ref<RegionOption[]>([])
const regionOptionsLoading = ref(false)
const regionOptionsError = ref('')
const constraints = ref<SmsConfigOptions['constraints'] | null>(null)
let regionOptionsRequest = 0
onBeforeUnmount(() => {
  regionOptionsRequest++
})

const canSave = computed(
  () =>
    !props.loading &&
    !saving.value &&
    !regionOptionsLoading.value &&
    regionOptionsError.value === '' &&
    form.value.smsSdkAppId.trim() !== '' &&
    form.value.signName.trim() !== '' &&
    form.value.region !== '' &&
    constraints.value !== null &&
    Number.isInteger(form.value.ttlMinutes) &&
    form.value.ttlMinutes >= constraints.value.minTTLMinutes &&
    form.value.ttlMinutes <= constraints.value.maxTTLMinutes,
)

watch(
  () => props.config,
  (value) => {
    form.value = {
      secretId: '',
      secretKey: '',
      smsSdkAppId: value.smsSdkAppId,
      signName: value.signName,
      region: value.region,
      endpoint: value.endpoint,
      ttlMinutes: value.ttlMinutes,
      isEnabled: value.isEnabled,
    }
  },
  { immediate: true },
)

function blankForm(): smsApi.SmsConfigInput {
  return {
    secretId: '',
    secretKey: '',
    smsSdkAppId: '',
    signName: '',
    region: '',
    endpoint: '',
    ttlMinutes: 0,
    isEnabled: YesNo.No,
  }
}

async function loadRegionOptions(): Promise<void> {
  const request = ++regionOptionsRequest
  regionOptionsLoading.value = true
  regionOptionsError.value = ''
  constraints.value = null
  regionOptions.value = []
  try {
    const options = await getSmsConfigOptions()
    if (request !== regionOptionsRequest) return
    regionOptions.value = options.regions
    constraints.value = options.constraints
  } catch {
    if (request !== regionOptionsRequest) return
    regionOptions.value = []
    regionOptionsError.value = t('sms.regionLoadFailed')
  } finally {
    if (request === regionOptionsRequest) regionOptionsLoading.value = false
  }
}

async function save(): Promise<void> {
  if (!canSave.value) return
  saving.value = true
  try {
    await smsApi.saveSmsConfig({
      ...form.value,
      smsSdkAppId: form.value.smsSdkAppId.trim(),
      signName: form.value.signName.trim(),
      endpoint: form.value.endpoint.trim(),
    })
    ElMessage.success(t('sms.saved'))
    emit('saved')
  } catch {
    // request.ts owns the API error notification.
  } finally {
    saving.value = false
  }
}

async function remove(): Promise<void> {
  try {
    await ElMessageBox.confirm(t('sms.deleteConfirm'))
    await smsApi.deleteSmsConfig()
    ElMessage.success(t('sms.deleted'))
    emit('deleted')
  } catch {
    // Dialog cancellation and request errors are handled by their respective layers.
  }
}

async function sendTest(): Promise<void> {
  const toPhone = testPhone.value.trim()
  if (testing.value || !props.catalogReady || toPhone === '' || testScene.value === '') return
  testing.value = true
  try {
    await smsApi.sendSmsTest({ toPhone, scene: testScene.value })
    ElMessage.success(t('sms.testSent'))
  } catch {
    // request.ts owns the API error notification.
  } finally {
    testing.value = false
  }
}

watch(locale, () => void loadRegionOptions(), { immediate: true })
</script>

<template>
  <div class="sms-config">
    <el-alert
      v-if="regionOptionsError"
      :title="regionOptionsError"
      type="error"
      :closable="false"
      show-icon
    />
    <el-form :model="form" label-width="120px" class="sms-config__form" @submit.prevent="save">
      <el-row :gutter="16">
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('sms.secretId')">
            <el-input
              v-model="form.secretId"
              data-testid="sms-config-secret-id"
              type="password"
              show-password
              autocomplete="off"
              :placeholder="
                config.configured ? t('sms.secretPlaceholder') : t('sms.secretIdPlaceholder')
              "
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('sms.secretKey')">
            <el-input
              v-model="form.secretKey"
              data-testid="sms-config-secret-key"
              type="password"
              show-password
              autocomplete="off"
              :placeholder="
                config.configured ? t('sms.secretPlaceholder') : t('sms.secretKeyPlaceholder')
              "
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('sms.sdkAppId')">
            <el-input
              v-model="form.smsSdkAppId"
              data-testid="sms-config-sdk-app-id"
              :placeholder="t('sms.sdkAppIdPlaceholder')"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('sms.signName')">
            <el-input
              v-model="form.signName"
              data-testid="sms-config-sign-name"
              :placeholder="t('sms.signNamePlaceholder')"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('sms.region')">
            <el-select-v2
              v-model="form.region"
              data-testid="sms-config-region"
              :options="regionOptions"
              :loading="regionOptionsLoading"
              :disabled="regionOptionsLoading || regionOptionsError !== ''"
              :placeholder="t('sms.regionPlaceholder')"
              class="sms-config__full"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('sms.endpoint')">
            <el-input
              v-model="form.endpoint"
              data-testid="sms-config-endpoint"
              :placeholder="t('sms.endpointPlaceholder')"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('sms.ttlMinutes')">
            <el-input-number
              v-model="form.ttlMinutes"
              data-testid="sms-config-ttl"
              :min="constraints?.minTTLMinutes"
              :max="constraints?.maxTTLMinutes"
              :placeholder="t('sms.ttlPlaceholder')"
              controls-position="right"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('sms.enabled')">
            <el-switch
              v-model="form.isEnabled"
              :active-value="YesNo.Yes"
              :inactive-value="YesNo.No"
            />
          </el-form-item>
        </el-col>
        <el-col v-if="canTest" :xs="24" :md="12">
          <el-form-item :label="t('sms.testPhone')">
            <el-input
              v-model="testPhone"
              data-testid="sms-test-phone"
              type="tel"
              inputmode="tel"
              :placeholder="t('sms.testPhonePlaceholder')"
            />
          </el-form-item>
        </el-col>
        <el-col v-if="canTest" :xs="24" :md="12">
          <el-form-item :label="t('sms.sceneLabel')">
            <div class="sms-config__test-action">
              <el-select-v2
                v-model="testScene"
                data-testid="sms-test-scene"
                :options="sceneOptions"
                :disabled="!catalogReady"
                :placeholder="t('sms.scenePlaceholder')"
                class="sms-config__test-scene"
              />
              <el-button
                data-testid="sms-test-send"
                :loading="testing"
                :disabled="!catalogReady || testPhone.trim() === ''"
                :icon="Send"
                @click="sendTest"
              >
                {{ t('sms.sendTest') }}
              </el-button>
            </div>
          </el-form-item>
        </el-col>
      </el-row>
      <div class="sms-config__actions">
        <el-button v-if="canDelete && config.configured" type="danger" @click="remove">
          {{ t('sms.delete') }}
        </el-button>
        <span />
        <el-button
          v-if="canUpdate"
          data-testid="sms-config-save"
          type="primary"
          :loading="saving"
          :disabled="!canSave"
          @click="save"
        >
          {{ t('sms.save') }}
        </el-button>
      </div>
    </el-form>
  </div>
</template>

<style scoped lang="scss">
.sms-config {
  &__form {
    max-width: none;
  }

  &__full {
    width: 100%;
  }

  :deep(.el-input-number) {
    width: 100%;
  }

  &__actions {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 2px;
    padding-top: 12px;
    border-top: 1px solid var(--el-border-color-lighter);
  }

  &__actions > span {
    flex: 1;
  }

  &__test-scene {
    min-width: 0;
    flex: 1;
  }

  &__test-action {
    display: flex;
    width: 100%;
    gap: 8px;
  }
}

@media (max-width: 640px) {
  .sms-config {
    &__test-action {
      align-items: stretch;
      flex-direction: column;
    }
  }
}
</style>
