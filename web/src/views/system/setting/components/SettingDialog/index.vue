<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import UpMedia from '@/components/UpMedia/index.vue'
import { isStorageObjectKey } from '@/utils/storageObjectKey'
import { useI18n } from 'vue-i18n'

import type { SystemSetting, SettingValueType } from '@/api/system/setting'
import type { SettingPresentation, SettingTypeOption } from '@/api/system/settingOptions'

defineOptions({ name: 'SettingDialog' })

type SettingForm = {
  key: string
  value: string
  valueType: SettingValueType
  description: string
}

const props = defineProps<{
  defaultPresentation: SettingPresentation
  editing: SystemSetting | null
  submitting: boolean
  canSave: boolean
  canUpload: boolean
  error: string
  valueTypeOptions: SettingTypeOption[]
}>()

const visible = defineModel<boolean>({ required: true })
const form = defineModel<SettingForm>('form', { required: true })
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
const valueError = ref('')
const uploading = ref(false)
const presentation = computed(() => props.editing?.presentation ?? props.defaultPresentation)
const editor = computed(
  () =>
    props.valueTypeOptions.find((option) => option.value === form.value.valueType)?.editor ??
    presentation.value.editor,
)
const mediaAccept = computed(() => presentation.value.mediaAccept)
// File preview classification is browser presentation, not upload acceptance.
const mediaVariant = computed(() =>
  presentation.value.mediaVariant === 'avatar'
    ? 'avatar'
    : /\.(?:png|jpg|jpeg|gif|webp)$/u.test(form.value.value)
      ? 'default'
      : presentation.value.mediaVariant,
)
const allowEmpty = computed(() =>
  props.editing?.valueType === form.value.valueType
    ? presentation.value.allowEmpty
    : (props.valueTypeOptions.find((option) => option.value === form.value.valueType)?.allowEmpty ??
      presentation.value.allowEmpty),
)
watch(visible, () => {
  valueError.value = ''
  uploading.value = false
})
watch(
  () => form.value.valueType,
  (type) => {
    valueError.value = ''
    uploading.value = false
    if (
      props.valueTypeOptions.find((option) => option.value === type)?.editor === 'media' &&
      form.value.value !== '' &&
      !isStorageObjectKey(form.value.value)
    )
      form.value.value = ''
  },
)
function setMedia(value: string | string[]): void {
  if (typeof value !== 'string' || !props.canSave || props.submitting) return
  form.value.value = value
  valueError.value = ''
}

function validateValue(): boolean {
  valueError.value = ''
  const rules = presentation.value
  if (form.value.value.trim() === '') {
    if (allowEmpty.value) return true
    valueError.value = t('setting.valueRequired')
    return false
  }
  if (editor.value === 'media') {
    if (form.value.value !== '' && !isStorageObjectKey(form.value.value)) {
      valueError.value = t('setting.mediaInvalid')
      return false
    }
    return true
  }
  if (rules.maxLength !== null && [...form.value.value.trim()].length > rules.maxLength) {
    valueError.value = t('setting.brandTitleInvalid')
    return false
  }
  if (editor.value === 'json') {
    try {
      JSON.parse(form.value.value)
    } catch {
      valueError.value = t('setting.jsonInvalid')
      return false
    }
  }
  if (rules.minimum !== null && rules.maximum !== null) {
    const numeric = Number(form.value.value)
    if (!Number.isInteger(numeric) || numeric < rules.minimum || numeric > rules.maximum) {
      valueError.value = t('setting.retentionRange', {
        minimum: rules.minimum,
        maximum: rules.maximum,
      })
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
  if (!props.canSave || props.submitting || uploading.value) return
  if (validateValue()) emit('save')
}
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="editing === null ? t('setting.create') : t('setting.edit')"
    width="min(560px, 94vw)"
    :show-close="!submitting"
    :close-on-press-escape="!submitting"
    :close-on-click-modal="!submitting"
  >
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      show-icon
      class="setting-submit-error"
    />
    <el-form label-position="top" @submit.prevent="save">
      <el-form-item :label="t('setting.key')">
        <el-input
          v-model="form.key"
          data-testid="setting-form-key"
          :maxlength="128"
          :disabled="editing !== null || uploading || submitting || !canSave"
          :placeholder="t('setting.keyPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('setting.type')">
        <el-select-v2
          v-model="form.valueType"
          data-testid="setting-form-type"
          :options="valueTypeOptions"
          :disabled="uploading || submitting || !canSave || presentation.valueTypeLocked"
          style="width: 100%"
        />
      </el-form-item>
      <el-form-item :label="t('setting.value')">
        <template v-if="editor === 'media'">
          <UpMedia
            v-if="visible"
            :key="form.key"
            :model-value="form.value"
            :rule-code="presentation.mediaRuleCode"
            :multiple="false"
            :variant="mediaVariant"
            :accept="mediaAccept"
            :file-label="''"
            :disabled="submitting || !canSave"
            :upload-disabled="!canUpload"
            @update:model-value="setMedia"
            @uploading-change="(value) => (uploading = value)"
          />
          <p class="setting-media-hint">{{ t('setting.mediaHint') }}</p>
        </template>
        <el-switch
          v-else-if="editor === 'boolean'"
          v-model="form.value"
          data-testid="setting-form-value"
          :disabled="submitting || !canSave"
          active-value="true"
          inactive-value="false"
          :active-text="t('setting.trueValue')"
          :inactive-text="t('setting.falseValue')"
        />
        <el-input
          v-else
          v-model="form.value"
          data-testid="setting-form-value"
          :disabled="submitting || !canSave"
          :type="editor === 'json' ? 'textarea' : editor === 'number' ? 'number' : 'text'"
          :min="presentation.minimum ?? undefined"
          :max="presentation.maximum ?? undefined"
          :step="presentation.minimum !== null ? 1 : undefined"
          :rows="editor === 'json' ? 8 : undefined"
          :maxlength="presentation.maxLength ?? undefined"
          :placeholder="t('setting.valuePlaceholder')"
        />
        <div v-if="valueError" class="el-form-item__error setting-value-error">
          {{ valueError }}
        </div>
        <el-button
          v-if="editor === 'json'"
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
      <el-button :disabled="submitting" @click="visible = false">{{
        t('setting.cancel')
      }}</el-button>
      <el-button
        data-testid="setting-save"
        type="primary"
        :loading="submitting"
        :disabled="uploading || !canSave"
        @click="save"
      >
        {{ t('setting.save') }}
      </el-button>
    </template>
  </AppDialog>
</template>

<style scoped lang="scss">
.setting-submit-error {
  margin-bottom: 16px;
}
.setting-media-hint {
  width: 100%;
  margin: 8px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}
.setting-json-format {
  margin-top: 6px;
  margin-left: auto;
}

.setting-value-error {
  position: static;
  width: 100%;
}
</style>
