<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Lock, User } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { getCurrentUser, login } from '../../../api/auth'
import { useAuthStore } from '../../../store/auth'
import { ApiError, ProtocolError } from '../../../types/http'

interface LoginForm {
  email: string
  password: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()
const formReference = ref<FormInstance>()
const form = reactive<LoginForm>({ email: '', password: '' })
const pending = ref(false)
const submitError = ref('')
const bootstrapError = computed(() => auth.status === 'error' ? auth.errorMessage : '')
const rules = computed<FormRules<LoginForm>>(() => ({
  email: [
    { required: true, message: t('auth.login.emailRequired'), trigger: 'blur' },
    { type: 'email', message: t('auth.login.emailInvalid'), trigger: 'blur' },
  ],
  password: [{ required: true, message: t('auth.login.passwordRequired'), trigger: 'blur' }],
}))

async function submit(): Promise<void> {
  if (pending.value || formReference.value === undefined) return
  if (form.email.trim() === '' || form.password === '') return
  const valid = await formReference.value.validate().catch(() => false)
  if (!valid) return
  pending.value = true
  submitError.value = ''
  try {
    const credential = await login({ email: form.email.trim(), password: form.password })
    auth.setCredential(credential)
    const currentUser = await getCurrentUser()
    auth.setAuthenticated(currentUser)
    await router.replace(safeRedirect(route.query.redirect))
  } catch (error: unknown) {
    auth.setAnonymous()
    submitError.value = error instanceof ProtocolError
      ? t('request.protocolError')
      : error instanceof ApiError && error.code === 10002
      ? t('auth.login.invalidCredentials')
      : error instanceof Error && error.message !== '' ? error.message : t('auth.login.failed')
  } finally {
    pending.value = false
  }
}

function safeRedirect(value: unknown): string {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//')
    ? value
    : '/dashboard'
}
</script>

<template>
  <main class="auth-page">
    <el-row class="auth-shell">
      <el-col :xs="24" :sm="24" :md="12" class="auth-brand-col">
      <section class="auth-brand" data-testid="login-brand" :aria-label="t('navigation.admin')">
        <div class="auth-brand__identity">
          <span class="auth-brand__mark" aria-hidden="true">A</span>
          <span class="auth-brand__name">Admin</span>
        </div>

        <div class="auth-brand__message">
          <p class="auth-brand__eyebrow">{{ t('auth.login.eyebrow') }}</p>
          <h1>{{ t('auth.login.heading') }}</h1>
          <p>{{ t('auth.login.description') }}</p>
        </div>

        <div class="auth-brand__trace" aria-hidden="true">
          <span></span>
          <span></span>
          <span></span>
        </div>
      </section>
      </el-col>

      <el-col :xs="24" :sm="24" :md="12" class="auth-form-col">
      <div class="auth-form-area">
        <section class="auth-panel" data-testid="login-panel" aria-labelledby="login-title">
          <header class="auth-panel__header">
            <el-icon class="auth-icon"><User /></el-icon>
            <div>
              <p class="auth-panel__eyebrow">{{ t('auth.login.eyebrow') }}</p>
              <h2 id="login-title">{{ t('auth.login.title') }}</h2>
            </div>
          </header>
          <p class="auth-caption">{{ t('auth.login.caption') }}</p>

          <p v-if="bootstrapError" class="auth-error" data-testid="bootstrap-error">{{ bootstrapError }}</p>
          <p v-if="submitError" class="auth-error" data-testid="login-error">{{ submitError }}</p>

          <el-form
            ref="formReference"
            class="auth-form"
            :model="form"
            :rules="rules"
            label-position="top"
            @submit.prevent="submit"
          >
            <el-form-item :label="t('auth.login.email')" prop="email">
              <el-input
                v-model="form.email"
                data-testid="login-email"
                type="email"
                inputmode="email"
                autocomplete="username"
                :placeholder="t('auth.login.emailPlaceholder')"
                size="large"
              >
                <template #prefix><el-icon><User /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item :label="t('auth.login.password')" prop="password">
              <el-input
                v-model="form.password"
                data-testid="login-password"
                type="password"
                autocomplete="current-password"
                :placeholder="t('auth.login.passwordPlaceholder')"
                size="large"
                show-password
              >
                <template #prefix><el-icon><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-button
              data-testid="login-submit"
              class="auth-submit"
              type="primary"
              native-type="submit"
              size="large"
              :loading="pending"
              :disabled="pending"
            >
              {{ t('auth.login.submit') }}
            </el-button>
          </el-form>

