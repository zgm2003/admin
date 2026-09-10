<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Send } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import * as mailApi from '@/api/message/mail'
import type { DictionaryOptions } from '@/api/system/dictionary'
import { YesNo } from '@/enums/yesNo'
import { useSystemDictionaryStore } from '@/store/systemDictionary'

const props = defineProps<{
  config: mailApi.MailConfig
  canUpdate: boolean
  canTest: boolean
  canDelete: boolean
}>()

const emit = defineEmits<{ saved: []; deleted: []; tested: [] }>()
type DictionaryOption = DictionaryOptions[string][number]

const { t, locale } = useI18n()
const dictionaries = useSystemDictionaryStore()
const formRef = ref<FormInstance>()
const saving = ref(false)
const testing = ref(false)
const testEmail = ref('')
const form = ref<mailApi.MailConfigInput>(blankForm())
const regionOptions = ref<DictionaryOption[]>([])
const regionOptionsLoading = ref(false)
const regionOptionsError = ref('')
let regionOptionsRequest = 0

const rules = computed<FormRules<mailApi.MailConfigInput>>(() => ({
  region: [{ required: true, message: t('mail.regionRequired'), trigger: 'change' }],
  fromEmail: [
    { required: true, type: 'email', message: t('auth.login.emailInvalid'), trigger: 'blur' },
  ],
  fromName: [{ required: true, whitespace: true, message: t('mail.fromName'), trigger: 'blur' }],
  ttlMinutes: [
    { required: true, type: 'number', min: 1, max: 60, message: t('mail.ttl'), trigger: 'change' },
  ],
}))

watch(
  () => props.config,
  (value) => {
    form.value = {
      secretId: '',
      secretKey: '',
      region: value.region,
      endpoint: value.endpoint,
      fromEmail: value.fromEmail,
      fromName: value.fromName,
      replyTo: value.replyTo,
      ttlMinutes: value.ttlMinutes,
      isEnabled: value.isEnabled,
    }
    testEmail.value = value.fromEmail
  },
  { immediate: true },
)

function blankForm(): mailApi.MailConfigInput {
  return {
    secretId: '',
    secretKey: '',
    region: '',
    endpoint: '',
    fromEmail: '',
    fromName: '',
    replyTo: '',
    ttlMinutes: 0,
    isEnabled: YesNo.No,
  }
}

async function loadRegionOptions(): Promise<void> {
  const request = ++regionOptionsRequest
  regionOptionsLoading.value = true
  regionOptionsError.value = ''
  try {
    await dictionaries.load(['message.mail.region'])
    if (request !== regionOptionsRequest) return
    const options = dictionaries.options('message.mail.region').value
    if (options === undefined) throw new Error('message.mail.region dictionary is not ready')
    regionOptions.value = options.map((item) => ({ ...item }))
  } catch {
    if (request !== regionOptionsRequest) return
    regionOptions.value = []
    regionOptionsError.value = t('mail.regionOptionsLoadFailed')
  } finally {
    if (request === regionOptionsRequest) regionOptionsLoading.value = false
  }
}

async function save(): Promise<void> {
  if (regionOptionsLoading.value || regionOptionsError.value !== '') return
  if (!(await formRef.value?.validate().catch(() => false))) return

  saving.value = true
  try {
    await mailApi.saveMailConfig(form.value)
    ElMessage.success(t('mail.saved'))
    emit('saved')
  } finally {
    saving.value = false
  }
}

async function sendTest(): Promise<void> {
  if (!testEmail.value.trim()) return

  testing.value = true
  try {
    await mailApi.sendMailTest({
      toEmail: testEmail.value.trim(),
      scene: 'login',
      variables: {
        code: '123456',
        ttl_minutes: String(form.value.ttlMinutes),
      },
    })
    ElMessage.success(t('mail.testSent'))
  } catch {
    // request.ts owns the API error notification.
  } finally {
    testing.value = false
    emit('tested')
  }
}

