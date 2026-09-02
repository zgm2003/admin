<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Send, Trash2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import * as mailApi from '../../../../../api/system/mail'
import { YesNo } from '../../../../../enums/yes-no'

const props = defineProps<{
  config: mailApi.MailConfig
  canUpdate: boolean
  canTest: boolean
  canDelete: boolean
}>()

const emit = defineEmits<{ saved: []; deleted: [] }>()
const { t } = useI18n()
const formRef = ref<FormInstance>()
const saving = ref(false)
const testing = ref(false)
const testEmail = ref('')
const form = ref<mailApi.MailConfigInput>(blankForm())

const statusText = computed(() => {
  if (!props.config.configured) return t('mail.statusUnconfigured')
  return props.config.isEnabled === YesNo.Yes ? t('mail.statusActive') : t('mail.statusInactive')
})

const statusType = computed(() => {
  if (!props.config.configured) return 'info'
  return props.config.isEnabled === YesNo.Yes ? 'success' : 'warning'
})

const rules = computed<FormRules<mailApi.MailConfigInput>>(() => ({
  region: [{ required: true, whitespace: true, message: t('mail.region'), trigger: 'blur' }],
  fromEmail: [{ required: true, type: 'email', message: t('auth.login.emailInvalid'), trigger: 'blur' }],
  fromName: [{ required: true, whitespace: true, message: t('mail.fromName'), trigger: 'blur' }],
  ttlMinutes: [{ required: true, type: 'number', min: 1, max: 60, message: t('mail.ttl'), trigger: 'change' }],
}))

watch(() => props.config, (value) => {
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
}, { immediate: true })

function blankForm(): mailApi.MailConfigInput {
  return {
    secretId: '',
    secretKey: '',
    region: '',
    endpoint: '',
    fromEmail: '',
    fromName: '',
    replyTo: '',
    ttlMinutes: 10,
    isEnabled: YesNo.No,
  }
}

async function save(): Promise<void> {
  if (!await formRef.value?.validate().catch(() => false)) return

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
  } finally {
    testing.value = false
  }
}

async function remove(): Promise<void> {
  await ElMessageBox.confirm(t('mail.deleteConfirm'))
  await mailApi.deleteMailConfig()
  ElMessage.success(t('mail.deleted'))
  emit('deleted')
}
</script>

<template>
  <div class="config-tab">
    <div class="config-status">
      <div>
        <strong>{{ t('mail.configTab') }}</strong>
        <span>{{ config.fromEmail || t('mail.statusDescription') }}</span>
      </div>
      <el-tag :type="statusType" effect="plain">{{ statusText }}</el-tag>
    </div>

    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="100px"
      class="mail-form"
      @submit.prevent="save"
    >
      <el-divider content-position="left">{{ t('mail.credentialsTitle') }}</el-divider>
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
            <el-input v-model="form.region" placeholder="ap-guangzhou" />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.endpoint')">
            <el-input v-model="form.endpoint" :placeholder="t('mail.optional')" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-divider content-position="left">{{ t('mail.senderTitle') }}</el-divider>
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
            <el-input-number v-model="form.ttlMinutes" :min="1" :max="60" controls-position="right" />
            <span class="input-unit">min</span>
          </el-form-item>
        </el-col>
      </el-row>

      <el-divider content-position="left">{{ t('mail.deliveryTitle') }}</el-divider>
      <el-row :gutter="20">
        <el-col :xs="24" :md="12">
          <el-form-item :label="t('mail.enabled')">
            <el-switch v-model="form.isEnabled" :active-value="YesNo.Yes" :inactive-value="YesNo.No" />
          </el-form-item>
        </el-col>
        <el-col v-if="canTest" :xs="24" :md="12">
          <el-form-item :label="t('mail.testRecipient')">
            <div class="test-input">
              <el-input v-model="testEmail" />
              <el-button :loading="testing" @click="sendTest">
                <Send :size="16" />
                {{ t('mail.test') }}
              </el-button>
            </div>
          </el-form-item>
        </el-col>
      </el-row>

      <el-alert v-if="config.lastTestError" :title="config.lastTestError" type="warning" show-icon :closable="false" />

      <div class="form-actions">
        <el-button v-if="canDelete && config.configured" text type="danger" @click="remove">
          <Trash2 :size="16" />
          {{ t('mail.delete') }}
        </el-button>
        <span />
        <el-button v-if="canUpdate" data-testid="mail-config-save" type="primary" :loading="saving" @click="save">
          {{ t('mail.save') }}
        </el-button>
      </div>
    </el-form>
  </div>
</template>

<style scoped lang="scss">
.config-status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.config-status > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.config-status strong {
  font-size: 15px;
  font-weight: 600;
}

.config-status span {
  overflow: hidden;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mail-form {
  padding-top: 2px;
}

.mail-form :deep(.el-divider__text) {
  font-size: 13px;
  font-weight: 600;
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
  margin-top: 8px;
  padding-top: 16px;
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
