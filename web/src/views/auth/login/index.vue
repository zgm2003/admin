<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { CircleCheckFilled, Lock, RefreshRight, User } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus/es/components/message/index'

import {
  getCurrentUser,
  getLoginConfig,
  login,
  sendLoginCode,
  type LoginConfigOption,
  type LoginType,
} from '@/api/auth/login'
import { getPublicLegalDocument, type LegalDocumentKind } from '@/api/system/setting'
import logoUrl from '@/assets/logo.png'
import AppCaptcha from '@/components/AppCaptcha/index.vue'
import AppDialog from '@/components/AppDialog/index.vue'
import { useAuthStore } from '@/store/auth'
import { ApiError } from '@/types/http'
import AuthDock from '@/views/auth/components/AuthDock/index.vue'
import {
  generateChallengeID,
  isDigitChar,
  safeRedirect,
  showLoginSuccess,
  type LoginForm,
} from './loginPage'

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
const deliveryChallengeId = ref(generateChallengeID())
const proofChallengeId = ref('')
const captchaVisible = ref(false)
const legalDialogVisible = ref(false)
const legalDocumentKind = ref<LegalDocumentKind>('privacyPolicy')
const legalDocumentContent = ref('')
const legalDocumentLoading = ref(false)
const legalDocumentError = ref('')
let countdownTimer: ReturnType<typeof setInterval> | undefined
let legalDocumentRequestSequence = 0
const bootstrapError = computed(() => (auth.status === 'error' ? auth.errorMessage : ''))
const brandPoints = computed(() => [
  t('auth.brand.pointOne'),
  t('auth.brand.pointTwo'),
  t('auth.brand.pointThree'),
])

const passwordMode = computed(() => activeType.value === 'password')
const codeMode = computed(() => activeType.value === 'email' || activeType.value === 'phone')
const accountInputType = computed(() => (activeType.value === 'phone' ? 'tel' : 'email'))
const accountInputMode = computed(() => (activeType.value === 'phone' ? 'tel' : 'email'))
const accountPlaceholder = computed(() =>
  activeType.value === 'phone'
    ? t('auth.login.phonePlaceholder')
    : t('auth.login.accountPlaceholder'),
)
const legalDocumentTitle = computed(() =>
  t(
    legalDocumentKind.value === 'privacyPolicy'
      ? 'auth.login.privacyPolicy'
      : 'auth.login.userAgreement',
  ),
)

watch(activeType, () => {
  submitError.value = ''
  form.value.code = ''
  deliveryChallengeId.value = generateChallengeID()
  proofChallengeId.value = ''
  resendSeconds.value = 0
  if (countdownTimer !== undefined) clearInterval(countdownTimer)
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

async function openLegalDocument(kind: LegalDocumentKind): Promise<void> {
  const requestSequence = ++legalDocumentRequestSequence
  legalDocumentKind.value = kind
  legalDialogVisible.value = true
  legalDocumentLoading.value = true
  legalDocumentContent.value = ''
  legalDocumentError.value = ''
  try {
    const document = await getPublicLegalDocument(kind)
    if (requestSequence !== legalDocumentRequestSequence) return
    if (document.contentHtml.trim() === '') {
      legalDocumentError.value = t('auth.login.legalNotConfigured')
      return
    }
    legalDocumentContent.value = document.contentHtml
  } catch {
    if (requestSequence !== legalDocumentRequestSequence) return
    legalDocumentError.value = t('auth.login.legalLoadFailed')
  } finally {
    if (requestSequence === legalDocumentRequestSequence) legalDocumentLoading.value = false
  }
}

onUnmounted(() => {
  if (countdownTimer !== undefined) clearInterval(countdownTimer)
})

function sendCode(): void {
  if (sending.value || resendSeconds.value > 0) return
  const loginType = activeType.value
  if (loginType !== 'email' && loginType !== 'phone') return
  if (form.value.account.trim() === '') {
    submitError.value = t('auth.login.accountRequired')
    return
  }
  submitError.value = ''
  captchaVisible.value = true
}

async function completeCaptcha(value: {
  captchaId: string
  captchaAnswer: { x: number; y: number }
}): Promise<void> {
  if (sending.value) return
  const loginType = activeType.value
  if (loginType !== 'email' && loginType !== 'phone') return
  sending.value = true
  try {
    const result = await sendLoginCode(
      form.value.account.trim(),
      loginType,
      'login',
      deliveryChallengeId.value,
      value,
    )
    proofChallengeId.value = result.challengeId
    resendSeconds.value = result.resendAfterSeconds
    startCountdown()
    captchaVisible.value = false
  } catch {
    // request.ts owns API error notifications.
  } finally {
    deliveryChallengeId.value = generateChallengeID()
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
        : await login({
            loginType,
            loginAccount,
            challengeId: proofChallengeId.value,
            code: form.value.code.trim(),
          })
    auth.setCredential(credential)
    const currentUser = await getCurrentUser()
    auth.setAuthenticated(currentUser)
    await router.replace(safeRedirect(route.query.redirect))
    showLoginSuccess({ isNewUser: credential.isNewUser, t, router })
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
              :icon="RefreshRight"
              @click="loadLoginConfig"
            >
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
                :type="accountInputType"
                :inputmode="accountInputMode"
                autocomplete="username"
                :placeholder="accountPlaceholder"
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
              {{
                t(
                  activeType === 'phone'
                    ? 'auth.login.phoneAutoRegister'
                    : 'auth.login.emailAutoRegister',
                )
              }}
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

          <div class="auth-legal-links" aria-label="legal documents">
            <el-button
              data-testid="login-user-agreement"
              link
              type="primary"
              @click="openLegalDocument('userAgreement')"
            >
              {{ t('auth.login.userAgreement') }}
            </el-button>
            <span aria-hidden="true">·</span>
            <el-button
              data-testid="login-privacy-policy"
              link
              type="primary"
              @click="openLegalDocument('privacyPolicy')"
            >
              {{ t('auth.login.privacyPolicy') }}
            </el-button>
          </div>

          <p class="auth-access-note">
            <el-icon><Lock /></el-icon>{{ t('auth.login.authorizedOnly') }}
          </p>
        </div>
      </section>
    </div>
    <AppCaptcha v-model="captchaVisible" :loading="sending" @complete="completeCaptcha" />
    <AppDialog
      v-model="legalDialogVisible"
      :title="legalDocumentTitle"
      width="760px"
      mobile-width="calc(100vw - 28px)"
      height="min(68vh, 640px)"
    >
      <el-skeleton v-if="legalDocumentLoading" :rows="8" animated />
      <el-alert
        v-else-if="legalDocumentError"
        data-testid="legal-document-empty"
        :title="legalDocumentError"
        type="info"
        :closable="false"
        show-icon
      />
      <article
        v-else
        class="auth-legal-document"
        data-testid="legal-document-content"
        v-html="legalDocumentContent"
      />
    </AppDialog>
  </main>
</template>

<style scoped src="../authPage.css"></style>
