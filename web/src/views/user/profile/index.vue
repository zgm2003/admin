<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'

import {
  changePassword,
  getAccountProfile,
  setPassword,
  updateAccountProfile,
} from '@/api/user/profile'
import type {
  AccountProfile,
  ChangePasswordInput,
  UpdateAccountProfileInput,
} from '@/api/user/profile'
import { usePermissionStore } from '@/store/permission'
import { useAuthStore } from '@/store/auth'
import { UpMedia } from '@/components/UpMedia'
import ProfileHero from '@/views/user/profile/components/ProfileHero/index.vue'

type ProfileTab = 'profile' | 'security'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const access = usePermissionStore()
const canUpdateProfile = computed(() => access.hasPermission('user:profile:update'))
const canUpdatePassword = computed(() => access.hasPermission('user:password:update'))
const setPasswordMode = computed(() => auth.passwordSetRequired)
const loading = ref(false)
const savingProfile = ref(false)
const changingPassword = ref(false)
const loadError = ref('')
const activeTab = ref<ProfileTab>(setPasswordMode.value ? 'security' : 'profile')
const heroAvatarURL = ref('')
const genderOptions = computed(() => [
  { label: t('user.profile.genderUnknown'), value: 0 },
  { label: t('user.profile.genderMale'), value: 1 },
  { label: t('user.profile.genderFemale'), value: 2 },
])
const profileForm = reactive<UpdateAccountProfileInput>({
  username: '',
  phone: null,
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
const heroEmail = computed(() => auth.user?.email ?? '')

function applyProfile(profile: AccountProfile): void {
  profileForm.username = profile.username
  profileForm.phone = profile.phone
  profileForm.avatar = profile.avatar
  profileForm.birthday = profile.birthday
  profileForm.gender = profile.gender
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

void loadProfile()
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
      <section v-loading="loading" class="account-profile__card">
        <header class="account-profile__card-head">
          <h2 class="account-profile__card-title">{{ t('user.profile.basicTitle') }}</h2>
        </header>
        <el-form label-position="top" @submit.prevent="saveProfile">
          <el-row :gutter="16">
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('user.profile.username')">
                <el-input v-model="profileForm.username" autocomplete="username" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('user.profile.email')">
                <el-input :model-value="heroEmail" disabled />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('user.profile.phone')">
                <el-input v-model="profileForm.phone" clearable />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('user.profile.avatar')">
                <UpMedia
                  v-model="profileForm.avatar"
                  rule-code="avatar"
                  variant="avatar"
                  width="178px"
                  :disabled="!canUpdateProfile"
                  @preview-change="heroAvatarURL = $event"
                />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('user.profile.birthday')">
                <el-date-picker
                  v-model="profileForm.birthday"
                  type="date"
                  value-format="YYYY-MM-DD"
                  class="account-profile__full"
                />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('user.profile.gender')">
                <el-select-v2
                  v-model="profileForm.gender"
                  :options="genderOptions"
                  data-testid="account-profile-gender"
                  class="account-profile__full"
                />
              </el-form-item>
            </el-col>
          </el-row>
          <div v-if="canUpdateProfile" class="account-profile__actions">
            <el-button
              data-testid="account-profile-save"
              type="primary"
              :loading="savingProfile"
              @click="saveProfile"
              >{{ t('user.profile.save') }}</el-button
            >
          </div>
        </el-form>
      </section>
    </div>

    <div v-show="activeTab === 'security'" class="account-profile__pane">
      <section class="account-profile__card account-profile__card--narrow">
        <header class="account-profile__card-head">
          <h2 class="account-profile__card-title">
            {{ t(setPasswordMode ? 'user.password.setTitle' : 'user.password.title') }}
          </h2>
        </header>
        <el-form label-position="top" @submit.prevent="submitPassword">
          <el-form-item v-if="!setPasswordMode" :label="t('user.password.current')"
            ><el-input
              v-model="passwordForm.currentPassword"
              data-testid="account-password-current"
              type="password"
              show-password
              autocomplete="current-password"
          /></el-form-item>
          <el-form-item :label="t('user.password.new')"
            ><el-input
              v-model="passwordForm.newPassword"
              data-testid="account-password-new"
              type="password"
              show-password
              autocomplete="new-password"
          /></el-form-item>
          <el-form-item :label="t('user.password.confirm')"
            ><el-input
              v-model="passwordForm.confirmPassword"
              data-testid="account-password-confirm"
              type="password"
              show-password
              autocomplete="new-password"
          /></el-form-item>
          <div v-if="canUpdatePassword" class="account-profile__actions">
            <el-button
              data-testid="account-password-submit"
              type="primary"
              :loading="changingPassword"
              @click="submitPassword"
              >{{
                t(setPasswordMode ? 'user.password.setSubmit' : 'user.password.submit')
              }}</el-button
            >
          </div>
        </el-form>
      </section>
    </div>
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

.account-profile__actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 6px;
}

@media (max-width: 560px) {
  .account-profile__card {
    padding: 20px 18px;
  }
}
</style>
