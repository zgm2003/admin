<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { Dictionary } from '@/api/system/dictionary'

defineOptions({ name: 'DictionaryFormDialog' })

type DictionaryForm = {
  code: string
  nameZh: string
  nameEn: string
  description: string
}

defineProps<{
  editing: Dictionary | null
  submitting: boolean
}>()

const visible = defineModel<boolean>({ required: true })
const form = defineModel<DictionaryForm>('form', { required: true })
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="editing === null ? t('dictionary.create') : t('dictionary.edit')"
    width="520px"
  >
    <el-form label-position="top" @submit.prevent="emit('save')">
      <el-form-item :label="t('dictionary.code')">
        <el-input
          v-model="form.code"
          data-testid="dictionary-form-code"
          :maxlength="128"
          :placeholder="t('dictionary.codePlaceholder')"
          :disabled="editing !== null"
        />
      </el-form-item>
      <el-form-item :label="t('dictionary.nameZh')">
        <el-input
          v-model="form.nameZh"
          data-testid="dictionary-form-name-zh"
          :maxlength="128"
          :placeholder="t('dictionary.nameZhPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('dictionary.nameEn')">
        <el-input
          v-model="form.nameEn"
          data-testid="dictionary-form-name-en"
          :maxlength="128"
          :placeholder="t('dictionary.nameEnPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('dictionary.description')">
        <el-input
          v-model="form.description"
          data-testid="dictionary-form-description"
          :maxlength="512"
          type="textarea"
          :placeholder="t('dictionary.descriptionPlaceholder')"
        />
      </el-form-item>
      <el-button
        data-testid="dictionary-save"
        type="primary"
        :loading="submitting"
        @click="emit('save')"
        >{{ t('dictionary.save') }}</el-button
      >
    </el-form>
  </AppDialog>
</template>
