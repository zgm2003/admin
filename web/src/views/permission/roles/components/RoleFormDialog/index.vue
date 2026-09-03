<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { AppDialog } from '@/components/AppDialog'
import type { RoleFormState } from '@/views/permission/roles/components/types'

const props = defineProps<{
  editing: boolean
  mutationError: string
  submitting: boolean
  formValid: boolean
}>()
const visible = defineModel<boolean>({ required: true })
const form = defineModel<RoleFormState>('form', { required: true })
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="t(!props.editing ? 'role.form.createTitle' : 'role.form.editTitle')"
    width="520px"
    append-to-body
  >
    <el-alert v-if="props.mutationError" :title="props.mutationError" type="error" />
    <el-form label-position="top">
      <el-form-item :label="t('role.form.code')">
        <el-input v-model="form.code" :disabled="props.editing" />
      </el-form-item>
      <el-form-item :label="t('role.form.name')">
        <el-input v-model="form.name" maxlength="64" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">
        {{ t('role.form.cancel') }}
      </el-button>
      <el-button
        type="primary"
        :loading="props.submitting"
        :disabled="!props.formValid"
        @click="emit('save')"
      >
        {{ t('role.form.submit') }}
      </el-button>
    </template>
  </AppDialog>
</template>
