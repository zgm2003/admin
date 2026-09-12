<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { CircleCheckFilled, Lock, User } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { forgotPassword, resetPassword } from '@/api/auth/login'
import logoUrl from '@/assets/logo.png'
import AuthDock from '@/views/auth/components/AuthDock/index.vue'
import CaptchaDialog from '@/views/auth/components/CaptchaDialog/index.vue'

type RecoveryLoginType = 'email' | 'phone'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const account = ref('')
const loginType = ref<RecoveryLoginType>('email')
const code = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const codeSent = ref(false)
const challengeId = ref('')
const sending = ref(false)
const pending = ref(false)
const submitError = ref('')
const resendSeconds = ref(0)
const captchaVisible = ref(false)
let countdownTimer: ReturnType<typeof setInterval> | undefined
const brandPoints = computed(() => [
  t('auth.brand.pointOne'),
  t('auth.brand.pointTwo'),
  t('auth.brand.pointThree'),
])
const accountLabel = computed(() => t(loginType.value === 'email' ? 'auth.forgot.email' : 'auth.forgot.phone'))
const accountPlaceholder = computed(() => t(loginType.value === 'email' ? 'auth.forgot.emailPlaceholder' : 'auth.forgot.phonePlaceholder'))

onMounted(() => {
  const presetAccount = route.query.account
  if (typeof presetAccount === 'string' && presetAccount !== '') {
    account.value = presetAccount
  }
})

onUnmounted(() => {
  if (countdownTimer !== undefined) clearInterval(countdownTimer)
})

watch(loginType, () => {
  account.value = ''
  code.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  codeSent.value = false
  challengeId.value = ''
  submitError.value = ''
  resendSeconds.value = 0
  if (countdownTimer !== undefined) {
    clearInterval(countdownTimer)
    countdownTimer = undefined
  }
})

function isDigitChar(char: string): boolean {
  return /^\d$/.test(char)
}

function sendCode(): void {
  if (sending.value || resendSeconds.value > 0) return
  const target = account.value.trim()
  if (target === '') {
    submitError.value = t('auth.forgot.emailRequired')
    return
  }
  submitError.value = ''
  captchaVisible.value = true
}

async function completeCaptcha(value: { captchaId: string; captchaAnswer: { x: number; y: number } }): Promise<void> {
  if (sending.value) return
  sending.value = true
  try {
    const result = await forgotPassword(account.value.trim(), loginType.value, value)
    challengeId.value = result.challengeId
    codeSent.value = true
    resendSeconds.value = result.resendAfterSeconds
    startCountdown()
    captchaVisible.value = false
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
  const target = account.value.trim()
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
      account: target,
      loginType: loginType.value,
      challengeId: challengeId.value,
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
    <AuthDock />

    <div class="auth-stage">
      <section class="auth-brand" data-testid="forgot-brand" :aria-label="t('navigation.admin')">
        <header class="auth-brand__identity auth-anim" style="--d: 0ms">
          <img class="auth-brand__logo" :src="logoUrl" :alt="t('navigation.admin')" />
          <div>
            <span class="auth-brand__name">{{ t('navigation.admin') }}</span>
            <span class="auth-brand__tag">{{ t('auth.forgot.eyebrow') }}</span>
          </div>
        </header>

        <div class="auth-brand__message">
          <h1 class="auth-anim" style="--d: 90ms">{{ t('auth.forgot.heading') }}</h1>
          <p class="auth-anim" style="--d: 180ms">{{ t('auth.forgot.description') }}</p>
        </div>

        <ul class="auth-brand__points">
          <li
            v-for="(point, index) in brandPoints"
            :key="point"
            class="auth-anim"
            :style="{ '--d': `${270 + index * 90}ms` }"
          >
            <span class="auth-brand__point-icon" aria-hidden="true">
              <el-icon><CircleCheckFilled /></el-icon>
            </span>
            <span>{{ point }}</span>
          </li>
        </ul>
      </section>

      <section class="auth-panel-wrap">
        <div
          class="auth-panel auth-anim"
          style="--d: 140ms"
          data-testid="forgot-panel"
          aria-labelledby="forgot-title"
        >
          <header class="auth-panel__header">
            <img class="auth-panel__logo" :src="logoUrl" :alt="t('navigation.admin')" />
            <div>
              <p class="auth-panel__eyebrow">{{ t('auth.forgot.eyebrow') }}</p>
              <h2 id="forgot-title">{{ t('auth.forgot.title') }}</h2>
            </div>
          </header>
          <p class="auth-caption">{{ t('auth.forgot.caption') }}</p>

          <el-radio-group v-model="loginType" class="auth-login-type" size="large">
            <el-radio-button value="email">{{ t('loginType.email') }}</el-radio-button>
            <el-radio-button value="phone">{{ t('loginType.phone') }}</el-radio-button>
          </el-radio-group>

          <p v-if="submitError" class="auth-error" data-testid="forgot-error">
            {{ submitError }}
          </p>

          <el-form class="auth-form" label-position="top" @submit.prevent="submit">
            <el-form-item :label="accountLabel">
              <div class="auth-code-row">
                <el-input
                  v-model="account"
                  data-testid="forgot-email"
                  :type="loginType === 'phone' ? 'tel' : 'email'"
                  :inputmode="loginType === 'phone' ? 'tel' : 'email'"
                  autocomplete="username"
                  :placeholder="accountPlaceholder"
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
                <el-input-otp
                  v-model="code"
                  data-testid="forgot-code"
                  class="auth-otp"
                  :length="6"
                  :validator="isDigitChar"
                />
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
        </div>
      </section>
    </div>
    <CaptchaDialog v-model="captchaVisible" :loading="sending" @complete="completeCaptcha" />
  </main>
</template>

<style scoped src="../authPage.css"></style>
