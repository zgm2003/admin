<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Send } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import * as smsApi from '@/api/message/sms'
import type { DictionaryOptions } from '@/api/system/dictionary'
import { YesNo } from '@/enums/yesNo'
import { useSystemDictionaryStore } from '@/store/systemDictionary'

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
type DictionaryOption = DictionaryOptions[string][number]

const { t, locale } = useI18n()
const dictionaries = useSystemDictionaryStore()
const form = ref<smsApi.SmsConfigInput>(blankForm())
const testPhone = ref('')
const testScene = ref<smsApi.SmsScene>('login')
const saving = ref(false)
const testing = ref(false)
const regionOptions = ref<DictionaryOption[]>([])
const regionOptionsLoading = ref(false)
const regionOptionsError = ref('')
let regionOptionsRequest = 0

const canSave = computed(
  () =>
    !props.loading &&
    !saving.value &&
    !regionOptionsLoading.value &&
    regionOptionsError.value === '' &&
    form.value.smsSdkAppId.trim() !== '' &&
    form.value.signName.trim() !== '' &&
    form.value.region !== '' &&
    form.value.ttlMinutes >= 1 &&
    form.value.ttlMinutes <= 60,
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
    ttlMinutes: 5,
    isEnabled: YesNo.No,
  }
}

async function loadRegionOptions(): Promise<void> {
  const request = ++regionOptionsRequest
  regionOptionsLoading.value = true
  regionOptionsError.value = ''
  try {
    await dictionaries.load(['message.sms.region'])
    if (request !== regionOptionsRequest) return
    const options = dictionaries.options('message.sms.region').value
    if (options === undefined) throw new Error('message.sms.region dictionary is not ready')
    regionOptions.value = options.map((item) => ({ ...item }))
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
  if (testing.value || !props.catalogReady || toPhone === '') return
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
    <el-form :model="form" label-position="top" @submit.prevent="save">
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
              :min="1"
              :max="60"
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

    <template v-if="canTest">
      <el-divider />
      <el-form class="sms-config__test" inline @submit.prevent="sendTest">
        <el-form-item :label="t('sms.testPhone')">
          <el-input
            v-model="testPhone"
            data-testid="sms-test-phone"
            type="tel"
            inputmode="tel"
            :placeholder="t('sms.testPhonePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('sms.sceneLabel')">
          <el-select-v2
            v-model="testScene"
            data-testid="sms-test-scene"
            :options="sceneOptions"
            :disabled="!catalogReady"
            :placeholder="t('sms.scenePlaceholder')"
          />
        </el-form-item>
        <el-button
          data-testid="sms-test-send"
          type="primary"
          :loading="testing"
          :disabled="!catalogReady || testPhone.trim() === ''"
          @click="sendTest"
        >
          <Send :size="16" />
          {{ t('sms.sendTest') }}
        </el-button>
      </el-form>
    </template>
  </div>
</template>

<style scoped>
.sms-config__full {
  width: 100%;
}

.sms-config :deep(.el-input-number) {
  width: 100%;
}

.sms-config__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.sms-config__actions > span {
  flex: 1;
}

.sms-config__test {
  display: flex;
  align-items: end;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
