<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  changePasswordByCode,
  changePassword,
  getAccountProfile,
  sendPasswordCode,
  setPassword,
  updateAccountProfile,
} from '@/api/user/profile'
import type {
  AccountProfile,
  ChangePasswordInput,
  PasswordCodeLoginType,
  UpdateAccountProfileInput,
} from '@/api/user/profile'
import { usePermissionStore } from '@/store/permission'
import { useAuthStore } from '@/store/auth'
import { useSystemDictionaryStore } from '@/store/systemDictionary'
import EmailBindingDialog from '@/views/user/profile/components/EmailBindingDialog/index.vue'
import PhoneBindingDialog from '@/views/user/profile/components/PhoneBindingDialog/index.vue'
import PasswordSecurityForm from '@/views/user/profile/components/PasswordSecurityForm/index.vue'
import ProfileDetailsForm from '@/views/user/profile/components/ProfileDetailsForm/index.vue'
import ProfileHero from '@/views/user/profile/components/ProfileHero/index.vue'

type ProfileTab = 'profile' | 'security'
type PasswordMode = 'password' | 'code'

const { t, locale } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const access = usePermissionStore()
const dictionaries = useSystemDictionaryStore()
const canUpdateProfile = computed(() => access.hasPermission('user:profile:update'))
const canUpdatePassword = computed(() => access.hasPermission('user:password:update'))
const canUpdateEmail = computed(() => access.hasPermission('user:email:update'))
const canUpdatePhone = computed(() => access.hasPermission('user:phone:update'))
const setPasswordMode = computed(() => auth.passwordSetRequired)
const loading = ref(false)
const savingProfile = ref(false)
const changingPassword = ref(false)
const sendingPasswordCode = ref(false)
const loadError = ref('')
const activeTab = ref<ProfileTab>(setPasswordMode.value ? 'security' : 'profile')
const heroAvatarURL = ref('')
const currentEmail = ref(auth.user?.email ?? '')
const currentPhone = ref<string | null>(auth.user?.phone ?? null)
const emailDialogVisible = ref(false)
const phoneDialogVisible = ref(false)
const passwordMode = ref<PasswordMode>('password')
const passwordLoginType = ref<PasswordCodeLoginType>('email')
const passwordChallengeID = ref('')
const passwordCode = ref('')
const passwordResendSeconds = ref(0)
let passwordResendTimer: number | undefined
const genderOptions = ref<Array<{ label: string; value: UpdateAccountProfileInput['gender'] }>>([])
const genderOptionsLoading = ref(false)
const genderOptionsError = ref('')
let genderOptionsRequest = 0
const profileForm = reactive<UpdateAccountProfileInput>({
  username: '',
  avatar: '',
  birthday: null,
  gender: 0,
})
const passwordForm = reactive<ChangePasswordInput>({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
})
const heroName = computed(() =>
  profileForm.username !== '' ? profileForm.username : (auth.user?.username ?? ''),
)
const heroEmail = computed(() => currentEmail.value)
const passwordModeOptions = computed(() => [
  { label: t('user.password.byCurrentPassword'), value: 'password' as const },
  { label: t('user.password.byCode'), value: 'code' as const },
])
const passwordIdentityOptions = computed<Array<{ label: string; value: PasswordCodeLoginType }>>(
  () => {
    const options: Array<{ label: string; value: PasswordCodeLoginType }> = []
    if (currentEmail.value !== '') {
      options.push({ label: t('user.profile.email'), value: 'email' })
    }
    if (currentPhone.value !== null) {
      options.push({ label: t('user.profile.phone'), value: 'phone' })
    }
    return options
  },
)

function applyProfile(profile: AccountProfile): void {
  profileForm.username = profile.username
  profileForm.avatar = profile.avatar
  profileForm.birthday = profile.birthday
  profileForm.gender = profile.gender
  currentEmail.value = profile.email
  currentPhone.value = profile.phone
}

async function loadProfile(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    applyProfile(await getAccountProfile())
  } catch (error: unknown) {
    loadError.value =
      error instanceof Error && error.message !== '' ? error.message : t('user.profile.loadFailed')
  } finally {
    loading.value = false
  }
}

async function loadGenderOptions(): Promise<void> {
  const request = ++genderOptionsRequest
  genderOptionsLoading.value = true
  genderOptionsError.value = ''
  try {
    await dictionaries.load(['user.gender'])
    if (request !== genderOptionsRequest) return
    const options = dictionaries.options('user.gender').value
    if (options === undefined) throw new Error('user.gender dictionary is not ready')
    const seen = new Set<number>()
    genderOptions.value = options.map((option) => {
      const value = parseGenderValue(option.value)
      if (seen.has(value)) throw new Error('user.gender dictionary contains duplicate values')
      seen.add(value)
      return { label: option.label, value }
    })
  } catch {
    if (request !== genderOptionsRequest) return
    genderOptions.value = []
    genderOptionsError.value = t('user.profile.genderOptionsLoadFailed')
  } finally {
    if (request === genderOptionsRequest) genderOptionsLoading.value = false
  }
}

