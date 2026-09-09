<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref, watch } from 'vue'
import { CircleCheckFilled, Lock, RefreshRight, User } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ElLink, ElMessage, ElNotification } from 'element-plus'

import {
  getCurrentUser,
  getLoginConfig,
  login,
  sendLoginCode,
  type LoginConfigOption,
  type LoginType,
} from '@/api/auth/login'
import logoUrl from '@/assets/logo.png'
import { useAuthStore } from '@/store/auth'
import { ApiError } from '@/types/http'
import AuthDock from '@/views/auth/components/AuthDock/index.vue'

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
const allowRegister = ref(false)
const activeType = ref<LoginType>()
const loading = ref(false)
const configFailed = ref(false)
const pending = ref(false)
const submitError = ref('')
const sending = ref(false)
const resendSeconds = ref(0)
const challengeId = ref(generateChallengeID())
let countdownTimer: ReturnType<typeof setInterval> | undefined
const bootstrapError = computed(() => (auth.status === 'error' ? auth.errorMessage : ''))
const brandPoints = computed(() => [
  t('auth.brand.pointOne'),
  t('auth.brand.pointTwo'),
  t('auth.brand.pointThree'),
])

const passwordMode = computed(() => activeType.value === 'password')
const codeMode = computed(() => activeType.value === 'email')

watch(activeType, () => {
  submitError.value = ''
})

onMounted(() => {
  const presetAccount = route.query.account
  if (typeof presetAccount === 'string' && presetAccount !== '') {
    form.value.account = presetAccount
  }
  void loadLoginConfig()
})

async function loadLoginConfig(): Promise<void> {
  if (loading.value) return
  loading.value = true
  try {
    const config = await getLoginConfig()
    options.value = config.loginTypes
    allowRegister.value = config.allowRegister
    activeType.value = config.loginTypes[0]?.value
    configFailed.value = false
  } catch {
    options.value = []
    allowRegister.value = false
    activeType.value = undefined
    configFailed.value = true
  } finally {
    loading.value = false
  }
}

onUnmounted(() => {
  if (countdownTimer !== undefined) clearInterval(countdownTimer)
})

function isDigitChar(char: string): boolean {
  return /^\d$/.test(char)
}

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
    resendSeconds.value = result.resendAfterSeconds
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
    showLoginSuccess(credential.isNewUser)
    if (currentUser.passwordSetRequired && !credential.isNewUser) {
      ElMessage.info(t('user.password.setupReminder'))
    }
  } catch (error: unknown) {
    auth.setAnonymous()
    submitError.value =
      error instanceof ApiError && error.code === 10002 ? t('auth.login.invalidCredentials') : ''
  } finally {
    pending.value = false
  }
}

function showLoginSuccess(isNewUser: boolean): void {
  if (!isNewUser) {
    ElNotification.success({ title: t('auth.login.success') })
    return
  }

  const profileLocation = router.resolve('/user/profile')
  ElNotification.success({
    title: t('auth.login.registeredSuccess'),
    message: h('span', [
      `${t('auth.login.registeredDescription')} `,
      h(
        ElLink,
        {
          type: 'primary',
          href: profileLocation.href,
          onClick: (event: MouseEvent) => {
            event.preventDefault()
            void router.push('/user/profile')
          },
        },
        () => t('auth.login.openProfile'),
      ),
    ]),
    duration: 8000,
  })
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
    <AuthDock />

    <div class="auth-stage">
      <section class="auth-brand" data-testid="login-brand" :aria-label="t('navigation.admin')">
        <header class="auth-brand__identity auth-anim" style="--d: 0ms">
          <img class="auth-brand__logo" :src="logoUrl" :alt="t('navigation.admin')" />
          <div>
            <span class="auth-brand__name">{{ t('navigation.admin') }}</span>
            <span class="auth-brand__tag">{{ t('auth.login.eyebrow') }}</span>
          </div>
        </header>

        <div class="auth-brand__message">
          <h1 class="auth-anim" style="--d: 90ms">{{ t('auth.login.heading') }}</h1>
          <p class="auth-anim" style="--d: 180ms">{{ t('auth.login.description') }}</p>
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
          data-testid="login-panel"
          aria-labelledby="login-title"
        >
          <header class="auth-panel__header">
            <img class="auth-panel__logo" :src="logoUrl" :alt="t('navigation.admin')" />
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

          <div
            v-if="configFailed"
            class="auth-config-error"
            data-testid="login-config-error"
            role="status"
          >
            <p>{{ t('auth.login.configUnavailable') }}</p>
            <el-button
              data-testid="login-config-retry"
              type="primary"
              plain
              :loading="loading"
              :disabled="loading"
              @click="loadLoginConfig"
            >
              <el-icon><RefreshRight /></el-icon>
              {{ t('auth.login.configRetry') }}
            </el-button>
          </div>

          <el-form
            v-if="!loading && options.length > 0"
            class="auth-form"
            :model="form"
            label-position="top"
            @submit.prevent="submit"
          >
            <el-tabs v-if="options.length > 0" v-model="activeType" class="auth-login-type" stretch>
              <el-tab-pane
                v-for="option in options"
                :key="option.value"
                :label="option.label"
                :name="option.value"
              />
            </el-tabs>

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

            <el-form-item v-else-if="codeMode">
              <template #label>
                <div class="auth-code-label">
                  <span>{{ t('auth.login.code') }}</span>
                  <el-button
                    data-testid="login-send-code"
                    text
                    type="primary"
                    :disabled="sending || resendSeconds > 0 || form.account.trim() === ''"
                    :loading="sending"
                    @click="sendCode"
                  >
                    {{
                      resendSeconds > 0
                        ? t('auth.login.resendIn', { seconds: resendSeconds })
                        : t('auth.login.sendCode')
                    }}
                  </el-button>
                </div>
              </template>
              <el-input-otp
                v-model="form.code"
                data-testid="login-code"
                class="auth-otp"
                :length="6"
                :validator="isDigitChar"
              />
            </el-form-item>
            <p v-if="codeMode && allowRegister" class="auth-hint" data-testid="login-register-hint">
              {{ t('auth.login.emailAutoRegister') }}
            </p>

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

            <div class="auth-forgot">
              <router-link data-testid="login-forgot-link" :to="{ path: '/forgotPassword' }">
                {{ t('auth.login.forgotPassword') }}
              </router-link>
            </div>
          </el-form>

          <p class="auth-access-note">
            <el-icon><Lock /></el-icon>{{ t('auth.login.authorizedOnly') }}
          </p>
        </div>
      </section>
    </div>
  </main>
</template>

<style scoped src="../authPage.css"></style>
