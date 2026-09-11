<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { UpdateAccountProfileInput } from '@/api/user/profile'
import { UpMedia } from '@/components/UpMedia'

interface Props {
  profileForm: UpdateAccountProfileInput
  currentEmail: string
  currentPhone: string | null
  genderOptions: Array<{ label: string; value: UpdateAccountProfileInput['gender'] }>
  genderOptionsLoading: boolean
  genderOptionsError: string
  loading: boolean
  canUpdateProfile: boolean
  canUpdateEmail: boolean
  canUpdatePhone: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:profileForm': [value: UpdateAccountProfileInput]
  save: []
  emailAction: []
  phoneAction: []
  avatarPreviewChange: [url: string]
}>()
const { t } = useI18n()

const form = reactive<UpdateAccountProfileInput>({ ...props.profileForm })
watch(
  () => props.profileForm,
  (value) => Object.assign(form, value),
  { deep: true },
)
function updateUsername(value: string): void {
  form.username = value
  emit('update:profileForm', { ...form })
}

function updateAvatar(value: string | string[]): void {
  if (typeof value !== 'string') return
  form.avatar = value
  emit('update:profileForm', { ...form })
}

function updateBirthday(value: string | null): void {
  form.birthday = value
  emit('update:profileForm', { ...form })
}

function updateGender(value: UpdateAccountProfileInput['gender']): void {
  form.gender = value
  emit('update:profileForm', { ...form })
}
</script>

<template>
  <section v-loading="loading" class="account-profile__card">
    <header class="account-profile__card-head">
      <h2 class="account-profile__card-title">{{ t('user.profile.basicTitle') }}</h2>
    </header>
    <el-form label-position="top" @submit.prevent="emit('save')">
      <el-row :gutter="16">
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('user.profile.username')">
            <el-input
              :model-value="form.username"
              autocomplete="username"
              @update:model-value="updateUsername"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('user.profile.email')">
            <div class="account-profile__identity-row">
              <el-input :model-value="currentEmail" disabled />
              <el-button
                v-if="canUpdateEmail"
                data-testid="profile-email-action"
                @click="emit('emailAction')"
              >
                {{
                  t(
                    currentEmail === ''
                      ? 'user.identity.bindEmail'
                      : 'user.identity.changeEmail',
                  )
                }}
              </el-button>
            </div>
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('user.profile.phone')">
            <div class="account-profile__identity-row">
              <el-input :model-value="currentPhone ?? ''" disabled />
              <el-button
                v-if="canUpdatePhone"
                data-testid="profile-phone-action"
                @click="emit('phoneAction')"
              >
                {{
                  t(
                    currentPhone === null
                      ? 'user.identity.bindPhone'
                      : 'user.identity.changePhone',
                  )
                }}
              </el-button>
            </div>
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('user.profile.avatar')">
            <UpMedia
              :model-value="form.avatar"
              rule-code="avatar"
              variant="avatar"
              width="178px"
              :disabled="!canUpdateProfile"
              @update:model-value="updateAvatar"
              @preview-change="emit('avatarPreviewChange', $event)"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('user.profile.birthday')">
            <el-date-picker
              :model-value="form.birthday"
              type="date"
              value-format="YYYY-MM-DD"
              class="account-profile__full"
              @update:model-value="updateBirthday"
            />
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('user.profile.gender')">
            <el-select-v2
              :model-value="form.gender"
              :options="genderOptions"
              :loading="genderOptionsLoading"
              :disabled="genderOptionsLoading || genderOptionsError !== ''"
              data-testid="account-profile-gender"
              class="account-profile__full"
              @update:model-value="updateGender"
            />
            <div v-if="genderOptionsError" class="el-form-item__error">
              {{ genderOptionsError }}
            </div>
          </el-form-item>
        </el-col>
      </el-row>
      <div v-if="canUpdateProfile" class="account-profile__actions">
        <el-button
          data-testid="account-profile-save"
          type="primary"
          @click="emit('save')"
          >{{ t('user.profile.save') }}</el-button
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

.account-profile__identity-row {
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

  .account-profile__identity-row {
    grid-template-columns: 1fr;
  }
}
</style>
