<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Lock, Message, User } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import {
  getCurrentUser,
  getLoginConfig,
  login,
  sendLoginCode,
  type LoginConfigOption,
  type LoginType,
} from '@/api/auth/login'
import { useAuthStore } from '@/store/auth'
import { ApiError } from '@/types/http'

interface LoginForm {
  account: string
  password: string
  code: string
}

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const auth = useAuthStore()
const form = ref<LoginForm>({ account: '', password: '', code: '' })
const options = ref<LoginConfigOption[]>([])
const activeType = ref<LoginType>()
const loading = ref(false)
const pending = ref(false)
const submitError = ref('')
const sending = ref(false)
const resendSeconds = ref(0)
const challengeId = ref(generateChallengeID())
let countdownTimer: ReturnType<typeof setInterval> | undefined
const bootstrapError = computed(() => (auth.status === 'error' ? auth.errorMessage : ''))

const passwordMode = computed(() => activeType.value === 'password')
const codeMode = computed(() => activeType.value === 'email')

watch(activeType, () => {
  submitError.value = ''
})

onMounted(async () => {
  loading.value = true
  try {
    const config = await getLoginConfig()
    options.value = config.loginTypes
    activeType.value = config.loginTypes[0]?.value
  } catch {
    options.value = []
    activeType.value = undefined
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  if (countdownTimer !== undefined) clearInterval(countdownTimer)
})

async function sendCode(): Promise<void> {
  if (sending.value || resendSeconds.value > 0) return
  if (form.value.account.trim() === '') {
    submitError.value = t('auth.login.accountRequired')
    return
  }
  sending.value = true
  submitError.value = ''
  try {
    const result = await sendLoginCode(
      form.value.account.trim(),
      'email',
      'login',
      challengeId.value,
    )
    const seconds = Math.max(
      1,
      Math.floor((new Date(result.expiresAt).getTime() - Date.now()) / 1000),
    )
    resendSeconds.value = Math.min(seconds, 900)
    startCountdown()
  } catch {
    // request.ts owns API error notifications.
  } finally {
    challengeId.value = generateChallengeID()
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
  const loginType = activeType.value
  if (loginType === undefined) return
  if (form.value.account.trim() === '') {
    submitError.value = t('auth.login.accountRequired')
    return
  }
  if (passwordMode.value && form.value.password === '') {
    submitError.value = t('auth.login.passwordRequired')
    return
  }
  if (codeMode.value && form.value.code.trim() === '') {
    submitError.value = t('auth.login.codeRequired')
    return
  }
  pending.value = true
  submitError.value = ''
  try {
    const loginAccount = form.value.account.trim()
    const credential =
      loginType === 'password'
        ? await login({ loginType, loginAccount, password: form.value.password })
        : await login({ loginType, loginAccount, code: form.value.code.trim() })
    auth.setCredential(credential)
    const currentUser = await getCurrentUser()
    auth.setAuthenticated(currentUser)
    await router.replace(safeRedirect(route.query.redirect))
  } catch (error: unknown) {
    auth.setAnonymous()
    submitError.value =
      error instanceof ApiError && error.code === 10002 ? t('auth.login.invalidCredentials') : ''
  } finally {
    pending.value = false
  }
}

function safeRedirect(value: unknown): string {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//')
    ? value
    : '/dashboard'
}

function generateChallengeID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `c-${Date.now()}-${Math.random().toString(16).slice(2)}`
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

            <p v-if="bootstrapError" class="auth-error" data-testid="bootstrap-error">
              {{ bootstrapError }}
            </p>
            <p v-if="submitError" class="auth-error" data-testid="login-error">{{ submitError }}</p>

            <el-form
              v-if="!loading && options.length > 0"
              class="auth-form"
              :model="form"
              label-position="top"
              @submit.prevent="submit"
            >
              <el-segmented
                v-if="options.length > 0"
                v-model="activeType"
                class="auth-login-type"
                :options="options"
                block
              />

              <el-form-item :label="t('auth.login.account')">
                <el-input
                  v-model="form.account"
                  data-testid="login-account"
                  type="email"
                  inputmode="email"
                  autocomplete="username"
                  :placeholder="t('auth.login.accountPlaceholder')"
                  size="large"
                >
                  <template #prefix
                    ><el-icon><User /></el-icon
                  ></template>
                </el-input>
              </el-form-item>

              <el-form-item v-if="passwordMode" :label="t('auth.login.password')">
                <el-input
                  v-model="form.password"
                  data-testid="login-password"
                  type="password"
                  autocomplete="current-password"
                  :placeholder="t('auth.login.passwordPlaceholder')"
                  size="large"
                  show-password
                >
                  <template #prefix
                    ><el-icon><Lock /></el-icon
                  ></template>
                </el-input>
              </el-form-item>

              <el-form-item v-else-if="codeMode" :label="t('auth.login.code')">
                <div class="auth-code-row">
                  <el-input
                    v-model="form.code"
                    data-testid="login-code"
                    inputmode="numeric"
                    maxlength="6"
                    :placeholder="t('auth.login.codePlaceholder')"
                    size="large"
                  >
                    <template #prefix
                      ><el-icon><Message /></el-icon
                    ></template>
                  </el-input>
                  <el-button
                    data-testid="login-send-code"
                    :disabled="sending || resendSeconds > 0 || form.account.trim() === ''"
                    :loading="sending"
                    size="large"
                    @click="sendCode"
                  >
                    {{
                      resendSeconds > 0
                        ? t('auth.login.resendIn', { seconds: resendSeconds })
                        : t('auth.login.sendCode')
                    }}
                  </el-button>
                </div>
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

<style scoped src="./LoginPage.css"></style>
