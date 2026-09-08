<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Lock, Message, User } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { forgotPassword, resetPassword } from '@/api/auth/login'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const email = ref('')
const code = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const codeSent = ref(false)
const sending = ref(false)
const pending = ref(false)
const submitError = ref('')
const resendSeconds = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  const presetAccount = route.query.account
  if (typeof presetAccount === 'string' && presetAccount !== '') {
    email.value = presetAccount
  }
})

onUnmounted(() => {
  if (countdownTimer !== undefined) clearInterval(countdownTimer)
})

async function sendCode(): Promise<void> {
  if (sending.value || resendSeconds.value > 0) return
  const target = email.value.trim()
  if (target === '') {
    submitError.value = t('auth.forgot.emailRequired')
    return
  }
  sending.value = true
  submitError.value = ''
  try {
    const result = await forgotPassword(target)
    codeSent.value = true
    resendSeconds.value = result.resendAfterSeconds
    startCountdown()
  } catch {
    // request.ts owns API error notifications.
  } finally {
    sending.value = false
  }
}

function startCountdown(): void {
  if (countdownTimer !== undefined) clearInterval(countdownTimer)
  countdownTimer = setInterval(() => {
    if (resendSeconds.value <= 1) {
      resendSeconds.value = 0
      if (countdownTimer !== undefined) clearInterval(countdownTimer)
      return
    }
    resendSeconds.value -= 1
  }, 1000)
}

async function submit(): Promise<void> {
  if (pending.value) return
  const target = email.value.trim()
  if (target === '') {
    submitError.value = t('auth.forgot.emailRequired')
    return
  }
  if (code.value.trim() === '') {
    submitError.value = t('auth.forgot.codeRequired')
    return
  }
  if (newPassword.value === '') {
    submitError.value = t('auth.forgot.passwordRequired')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    submitError.value = t('auth.forgot.mismatch')
    return
  }
  pending.value = true
  submitError.value = ''
  try {
    await resetPassword({
      email: target,
      code: code.value.trim(),
      newPassword: newPassword.value,
      confirmPassword: confirmPassword.value,
    })
    ElMessage.success(t('auth.forgot.successMessage'))
    await router.replace({ path: '/login', query: { account: target } })
  } catch {
    // request.ts owns API error notifications.
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <main class="auth-page">
    <el-row class="auth-shell">
      <el-col :xs="24" :sm="24" :md="12" class="auth-brand-col">
        <section class="auth-brand" data-testid="forgot-brand" :aria-label="t('navigation.admin')">
          <div class="auth-brand__identity">
            <span class="auth-brand__mark" aria-hidden="true">A</span>
            <span class="auth-brand__name">Admin</span>
          </div>

          <div class="auth-brand__message">
            <p class="auth-brand__eyebrow">{{ t('auth.forgot.eyebrow') }}</p>
            <h1>{{ t('auth.forgot.heading') }}</h1>
            <p>{{ t('auth.forgot.description') }}</p>
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
          <section class="auth-panel" data-testid="forgot-panel" aria-labelledby="forgot-title">
            <header class="auth-panel__header">
              <el-icon class="auth-icon"><Message /></el-icon>
              <div>
                <p class="auth-panel__eyebrow">{{ t('auth.forgot.eyebrow') }}</p>
                <h2 id="forgot-title">{{ t('auth.forgot.title') }}</h2>
              </div>
            </header>
            <p class="auth-caption">{{ t('auth.forgot.caption') }}</p>

            <p v-if="submitError" class="auth-error" data-testid="forgot-error">
              {{ submitError }}
            </p>

            <el-form class="auth-form" label-position="top" @submit.prevent="submit">
              <el-form-item :label="t('auth.forgot.email')">
                <div class="auth-code-row">
                  <el-input
                    v-model="email"
                    data-testid="forgot-email"
                    type="email"
                    inputmode="email"
                    autocomplete="username"
                    :placeholder="t('auth.forgot.emailPlaceholder')"
                    size="large"
                  >
                    <template #prefix
                      ><el-icon><User /></el-icon
                    ></template>
                  </el-input>
                  <el-button
                    data-testid="forgot-send-code"
                    :disabled="sending || resendSeconds > 0"
                    :loading="sending"
                    size="large"
                    @click="sendCode"
                  >
                    {{
                      resendSeconds > 0
                        ? t('auth.forgot.resendIn', { seconds: resendSeconds })
                        : t('auth.forgot.sendCode')
                    }}
                  </el-button>
                </div>
              </el-form-item>

              <template v-if="codeSent">
                <el-form-item :label="t('auth.forgot.code')">
                  <el-input
                    v-model="code"
                    data-testid="forgot-code"
                    inputmode="numeric"
                    maxlength="6"
                    :placeholder="t('auth.forgot.codePlaceholder')"
                    size="large"
                  >
                    <template #prefix
                      ><el-icon><Message /></el-icon
                    ></template>
                  </el-input>
                </el-form-item>

                <el-form-item :label="t('auth.forgot.newPassword')">
                  <el-input
                    v-model="newPassword"
                    data-testid="forgot-new-password"
                    type="password"
                    autocomplete="new-password"
                    :placeholder="t('auth.forgot.newPasswordPlaceholder')"
                    size="large"
                    show-password
                  >
                    <template #prefix
                      ><el-icon><Lock /></el-icon
                    ></template>
                  </el-input>
                </el-form-item>

                <el-form-item :label="t('auth.forgot.confirmPassword')">
                  <el-input
                    v-model="confirmPassword"
                    data-testid="forgot-confirm-password"
                    type="password"
                    autocomplete="new-password"
                    :placeholder="t('auth.forgot.confirmPasswordPlaceholder')"
                    size="large"
                    show-password
                  >
                    <template #prefix
                      ><el-icon><Lock /></el-icon
                    ></template>
                  </el-input>
                </el-form-item>

                <el-button
                  data-testid="forgot-submit"
                  class="auth-submit"
                  type="primary"
                  native-type="submit"
                  size="large"
                  :loading="pending"
                  :disabled="pending"
                >
                  {{ t('auth.forgot.submit') }}
                </el-button>
              </template>
            </el-form>

            <div class="auth-forgot">
              <router-link data-testid="forgot-back-login" :to="{ path: '/login' }">
                {{ t('auth.forgot.backToLogin') }}
              </router-link>
            </div>

            <p class="auth-access-note">
              <el-icon><Lock /></el-icon>{{ t('auth.forgot.authorizedOnly') }}
            </p>
          </section>
        </div>
      </el-col>
    </el-row>
  </main>
</template>

<style scoped src="../login/LoginPage.css"></style>
