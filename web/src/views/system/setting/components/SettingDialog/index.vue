<script setup lang="ts">
import { ref } from 'vue'
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
const valueError = ref('')

function validateValue(): boolean {
  valueError.value = ''
  if (form.value.value.trim() === '') {
    valueError.value = t('setting.valueRequired')
    return false
  }
  if (form.value.valueType === 4) {
    try {
      JSON.parse(form.value.value)
    } catch {
      valueError.value = t('setting.jsonInvalid')
      return false
    }
  }
  return true
}

function formatJSON(): void {
  valueError.value = ''
  try {
    form.value.value = JSON.stringify(JSON.parse(form.value.value), null, 2)
  } catch {
    valueError.value = t('setting.jsonInvalid')
  }
}

function save(): void {
  if (validateValue()) emit('save')
}
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="editing === null ? t('setting.create') : t('setting.edit')"
    width="min(560px, 94vw)"
  >
    <el-form label-position="top" @submit.prevent="save">
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
        <el-switch
          v-if="form.valueType === 3"
          v-model="form.value"
          data-testid="setting-form-value"
          active-value="true"
          inactive-value="false"
          :active-text="t('setting.trueValue')"
          :inactive-text="t('setting.falseValue')"
        />
        <el-input
          v-else
          v-model="form.value"
          data-testid="setting-form-value"
          :type="form.valueType === 4 ? 'textarea' : form.valueType === 2 ? 'number' : 'text'"
          :rows="form.valueType === 4 ? 8 : undefined"
          :placeholder="t('setting.valuePlaceholder')"
        />
        <div v-if="valueError" class="el-form-item__error setting-value-error">
          {{ valueError }}
        </div>
        <el-button
          v-if="form.valueType === 4"
          data-testid="setting-json-format"
          class="setting-json-format"
          text
          type="primary"
          @click="formatJSON"
          >{{ t('setting.formatJson') }}</el-button
        >
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
      <el-button data-testid="setting-save" type="primary" :loading="submitting" @click="save">
        {{ t('setting.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>

<style scoped>
.setting-json-format {
  margin-top: 6px;
  margin-left: auto;
}

.setting-value-error {
  position: static;
  width: 100%;
}
</style>