async function remove(): Promise<void> {
  try {
    await ElMessageBox.confirm(t('mail.deleteConfirm'))
    await mailApi.deleteMailConfig()
    ElMessage.success(t('mail.deleted'))
    emit('deleted')
  } catch {
    // ElMessageBox cancellation and request errors are handled by their respective layers.
  }
}

watch(locale, () => void loadRegionOptions(), { immediate: true })
</script>

<template>
  <div class="config-tab">
    <el-alert
      v-if="regionOptionsError"
      :title="regionOptionsError"
      type="error"
      show-icon
      :closable="false"
    />
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="120px"
      class="mail-form"
      @submit.prevent="save"
    >
      <el-row :gutter="20">
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.secretId')">
            <el-input
              v-model="form.secretId"
              type="password"
              show-password
              autocomplete="off"
              :placeholder="config.configured ? t('mail.secretKeepPlaceholder') : ''"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.secretKey')">
            <el-input
              v-model="form.secretKey"
              type="password"
              show-password
              autocomplete="off"
              :placeholder="config.configured ? t('mail.secretKeepPlaceholder') : ''"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.region')" prop="region">
            <el-select-v2
              v-model="form.region"
              :options="regionOptions"
              :loading="regionOptionsLoading"
              :disabled="regionOptionsLoading || regionOptionsError !== ''"
              :placeholder="t('mail.regionPlaceholder')"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.endpoint')">
            <el-input v-model="form.endpoint" :placeholder="t('mail.optional')" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="20">
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.fromEmail')" prop="fromEmail">
            <el-input v-model="form.fromEmail" />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.fromName')" prop="fromName">
            <el-input v-model="form.fromName" />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.replyTo')">
            <el-input v-model="form.replyTo" :placeholder="t('mail.optional')" />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.ttl')" prop="ttlMinutes">
            <el-input-number
              v-model="form.ttlMinutes"
              :min="1"
              :max="60"
              controls-position="right"
            />
            <span class="input-unit">min</span>
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="20">
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.enabled')">
            <el-switch
              v-model="form.isEnabled"
              :active-value="YesNo.Yes"
              :inactive-value="YesNo.No"
            />
          </el-form-item>
        </el-col>
        <el-col v-if="canTest" :xs="24" :md="12">
          <el-form-item :label="t('mail.testRecipient')">
            <div class="test-input">
              <el-input v-model="testEmail" />
              <el-button
                data-testid="mail-config-test"
                :loading="testing"
                :disabled="!config.configured || config.isEnabled !== YesNo.Yes"
                @click="sendTest"
              >
                <Send :size="16" />
                {{ t('mail.test') }}
              </el-button>
            </div>
          </el-form-item>
        </el-col>
      </el-row>

      <el-alert
        v-if="config.lastTestError"
        :title="config.lastTestError"
        type="warning"
        show-icon
        :closable="false"
      />

      <div class="form-actions">
        <el-button v-if="canDelete && config.configured" type="danger" @click="remove">
          {{ t('mail.delete') }}
        </el-button>
        <span />
        <el-button
          v-if="canUpdate"
          data-testid="mail-config-save"
          type="primary"
          :loading="saving"
          :disabled="regionOptionsLoading || regionOptionsError !== ''"
          @click="save"
        >
          {{ t('mail.save') }}
        </el-button>
      </div>
    </el-form>
  </div>
</template>

<style scoped>
.mail-form {
  max-width: none;
}

.mail-form :deep(.el-input-number) {
  width: calc(100% - 32px);
}

.input-unit {
  width: 24px;
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.test-input {
  display: flex;
  width: 100%;
  gap: 8px;
}

.form-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 2px;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
}

.form-actions > span {
  flex: 1;
}

@media (max-width: 640px) {
  .test-input {
    flex-direction: column;
  }
}
</style>
