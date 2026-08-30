<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'

import { changePassword, getAccountProfile, updateAccountProfile } from '../../../api/user/profile'
import type { AccountProfile, ChangePasswordInput, UpdateAccountProfileInput } from '../../../api/user/profile'
import { useAccessStore } from '../../../store/access'
import { useAuthStore } from '../../../store/auth'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const access = useAccessStore()
const canUpdateProfile = computed(() => access.hasPermission('account:profile:update'))
const canUpdatePassword = computed(() => access.hasPermission('account:password:update'))
const loading = ref(false)
const savingProfile = ref(false)
const changingPassword = ref(false)
const loadError = ref('')
const profileForm = reactive<UpdateAccountProfileInput>({ username: '', phone: null, birthday: null, gender: 0 })
const passwordForm = reactive<ChangePasswordInput>({ currentPassword: '', newPassword: '', confirmPassword: '' })

function applyProfile(profile: AccountProfile): void {
  profileForm.username = profile.username
  profileForm.phone = profile.phone
  profileForm.birthday = profile.birthday
  profileForm.gender = profile.gender
}

async function loadProfile(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    applyProfile(await getAccountProfile())
  } catch (error: unknown) {
    loadError.value = error instanceof Error && error.message !== '' ? error.message : t('account.profile.loadFailed')
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
    auth.updateProfile(updated.userId, updated.username, updated.phone)
    ElMessage.success(t('account.profile.saved'))
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
    await changePassword({ ...passwordForm })
    await ElMessageBox.alert(t('account.password.successMessage'), t('account.password.successTitle'), { type: 'success' })
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

    <el-row :gutter="16" class="account-profile__grid">
      <el-col :xs="24" :lg="14">
        <el-card shadow="never" v-loading="loading">
          <template #header><div class="account-profile__card-title">{{ t('account.profile.basicTitle') }}</div></template>
          <el-form label-position="top" @submit.prevent="saveProfile">
            <el-row :gutter="16">
              <el-col :xs="24" :sm="12">
                <el-form-item :label="t('account.profile.username')">
                  <el-input v-model="profileForm.username" autocomplete="username" />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item :label="t('account.profile.email')">
                  <el-input :model-value="auth.user?.email ?? ''" disabled />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item :label="t('account.profile.phone')">
                  <el-input v-model="profileForm.phone" clearable />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item :label="t('account.profile.birthday')">
                  <el-date-picker v-model="profileForm.birthday" type="date" value-format="YYYY-MM-DD" class="account-profile__full" />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :sm="12">
                <el-form-item :label="t('account.profile.gender')">
                  <el-select v-model="profileForm.gender" class="account-profile__full">
                    <el-option :label="t('account.profile.genderUnknown')" :value="0" />
                    <el-option :label="t('account.profile.genderMale')" :value="1" />
                    <el-option :label="t('account.profile.genderFemale')" :value="2" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>
            <div v-if="canUpdateProfile" class="account-profile__actions"><el-button data-testid="account-profile-save" type="primary" :loading="savingProfile" @click="saveProfile">{{ t('account.profile.save') }}</el-button></div>
          </el-form>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="10">
        <el-card shadow="never">
          <template #header><div class="account-profile__card-title">{{ t('account.password.title') }}</div></template>
          <el-form label-position="top" @submit.prevent="submitPassword">
            <el-form-item :label="t('account.password.current')"><el-input v-model="passwordForm.currentPassword" type="password" show-password autocomplete="current-password" /></el-form-item>
            <el-form-item :label="t('account.password.new')"><el-input v-model="passwordForm.newPassword" type="password" show-password autocomplete="new-password" /></el-form-item>
            <el-form-item :label="t('account.password.confirm')"><el-input v-model="passwordForm.confirmPassword" type="password" show-password autocomplete="new-password" /></el-form-item>
            <div v-if="canUpdatePassword" class="account-profile__actions"><el-button data-testid="account-password-submit" type="primary" :loading="changingPassword" @click="submitPassword">{{ t('account.password.submit') }}</el-button></div>
          </el-form>
        </el-card>
      </el-col>
    </el-row>
  </section>
</template>

<style scoped lang="scss">
.account-profile { display: grid; gap: 16px; }
.account-profile__grid { align-items: start; }
.account-profile__card-title { color: var(--el-text-color-primary); font-weight: 650; }
.account-profile__full { width: 100%; }
.account-profile__actions { display: flex; justify-content: flex-end; margin-top: 6px; }
</style>
