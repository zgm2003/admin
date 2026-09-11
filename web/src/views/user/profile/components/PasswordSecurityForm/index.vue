<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ChangePasswordInput, PasswordCodeLoginType } from '@/api/user/profile'

type PasswordMode = 'password' | 'code'

interface Props {
  setPasswordMode: boolean
  passwordMode: PasswordMode
  passwordModeOptions: Array<{ label: string; value: PasswordMode }>
  passwordLoginType: PasswordCodeLoginType
  passwordIdentityOptions: Array<{ label: string; value: PasswordCodeLoginType }>
  passwordForm: ChangePasswordInput
  passwordCode: string
  passwordResendSeconds: number
  changingPassword: boolean
  sendingPasswordCode: boolean
  canUpdatePassword: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:passwordForm': [value: ChangePasswordInput]
  'update:passwordMode': [value: PasswordMode]
  'update:passwordLoginType': [value: PasswordCodeLoginType]
  'update:passwordCode': [value: string]
  submit: []
  sendCode: []
}>()
const { t } = useI18n()

const form = reactive<ChangePasswordInput>({ ...props.passwordForm })
watch(
  () => props.passwordForm,
  (value) => Object.assign(form, value),
  { deep: true },
)
function updateCurrentPassword(value: string): void {
  form.currentPassword = value
  emit('update:passwordForm', { ...form })
}

function updateNewPassword(value: string): void {
  form.newPassword = value
  emit('update:passwordForm', { ...form })
}

function updateConfirmPassword(value: string): void {
  form.confirmPassword = value
  emit('update:passwordForm', { ...form })
}

function updatePasswordMode(value: unknown): void {
  if (value === 'password' || value === 'code') {
    emit('update:passwordMode', value)
  }
}

function updatePasswordLoginType(value: unknown): void {
  if (value === 'email' || value === 'phone') {
    emit('update:passwordLoginType', value)
  }
}

function updatePasswordCode(value: string): void {
  emit('update:passwordCode', value)
}
</script>

<template>
  <section class="account-profile__card account-profile__card--narrow">
    <header class="account-profile__card-head">
      <h2 class="account-profile__card-title">
        {{ t(setPasswordMode ? 'user.password.setTitle' : 'user.password.title') }}
      </h2>
    </header>
    <el-form label-position="top" @submit.prevent="emit('submit')">
      <el-form-item v-if="!setPasswordMode" :label="t('user.password.method')">
        <el-segmented
          :model-value="passwordMode"
          :options="passwordModeOptions"
          data-testid="account-password-mode"
          @update:model-value="updatePasswordMode"
        />
      </el-form-item>
      <el-form-item
        v-if="!setPasswordMode && passwordMode === 'password'"
        :label="t('user.password.current')"
        ><el-input
          :model-value="form.currentPassword"
          data-testid="account-password-current"
          type="password"
          show-password
          autocomplete="current-password"
          :placeholder="t('user.password.currentPlaceholder')"
          @update:model-value="updateCurrentPassword"
      /></el-form-item>
      <template v-if="!setPasswordMode && passwordMode === 'code'">
        <el-alert
          v-if="passwordIdentityOptions.length === 0"
          type="warning"
          :title="t('user.password.noIdentity')"
          :closable="false"
          show-icon
        />
        <el-form-item v-else :label="t('user.password.codeIdentity')">
          <el-segmented
            :model-value="passwordLoginType"
            :options="passwordIdentityOptions"
            data-testid="account-password-login-type"
            @update:model-value="updatePasswordLoginType"
          />
        </el-form-item>
        <el-form-item :label="t('user.password.code')">
          <div class="account-profile__code-row">
            <el-input
              :model-value="passwordCode"
              data-testid="account-password-code"
              maxlength="6"
              inputmode="numeric"
              :placeholder="t('user.password.codePlaceholder')"
              @update:model-value="updatePasswordCode"
            />
            <el-button
              data-testid="account-password-code-send"
              :loading="sendingPasswordCode"
              :disabled="passwordResendSeconds > 0 || passwordIdentityOptions.length === 0"
              @click="emit('sendCode')"
            >
              {{
                passwordResendSeconds > 0
                  ? t('user.password.resendIn', { seconds: passwordResendSeconds })
                  : t('user.password.sendCode')
              }}
            </el-button>
          </div>
        </el-form-item>
      </template>
      <el-form-item :label="t('user.password.new')"
        ><el-input
          :model-value="form.newPassword"
          data-testid="account-password-new"
          type="password"
          show-password
          autocomplete="new-password"
          :placeholder="t('user.password.newPlaceholder')"
          @update:model-value="updateNewPassword"
      /></el-form-item>
      <el-form-item :label="t('user.password.confirm')"
        ><el-input
          :model-value="form.confirmPassword"
          data-testid="account-password-confirm"
          type="password"
          show-password
          autocomplete="new-password"
          :placeholder="t('user.password.confirmPlaceholder')"
          @update:model-value="updateConfirmPassword"
      /></el-form-item>
      <div v-if="canUpdatePassword" class="account-profile__actions">
        <el-button
          data-testid="account-password-submit"
          type="primary"
          :loading="changingPassword"
          @click="emit('submit')"
          >{{ t(setPasswordMode ? 'user.password.setSubmit' : 'user.password.submit') }}</el-button
        >
      </div>
    </el-form>
  </section>
</template>

<style scoped>
.account-profile__card {
  padding: 26px 28px;
  background: var(--admin-surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  box-shadow: var(--admin-shadow-sm);
}

.account-profile__card--narrow {
  max-width: 520px;
}

.account-profile__card-head {
  margin-bottom: 18px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--admin-border);
}

.account-profile__card-title {
  margin: 0;
  color: var(--admin-text);
  font-size: 16px;
  font-weight: 700;
}

.account-profile__code-row {
  display: grid;
  width: 100%;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

.account-profile__actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 6px;
}

@media (max-width: 560px) {
  .account-profile__card {
    padding: 20px 18px;
  }

  .account-profile__code-row {
    grid-template-columns: 1fr;
  }
}
</style>
