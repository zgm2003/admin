<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { SystemSetting, SettingValueType } from '@/api/system/setting'

defineOptions({ name: 'SettingDialog' })

type SettingForm = {
  key: string
  value: string
  valueType: SettingValueType
  description: string
}

type SettingTypeOption = {
  label: string
  value: SettingValueType
}

defineProps<{
  editing: SystemSetting | null
  submitting: boolean
  valueTypeOptions: SettingTypeOption[]
}>()

const visible = defineModel<boolean>({ required: true })
const form = defineModel<SettingForm>('form', { required: true })
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="editing === null ? t('setting.create') : t('setting.edit')"
    width="min(560px, 94vw)"
  >
    <el-form label-position="top" @submit.prevent="emit('save')">
      <el-form-item :label="t('setting.key')">
        <el-input
          v-model="form.key"
          data-testid="setting-form-key"
          :maxlength="128"
          :disabled="editing !== null"
          :placeholder="t('setting.keyPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('setting.type')">
        <el-select-v2
          v-model="form.valueType"
          data-testid="setting-form-type"
          :options="valueTypeOptions"
          style="width: 100%"
        />
      </el-form-item>
      <el-form-item :label="t('setting.value')">
        <el-input
          v-model="form.value"
          data-testid="setting-form-value"
          type="textarea"
          :rows="4"
          :placeholder="t('setting.valuePlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('setting.descriptionField')">
        <el-input
          v-model="form.description"
          data-testid="setting-form-description"
          :maxlength="512"
          :placeholder="t('setting.descriptionPlaceholder')"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('setting.cancel') }}</el-button>
      <el-button
        data-testid="setting-save"
        type="primary"
        :loading="submitting"
        @click="emit('save')"
      >
        {{ t('setting.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>
