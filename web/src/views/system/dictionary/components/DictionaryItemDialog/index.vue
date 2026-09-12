<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { DictionaryItem } from '@/api/system/dictionary'
import { AppDialog } from '@/components/AppDialog'

defineOptions({ name: 'DictionaryItemDialog' })

type DictionaryItemForm = {
  value: string
  labelZh: string
  labelEn: string
  sort: number
}

defineProps<{
  editing: DictionaryItem | null
  submitting: boolean
}>()

const visible = defineModel<boolean>({ required: true })
const form = defineModel<DictionaryItemForm>('form', { required: true })
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="editing === null ? t('dictionary.createItem') : t('dictionary.editItem')"
    width="520px"
  >
    <el-form label-position="top" @submit.prevent="emit('save')">
      <el-form-item :label="t('dictionary.value')">
        <el-input
          v-model="form.value"
          data-testid="dictionary-item-form-value"
          :maxlength="128"
          :placeholder="t('dictionary.valuePlaceholder')"
          :disabled="editing !== null"
        />
      </el-form-item>
      <el-form-item :label="t('dictionary.labelZh')">
        <el-input
          v-model="form.labelZh"
          data-testid="dictionary-item-form-label-zh"
          :maxlength="256"
          :placeholder="t('dictionary.labelZhPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('dictionary.labelEn')">
        <el-input
          v-model="form.labelEn"
          data-testid="dictionary-item-form-label-en"
          :maxlength="256"
          :placeholder="t('dictionary.labelEnPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('dictionary.sort')">
        <el-input-number
          v-model="form.sort"
          data-testid="dictionary-item-form-sort"
          :min="0"
          :placeholder="t('dictionary.sortPlaceholder')"
        />
      </el-form-item>
      <el-button
        data-testid="dictionary-item-save"
        type="primary"
        :loading="submitting"
        @click="emit('save')"
        >{{ t('dictionary.save') }}</el-button
      >
    </el-form>
  </AppDialog>
</template>
