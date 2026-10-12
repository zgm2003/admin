<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import FileSelectDialog from '@/components/FileSelectDialog/index.vue'
import { usePermissionStore } from '@/store/permission'
import type { StorageFile, CompletedFile } from '@/api/storage/file'
import type { UpdateAccountProfileInput } from '@/api/user/profile'

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
const access = usePermissionStore()
const canSelectFile = computed(
  () =>
    props.canUpdateProfile &&
    access.hasPermission('storage:file:list') &&
    access.hasPermission('storage:file:detail'),
)
const canUploadAvatar = computed(
  () =>
    props.canUpdateProfile &&
    access.hasPermission('storage:object:upload') &&
    access.hasPermission('storage:file:detail'),
)
const selectorVisible = ref(false)
const avatarKey = ref(props.profileForm.avatar)
const selectedName = ref('')

const form = reactive<UpdateAccountProfileInput>({ ...props.profileForm })
watch(
  () => props.profileForm,
  (value) => {
    Object.assign(form, value)
    if (value.avatarFileId === undefined) {
      avatarKey.value = value.avatar
      selectedName.value = ''
    }
    if (value.avatarFileId === undefined) {
      delete form.avatarFileId
      delete form.avatarRequestKey
    }
  },
  { deep: true },
)
function updateUsername(value: string): void {
  form.username = value
  emit('update:profileForm', { ...form })
}

function updateAvatar(value: string | string[]): void {
  if (typeof value !== 'string') return
  avatarKey.value = value
  if (value === '') {
    form.avatar = ''
    delete form.avatarFileId
    delete form.avatarRequestKey
    selectedName.value = ''
    emit('update:profileForm', { ...form })
  }
}
function selectFile(file: StorageFile): void {
  form.avatar = ''
  form.avatarFileId = file.id
  form.avatarRequestKey = crypto.randomUUID()
  avatarKey.value = file.objectKey
  selectedName.value = file.fileName
  emit('update:profileForm', { ...form })
}
function avatarUploaded(items: CompletedFile[]): void {
  const file = items[0]
  if (!file) return
  form.avatar = ''
  form.avatarFileId = file.fileId
  form.avatarRequestKey = crypto.randomUUID()
  avatarKey.value = file.objectKey
  selectedName.value = file.objectKey.split('/').at(-1) ?? ''
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
                  t(currentEmail === '' ? 'user.identity.bindEmail' : 'user.identity.changeEmail')
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
                  t(currentPhone === null ? 'user.identity.bindPhone' : 'user.identity.changePhone')
                }}
              </el-button>
            </div>
          </el-form-item>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-form-item :label="t('user.profile.avatar')">
            <UpMedia
              :model-value="avatarKey"
              rule-code="avatar"
              variant="avatar"
              width="178px"
              :disabled="!canUpdateProfile"
              :upload-disabled="!canUploadAvatar"
              @uploaded="avatarUploaded"
              @update:model-value="updateAvatar"
              @preview-change="emit('avatarPreviewChange', $event)"
            />
            <el-button
              v-if="canSelectFile"
              type="primary"
              data-testid="profile-choose-file"
              @click="selectorVisible = true"
              >{{ t('storage.file.chooseAvatar') }}</el-button
            >
            <span v-if="selectedName">{{
              t('storage.file.selectedAvatar', { name: selectedName })
            }}</span>
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
        <el-button data-testid="account-profile-save" type="primary" @click="emit('save')">{{
          t('user.profile.save')
        }}</el-button>
      </div>
    </el-form>
    <FileSelectDialog v-model="selectorVisible" purpose="avatar" @select="selectFile" />
  </section>
</template>

<style scoped lang="scss">
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
