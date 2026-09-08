<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AppDialog } from '@/components/AppDialog'
import { YesNo } from '@/enums/yes-no'
import type { UserRolesResponse, UserRoleSummary } from '@/api/user/account'

const props = defineProps<{
  roleData: UserRolesResponse | null
  roleLoading: boolean
  roleError: string
  roleSaving: boolean
  hasEnabledSelection: boolean
  roleToggleDisabled: (role: UserRoleSummary) => boolean
}>()
const visible = defineModel<boolean>({ required: true })
const selectedRoleIDs = defineModel<number[]>('selectedRoleIDs', { required: true })
const emit = defineEmits<{ 'select-all': []; clear: []; save: [] }>()
const { t } = useI18n()
</script>

<template>
  <AppDialog
    v-model="visible"
    class="user-role-dialog"
    :title="t('user.assignRolesTitle')"
    width="min(680px, 94vw)"
    height="min(62vh, 620px)"
    append-to-body
  >
    <div class="role-dialog-scroll">
      <div v-if="props.roleLoading">{{ t('user.roleLoadFailed') }}</div>
      <el-alert v-if="props.roleError" :title="props.roleError" type="error" show-icon /><template
        v-if="props.roleData"
      >
        <el-space class="role-dialog-toolbar" wrap :size="8">
          <el-button @click="emit('select-all')">{{ t('user.selectAll') }}</el-button
          ><el-button @click="emit('clear')">{{ t('user.clear') }}</el-button>
        </el-space>
        <el-checkbox-group v-model="selectedRoleIDs" class="role-checks"
          ><el-checkbox
            v-for="role in props.roleData.roles"
            :key="role.id"
            :value="role.id"
            :disabled="props.roleToggleDisabled(role)"
            ><span>{{ role.name }} ({{ role.code }})</span
            ><el-tag v-if="role.isEnabled === YesNo.No" type="info" size="small">{{
              t('user.roleDisabled')
            }}</el-tag></el-checkbox
          ></el-checkbox-group
        ><el-alert
          v-if="!props.hasEnabledSelection"
          :title="t('user.enabledRoleRequired')"
          type="warning"
        />
      </template>
    </div>
    <template #footer
      ><el-button @click="visible = false">{{ t('user.cancel') }}</el-button
      ><el-button
        type="primary"
        :loading="props.roleSaving"
        :disabled="props.roleData === null || !props.hasEnabledSelection"
        @click="emit('save')"
        >{{ t('user.save') }}</el-button
      ></template
    >
  </AppDialog>
</template>
