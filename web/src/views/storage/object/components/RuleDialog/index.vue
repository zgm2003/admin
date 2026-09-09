<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules } from 'element-plus'

import { AppDialog } from '@/components/AppDialog'
import type { ConfigSummary, PlatformOption } from '@/api/storage/uploadRule'
import type { RuleForm } from '@/views/storage/object/components/types'

const props = defineProps<{
  editing: boolean
  rules: FormRules<RuleForm>
  platforms: PlatformOption[]
  configs: ConfigSummary[]
  fileSizeMb: number
  extensions: string[]
  mimeTypes: string[]
  allExtensionsSelected: boolean
  someExtensionsSelected: boolean
  allMimeTypesSelected: boolean
  someMimeTypesSelected: boolean
  extensionsError: string
  toggleAllExtensions: (checked: boolean | string | number) => void
  toggleAllMimeTypes: (checked: boolean | string | number) => void
}>()
const visible = defineModel<boolean>({ required: true })
const form = defineModel<RuleForm>('form', { required: true })
const emit = defineEmits<{
  'update:fileSizeMb': [value: number]
  save: []
}>()
const { t } = useI18n()
const formRef = ref<FormInstance>()
const platformOptions = computed(() =>
  props.platforms.map((item) => ({ label: item.name, value: item.id })),
)
const configOptions = computed(() =>
  props.configs.map((item) => ({ label: item.name, value: item.id })),
)
const extensionOptions = computed(() =>
  props.extensions.map((item) => ({ label: item, value: item })),
)
const mimeTypeOptions = computed(() =>
  props.mimeTypes.map((item) => ({ label: item, value: item })),
)
defineExpose({ validate: () => formRef.value?.validate() })
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="props.editing ? t('storage.editRule') : t('storage.addRule')"
    width="min(760px, 94vw)"
    height="min(72vh, 720px)"
    :append-to-body="false"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="props.rules"
      label-position="top"
      data-testid="storage-rule-form"
    >
      <el-row :gutter="16">
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.platform')" prop="platformId"
            ><el-select-v2
              v-model="form.platformId"
              :options="platformOptions"
              data-testid="storage-rule-platform"
              class="storage-rule-select"
              :disabled="props.editing"
              :placeholder="t('storage.rulePlatformPlaceholder')" /></el-form-item
        ></el-col>
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.config')" prop="cosConfigId"
            ><el-select-v2
              v-model="form.cosConfigId"
              :options="configOptions"
              data-testid="storage-rule-config"
              class="storage-rule-select"
              :placeholder="t('storage.ruleConfigPlaceholder')" /></el-form-item
        ></el-col>
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.code')" prop="codes"
            ><el-input-tag
              v-model="form.codes"
              data-testid="storage-rule-codes"
              :disabled="props.editing"
              :placeholder="t('storage.ruleCodePlaceholder')"
            />
            <div class="form-help">{{ t('storage.ruleCodeHelp') }}</div></el-form-item
          ></el-col
        >
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.name')" prop="name"
            ><el-input
              v-model="form.name"
              data-testid="storage-rule-name"
              :placeholder="t('storage.ruleNamePlaceholder')" /></el-form-item
        ></el-col>
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.maxFileSizeBytes')" prop="maxFileSizeBytes"
            ><el-input-number
              :model-value="props.fileSizeMb"
              data-testid="storage-rule-max-file-size-mb"
              :min="0.01"
              :max="1024"
              :step="1"
              :precision="2"
              controls-position="right"
              @update:model-value="emit('update:fileSizeMb', $event ?? 0)" /></el-form-item
        ></el-col>
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.accessMode')" prop="accessMode"
            ><el-radio-group v-model="form.accessMode"
              ><el-radio value="private">{{ t('storage.private') }}</el-radio
              ><el-radio value="public">{{ t('storage.public') }}</el-radio></el-radio-group
            ></el-form-item
          ></el-col
        >
        <el-col v-if="form.accessMode === 'public'" :xs="24"
          ><el-alert
            data-testid="storage-public-warning"
            :title="t('storage.publicWarning')"
            type="warning"
            show-icon
            :closable="false"
        /></el-col>
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.extensions')" prop="allowedExtensions"
            ><el-select-v2
              v-model="form.allowedExtensions"
              :options="extensionOptions"
              data-testid="storage-rule-extensions"
              class="storage-rule-select"
              multiple
              filterable
              allow-create
              default-first-option
              :reserve-keyword="false"
              :placeholder="t('storage.extensionsPlaceholder')"
              ><template #header
                ><el-checkbox
                  data-testid="storage-rule-extensions-select-all"
                  :model-value="props.allExtensionsSelected"
                  :indeterminate="props.someExtensionsSelected"
                  @change="props.toggleAllExtensions"
                  >{{ t('storage.selectAll') }}</el-checkbox
                ></template
              >
            </el-select-v2>
            <div v-if="props.extensionsError" class="el-form-item__error">
              {{ props.extensionsError }}
            </div>
            <div class="form-help">{{ t('storage.extensionsHelp') }}</div></el-form-item
          ></el-col
        >
        <el-col :xs="24" :sm="12"
          ><el-form-item :label="t('storage.mimeTypes')"
            ><el-select-v2
              v-model="form.allowedMimeTypes"
              :options="mimeTypeOptions"
              data-testid="storage-rule-mime-types"
              class="storage-rule-select"
              multiple
              filterable
              allow-create
              default-first-option
              :reserve-keyword="false"
              :placeholder="t('storage.mimeTypesPlaceholder')"
              ><template #header
                ><el-checkbox
                  data-testid="storage-rule-mime-types-select-all"
                  :model-value="props.allMimeTypesSelected"
                  :indeterminate="props.someMimeTypesSelected"
                  @change="props.toggleAllMimeTypes"
                  >{{ t('storage.selectAll') }}</el-checkbox
                ></template
              >
            </el-select-v2>
            <div class="form-help">{{ t('storage.mimeTypesHelp') }}</div></el-form-item
          ></el-col
        >
        <el-col :xs="24"
          ><el-form-item :label="t('storage.remark')"
            ><el-input
              v-model="form.remark"
              type="textarea"
              :rows="2"
              :placeholder="t('storage.remarkPlaceholder')" /></el-form-item
        ></el-col>
      </el-row>
    </el-form>
    <template #footer
      ><el-button @click="visible = false">{{ t('storage.cancel') }}</el-button
      ><el-button type="primary" @click="emit('save')">{{ t('storage.save') }}</el-button></template
    >
  </AppDialog>
</template>

<style scoped>
.form-help {
  margin-top: 6px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.storage-rule-select {
  width: 100%;
}
</style>
