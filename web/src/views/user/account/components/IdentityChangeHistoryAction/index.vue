<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { UserListItem } from '@/api/user/account'
import EmailChangeLogDialog from '@/views/user/account/components/EmailChangeLogDialog/index.vue'
import { useIdentityChangeLogs } from '@/views/user/account/useIdentityChangeLogs'

defineOptions({ name: 'IdentityChangeHistoryAction' })
const props = defineProps<{ user: UserListItem; enabled: boolean }>()
const { t } = useI18n()
const errorMessage = (error: unknown, fallback: string): string =>
  error instanceof Error && error.message !== '' ? error.message : t(fallback)
const logs = useIdentityChangeLogs(errorMessage)
function open(): void {
  if (props.enabled) void logs.open(props.user)
}
</script>

<template>
  <el-button v-if="props.enabled" text type="primary" @click="open">{{
    t('user.emailChangeView')
  }}</el-button>
  <EmailChangeLogDialog
    v-if="logs.visible.value"
    v-model="logs.visible.value"
    :username="logs.target.value?.username ?? ''"
    :rows="logs.emailRows.value"
    :loading="logs.emailLoading.value"
    :error="logs.emailError.value"
    :pagination="logs.emailPagination.value"
    :phone-rows="logs.phoneRows.value"
    :phone-loading="logs.phoneLoading.value"
    :phone-error="logs.phoneError.value"
    :phone-pagination="logs.phonePagination.value"
    @refresh="logs.refreshEmail"
    @refresh:phone="logs.refreshPhone"
    @update:pagination="logs.updateEmailPagination"
    @update:phone-pagination="logs.updatePhonePagination"
  />
</template>
