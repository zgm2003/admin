<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AppDialog } from '@/components/AppDialog'
import type { UserListItem } from '@/api/user/account'
import type { UserFormState } from '@/views/account/users/components/types'

const props = defineProps<{
  editingUser: UserListItem | null
  editError: string
  editSaving: boolean
  usernameValid: boolean
  phoneValid: boolean
}>()
const visible = defineModel<boolean>({ required: true })
const form = defineModel<UserFormState>('form', { required: true })
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
</script>

<template>
  <AppDialog
    v-model="visible"
    class="user-edit-dialog"
    :title="t('user.editTitle')"
    width="min(520px, 94vw)"
    append-to-body
  >
    <el-alert v-if="props.editError" :title="props.editError" type="error" /><el-form
      label-position="top"
      ><el-form-item :label="t('user.email')"
        ><el-input :model-value="props.editingUser?.email ?? ''" disabled /></el-form-item
      ><el-form-item
        :label="t('user.username')"
        :error="form.username !== '' && !props.usernameValid ? t('user.invalidUsername') : ''"
        ><el-input v-model="form.username" maxlength="64" /></el-form-item
      ><el-form-item
        :label="t('user.phone')"
        :error="form.phone !== '' && !props.phoneValid ? t('user.invalidPhone') : ''"
        ><el-input v-model="form.phone" data-testid="user-phone" /></el-form-item
    ></el-form>
    <template #footer
      ><el-button @click="visible = false">{{ t('user.cancel') }}</el-button
      ><el-button
        type="primary"
        :loading="props.editSaving"
        :disabled="!props.usernameValid || !props.phoneValid"
        @click="emit('save')"
        >{{ t('user.save') }}</el-button
      ></template
    >
  </AppDialog>
</template>