function parseGenderValue(value: string): UpdateAccountProfileInput['gender'] {
  if (value === '0') return 0
  if (value === '1') return 1
  if (value === '2') return 2
  throw new Error('user.gender dictionary value is invalid')
}

async function saveProfile(): Promise<void> {
  if (savingProfile.value) return
  savingProfile.value = true
  try {
    const updated = await updateAccountProfile({ ...profileForm })
    applyProfile(updated)
    auth.updateProfile(updated.userId, updated.username, updated.phone, updated.avatar)
    ElMessage.success(t('user.profile.saved'))
  } catch {
    // request.ts emits the single API error notification
  } finally {
    savingProfile.value = false
  }
}

async function submitPassword(): Promise<void> {
  if (changingPassword.value) return
  changingPassword.value = true
  try {
    if (setPasswordMode.value) {
      await setPassword({
        newPassword: passwordForm.newPassword,
        confirmPassword: passwordForm.confirmPassword,
      })
      auth.markPasswordSet()
      passwordForm.currentPassword = ''
      passwordForm.newPassword = ''
      passwordForm.confirmPassword = ''
      ElMessage.success(t('user.password.setSuccessMessage'))
      return
    }
    if (passwordMode.value === 'code') {
      await changePasswordByCode({
        loginType: passwordLoginType.value,
        challengeId: passwordChallengeID.value,
        code: passwordCode.value,
        newPassword: passwordForm.newPassword,
        confirmPassword: passwordForm.confirmPassword,
      })
      passwordChallengeID.value = ''
      passwordCode.value = ''
      passwordForm.currentPassword = ''
      passwordForm.newPassword = ''
      passwordForm.confirmPassword = ''
      stopPasswordCountdown()
      ElMessage.success(t('user.password.codeSuccessMessage'))
      return
    }
    await changePassword({ ...passwordForm })
    await ElMessageBox.alert(t('user.password.successMessage'), t('user.password.successTitle'), {
      type: 'success',
    })
    access.reset()
    auth.setAnonymous()
    await router.replace({ name: 'login' })
  } catch {
    // request.ts emits the single API error notification
  } finally {
    changingPassword.value = false
  }
}

async function sendCurrentIdentityPasswordCode(): Promise<void> {
  if (
    sendingPasswordCode.value ||
    passwordResendSeconds.value > 0 ||
    !passwordIdentityOptions.value.some((option) => option.value === passwordLoginType.value)
  ) {
    return
  }
  sendingPasswordCode.value = true
  try {
    const result = await sendPasswordCode(passwordLoginType.value)
    passwordChallengeID.value = result.challengeId
    startPasswordCountdown(result.resendAfterSeconds)
  } catch {
    // request.ts emits the single API error notification
  } finally {
    sendingPasswordCode.value = false
  }
}

function startPasswordCountdown(seconds: number): void {
  stopPasswordCountdown()
  passwordResendSeconds.value = seconds
  if (seconds <= 0) return
  passwordResendTimer = window.setInterval(() => {
    passwordResendSeconds.value = Math.max(0, passwordResendSeconds.value - 1)
    if (passwordResendSeconds.value === 0) stopPasswordCountdown()
  }, 1000)
}

function stopPasswordCountdown(): void {
  if (passwordResendTimer !== undefined) {
    window.clearInterval(passwordResendTimer)
    passwordResendTimer = undefined
  }
  passwordResendSeconds.value = 0
}

function handleEmailUpdated(email: string): void {
  currentEmail.value = email
  if (auth.user !== null) auth.updateEmail(auth.user.userId, email)
}

function handlePhoneUpdated(phone: string): void {
  currentPhone.value = phone
  if (auth.user !== null) auth.updatePhone(auth.user.userId, phone)
}

watch(
  passwordIdentityOptions,
  (options) => {
    if (!options.some((option) => option.value === passwordLoginType.value) && options[0]) {
      passwordLoginType.value = options[0].value
      passwordChallengeID.value = ''
      passwordCode.value = ''
      stopPasswordCountdown()
    }
  },
  { immediate: true },
)
watch(passwordLoginType, () => {
  passwordChallengeID.value = ''
  passwordCode.value = ''
  stopPasswordCountdown()
})

