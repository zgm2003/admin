<!-- 组件：角色新建与编辑弹窗 -->
<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { RoleFormState } from '@/views/permission/role/components/types'

const props = defineProps<{
  editing: boolean
  submitting: boolean
  formValid: boolean
  nameMaxLength: number | undefined
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
    
    <el-form label-position="top">
      <el-form-item :label="t('role.form.code')">
        <el-input v-model="form.code" :disabled="props.editing" />
      </el-form-item>
      <el-form-item :label="t('role.form.name')">
        <el-input v-model="form.name" :maxlength="props.nameMaxLength" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">
        {{ t('common.cancel') }}
      </el-button>
      <el-button
        type="primary"
        :loading="props.submitting"
        :disabled="!props.formValid"
        @click="emit('save')"
      >
        {{ t('common.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>