          <p class="auth-access-note">
            <el-icon><Lock /></el-icon>{{ t('auth.login.authorizedOnly') }}
          </p>
        </section>
      </div>
      </el-col>
    </el-row>
  </main>
</template>

<style scoped lang="scss">
.auth-page {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 0;
  padding: 32px;
  overflow: auto;
  color: var(--el-text-color-primary);
  background: var(--el-bg-color-page);
}

.auth-shell {
  display: flex;
  width: min(1080px, 100%);
  min-height: 620px;
  overflow: hidden;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  box-shadow: var(--el-box-shadow-lighter);
}

.auth-brand-col,
.auth-form-col {
  display: flex;
  min-width: 0;
}

.auth-brand-col {
  border-right: 1px solid var(--el-border-color-light);
}

.auth-brand {
  position: relative;
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 48px;
  overflow: hidden;
  background: var(--el-fill-color-light);
}

.auth-brand::after {
  position: absolute;
  right: 48px;
  bottom: 48px;
  width: 132px;
  height: 132px;
  content: '';
  border: 1px solid var(--el-border-color);
  border-right-color: transparent;
  border-bottom-color: transparent;
}

.auth-brand__identity {
  display: flex;
  align-items: center;
  gap: 12px;
}

.auth-brand__mark {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  color: var(--el-color-white);
  background: var(--el-color-primary);
  border-radius: 6px;
  font-size: 16px;
  font-weight: 800;
}

.auth-brand__name {
  font-size: 16px;
  font-weight: 750;
}

.auth-brand__message {
  margin: auto 0;
  padding-bottom: 72px;
}

.auth-brand__eyebrow,
.auth-panel__eyebrow {
  margin: 0 0 12px;
  color: var(--el-color-primary);
  font-size: 11px;
  font-weight: 750;
}

.auth-brand h1 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 42px;
  font-weight: 760;
  line-height: 1.24;
}

.auth-brand__message > p:last-child {
  max-width: 360px;
  margin: 22px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 14px;
  line-height: 1.8;
}

.auth-brand__trace {
  position: absolute;
  right: 32px;
  bottom: 32px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.auth-brand__trace::before {
  position: absolute;
  right: 10px;
  left: 10px;
  height: 1px;
  content: '';
  background: var(--el-border-color);
}

.auth-brand__trace span {
  position: relative;
  width: 8px;
  height: 8px;
  background: var(--el-color-primary);
  border: 2px solid var(--el-bg-color);
  border-radius: 50%;
}

.auth-form-area {
  display: flex;
  align-items: center;
  padding: 48px;
  width: 100%;
}

.auth-panel {
  width: min(380px, 100%);
  margin: 0 auto;
}

.auth-panel__header {
  display: flex;
  align-items: center;
  gap: 14px;
}

.auth-panel__header > .auth-icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border-radius: 6px;
  font-size: 20px;
}

.auth-panel__eyebrow {
  margin-bottom: 4px;
}

.auth-panel h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 750;
}

.auth-caption {
  margin: 16px 0 30px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.auth-form :deep(.el-form-item) {
  margin-bottom: 22px;
}

.auth-form :deep(.el-form-item__label) {
  color: var(--el-text-color-regular);
  font-size: 13px;
  font-weight: 650;
}

.auth-form :deep(.el-input__wrapper) {
  border-radius: 6px;
}

.auth-submit {
  width: 100%;
  margin-top: 6px;
  border-radius: 6px;
  font-weight: 650;
}

.auth-access-note {
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 24px 0 0;
  gap: 6px;
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}

.auth-error {
  margin: 0 0 16px;
  padding: 10px 12px;
  color: var(--el-color-danger);
  background: var(--el-color-danger-light-9);
  border-left: 3px solid var(--el-color-danger);
  border-radius: 4px;
  font-size: 13px;
}

@media (max-width: 760px) {
  .auth-page {
    align-items: stretch;
    padding: 16px;
  }

  .auth-shell { min-height: 0; }

  .auth-brand-col { border-right: 0; }

  .auth-brand {
    min-height: 84px;
    padding: 22px 24px;
    border-bottom: 1px solid var(--el-border-color-light);
  }

  .auth-brand__message,
  .auth-brand__trace,
  .auth-brand::after {
    display: none;
  }

  .auth-form-area {
    padding: 42px 24px;
  }
}

@media (max-width: 420px) {
  .auth-page {
    padding: 0;
  }

  .auth-shell {
    min-height: 100dvh;
    border: 0;
    border-radius: 0;
    box-shadow: none;
  }

  .auth-form-area {
    padding: 34px 20px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-page *,
  .auth-page *::before,
  .auth-page *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
  }
}
</style>