void loadProfile()
watch(locale, () => void loadGenderOptions(), { immediate: true })
onBeforeUnmount(stopPasswordCountdown)
</script>

<template>
  <section class="account-profile" data-testid="account-profile-page">
    <el-alert v-if="loadError" type="error" :title="loadError" :closable="false" show-icon />

    <ProfileHero
      :name="heroName"
      :email="heroEmail"
      :avatar-url="heroAvatarURL"
      @avatar-error="heroAvatarURL = ''"
    />

    <div class="account-profile__tabs" role="tablist" :aria-label="t('layout.user.profile')">
      <button
        type="button"
        role="tab"
        data-testid="account-profile-tab-profile"
        class="account-profile__tab"
        :class="{ 'is-active': activeTab === 'profile' }"
        :aria-selected="activeTab === 'profile'"
        @click="activeTab = 'profile'"
      >
        {{ t('user.profile.tabProfile') }}
      </button>
      <button
        type="button"
        role="tab"
        data-testid="account-profile-tab-security"
        class="account-profile__tab"
        :class="{ 'is-active': activeTab === 'security' }"
        :aria-selected="activeTab === 'security'"
        @click="activeTab = 'security'"
      >
        {{ t('user.profile.tabSecurity') }}
      </button>
    </div>

    <div v-show="activeTab === 'profile'" class="account-profile__pane">
      <ProfileDetailsForm
        :profile-form="profileForm"
        :current-email="currentEmail"
        :current-phone="currentPhone"
        :gender-options="genderOptions"
        :gender-options-loading="genderOptionsLoading"
        :gender-options-error="genderOptionsError"
        :loading="loading"
        :can-update-profile="canUpdateProfile"
        :can-update-email="canUpdateEmail"
        :can-update-phone="canUpdatePhone"
        @update:profile-form="Object.assign(profileForm, $event)"
        @save="saveProfile"
        @email-action="emailDialogVisible = true"
        @phone-action="phoneDialogVisible = true"
        @avatar-preview-change="heroAvatarURL = $event"
      />
    </div>

    <div v-show="activeTab === 'security'" class="account-profile__pane">
      <PasswordSecurityForm
        :set-password-mode="setPasswordMode"
        :password-mode="passwordMode"
        :password-mode-options="passwordModeOptions"
        :password-login-type="passwordLoginType"
        :password-identity-options="passwordIdentityOptions"
        :password-form="passwordForm"
        :password-code="passwordCode"
        :password-resend-seconds="passwordResendSeconds"
        :changing-password="changingPassword"
        :sending-password-code="sendingPasswordCode"
        :can-update-password="canUpdatePassword"
        @update:password-form="Object.assign(passwordForm, $event)"
        @update:password-mode="passwordMode = $event"
        @update:password-login-type="passwordLoginType = $event"
        @update:password-code="passwordCode = $event"
        @submit="submitPassword"
        @send-code="sendCurrentIdentityPasswordCode"
      />
    </div>

    <EmailBindingDialog
      v-model="emailDialogVisible"
      :current-email="currentEmail"
      @updated="handleEmailUpdated"
    />
    <PhoneBindingDialog
      v-model="phoneDialogVisible"
      :current-phone="currentPhone"
      @updated="handlePhoneUpdated"
    />
  </section>
</template>

<style scoped>
.account-profile {
  display: grid;
  max-width: 960px;
  margin: 0 auto;
  gap: 16px;
}

/* Tab navigation. */
.account-profile__tabs {
  display: inline-flex;
  width: fit-content;
  max-width: 100%;
  padding: 4px;
  gap: 4px;
  background: var(--admin-surface);
  border: 1px solid var(--admin-border);
  border-radius: 8px;
  box-shadow: var(--admin-shadow-sm);
}

.account-profile__tab {
  padding: 8px 18px;
  color: var(--admin-text-soft);
  font: inherit;
  font-size: 13px;
  font-weight: 650;
  white-space: nowrap;
  border: 0;
  border-radius: 4px;
  background: transparent;
  cursor: pointer;
  transition:
    color 0.16s ease,
    background-color 0.16s ease,
    box-shadow 0.16s ease;
}

.account-profile__tab:hover {
  color: var(--admin-text);
}

.account-profile__tab.is-active {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-8) inset;
}

.account-profile__tab:focus-visible {
  outline: 2px solid var(--el-color-primary);
  outline-offset: 2px;
}

/* Form panels. */
.account-profile__pane {
  min-width: 0;
}

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

.account-profile__full {
  width: 100%;
}

.account-profile__identity-row,
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

  .account-profile__identity-row,
  .account-profile__code-row {
    grid-template-columns: 1fr;
  }
}
</style>
