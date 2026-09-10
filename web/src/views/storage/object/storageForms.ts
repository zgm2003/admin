import { computed, ref, type ComputedRef } from 'vue'
import type { FormRules } from 'element-plus'

import type { CosConfig } from '@/api/storage/cosConfig'
import type { UploadRule } from '@/api/storage/uploadRule'
import { YesNo } from '@/enums/yesNo'
import type { ConfigForm, RuleForm } from './components/types'

const bytesPerMegabyte = 1024 * 1024

function blankConfig(): ConfigForm {
  return {
    name: '',
    appId: '',
    secretId: '',
    secretKey: '',
    bucket: '',
    region: '',
    endpoint: null,
    bucketDomain: null,
    isEnabled: YesNo.Yes,
    remark: '',
  }
}

function blankRule(): RuleForm {
  return {
    platformId: 0,
    codes: [],
    name: '',
    cosConfigId: 0,
    maxFileSizeBytes: 1_048_576,
    allowedExtensions: [],
    allowedMimeTypes: [],
    accessMode: 'private',
    isEnabled: YesNo.Yes,
    remark: '',
  }
}

function httpsURLError(value: string | null, message: string): string {
  const normalized = value?.trim() ?? ''
  if (normalized === '') return ''
  try {
    const url = new URL(normalized)
    if (
      url.protocol !== 'https:' ||
      url.username !== '' ||
      url.password !== '' ||
      url.search !== '' ||
      url.hash !== '' ||
      (url.pathname !== '' && url.pathname !== '/')
    ) {
      throw new Error(message)
    }
    return ''
  } catch {
    return message
  }
}

function toggleCommonOptions(
  current: string[],
  options: readonly string[],
  checked: boolean,
): string[] {
  if (checked) return [...new Set([...current, ...options])]
  return current.filter((item) => !options.includes(item))
}

export function useStorageForms(
  t: (key: string) => string,
  extensionValues: ComputedRef<readonly string[]>,
  mimeTypeValues: ComputedRef<readonly string[]>,
) {
  const configDialog = ref(false)
  const ruleDialog = ref(false)
  const editingConfig = ref<number | null>(null)
  const editingRule = ref<number | null>(null)
  const configForm = ref<ConfigForm>(blankConfig())
  const ruleForm = ref<RuleForm>(blankRule())
  const configUrlErrors = ref({ endpoint: '', bucketDomain: '' })
  const ruleExtensionsError = ref('')

  const configRules = computed<FormRules<ConfigForm>>(() => ({
    name: [
      { required: true, whitespace: true, message: t('storage.nameRequired'), trigger: 'blur' },
    ],
    appId: [
      { required: true, whitespace: true, message: t('storage.appIdRequired'), trigger: 'blur' },
    ],
    secretId: [
      {
        required: editingConfig.value === null,
        whitespace: true,
        message: t('storage.secretIdRequired'),
        trigger: 'blur',
      },
    ],
    secretKey: [
      {
        required: editingConfig.value === null,
        whitespace: true,
        message: t('storage.secretKeyRequired'),
        trigger: 'blur',
      },
    ],
    bucket: [
      { required: true, whitespace: true, message: t('storage.bucketRequired'), trigger: 'blur' },
    ],
    region: [{ required: true, message: t('storage.regionRequired'), trigger: 'change' }],
  }))
  const ruleRules = computed<FormRules<RuleForm>>(() => ({
    platformId: [
      {
        required: true,
        type: 'number',
        min: 1,
        message: t('storage.rulePlatformRequired'),
        trigger: 'change',
      },
    ],
    cosConfigId: [
      {
        required: true,
        type: 'number',
        min: 1,
        message: t('storage.ruleConfigRequired'),
        trigger: 'change',
      },
    ],
    codes: [
      {
        required: editingRule.value === null,
        validator: (_rule, value, callback) =>
          Array.isArray(value) && value.length > 0
            ? callback()
            : callback(new Error(t('storage.ruleCodeRequired'))),
        trigger: 'change',
      },
    ],
    name: [
      { required: true, whitespace: true, message: t('storage.ruleNameRequired'), trigger: 'blur' },
    ],
    maxFileSizeBytes: [
      {
        required: true,
        type: 'number',
        min: 1,
        message: t('storage.maxFileSizeRequired'),
        trigger: 'change',
      },
    ],
    allowedExtensions: [
      {
        required: true,
        validator: (_rule, value, callback) =>
          Array.isArray(value) && value.length > 0
            ? callback()
            : callback(new Error(t('storage.extensionsRequired'))),
        trigger: 'change',
      },
    ],
    accessMode: [{ required: true, message: t('storage.accessModeRequired'), trigger: 'change' }],
  }))
  const ruleMaxFileSizeMB = computed<number>({
    get: () => ruleForm.value.maxFileSizeBytes / bytesPerMegabyte,
    set: (value) => {
      ruleForm.value.maxFileSizeBytes = Math.round(value * bytesPerMegabyte)
    },
  })
  const allExtensionsSelected = computed(
    () =>
      extensionValues.value.length > 0 &&
      extensionValues.value.every((item) => ruleForm.value.allowedExtensions.includes(item)),
  )
  const someExtensionsSelected = computed(
    () =>
      !allExtensionsSelected.value &&
      extensionValues.value.some((item) => ruleForm.value.allowedExtensions.includes(item)),
  )
  const allMimeTypesSelected = computed(
    () =>
      mimeTypeValues.value.length > 0 &&
      mimeTypeValues.value.every((item) => ruleForm.value.allowedMimeTypes.includes(item)),
  )
  const someMimeTypesSelected = computed(
    () =>
      !allMimeTypesSelected.value &&
      mimeTypeValues.value.some((item) => ruleForm.value.allowedMimeTypes.includes(item)),
  )

  function validateConfigURLField(field: 'endpoint' | 'bucketDomain'): void {
    const message =
      field === 'endpoint' ? t('storage.endpointHttpsRequired') : t('storage.domainHttpsRequired')
    configUrlErrors.value[field] = httpsURLError(configForm.value[field], message)
  }

  function validateConfigURLs(): boolean {
    configUrlErrors.value = {
      endpoint: httpsURLError(configForm.value.endpoint, t('storage.endpointHttpsRequired')),
      bucketDomain: httpsURLError(configForm.value.bucketDomain, t('storage.domainHttpsRequired')),
    }
    return configUrlErrors.value.endpoint === '' && configUrlErrors.value.bucketDomain === ''
  }

  function openConfig(row: CosConfig | undefined, defaultRegion: string): void {
    editingConfig.value = row?.id ?? null
    configForm.value = row
      ? { ...blankConfig(), ...row, secretId: '', secretKey: '' }
      : { ...blankConfig(), region: defaultRegion }
    configUrlErrors.value = { endpoint: '', bucketDomain: '' }
    configDialog.value = true
  }

  function openRule(
    row: UploadRule | undefined,
    defaults: { platformId: number; cosConfigId: number },
  ): void {
    editingRule.value = row?.id ?? null
    ruleForm.value = row ? { ...row } : { ...blankRule(), ...defaults }
    ruleExtensionsError.value = ''
    ruleDialog.value = true
  }

  function normalizeRuleValues(): {
    allowedExtensions: string[]
    allowedMimeTypes: string[]
  } | null {
    const allowedExtensions = ruleForm.value.allowedExtensions
      .map((item) => item.trim().toLowerCase().replace(/^\./, ''))
      .filter(Boolean)
    const allowedMimeTypes = ruleForm.value.allowedMimeTypes
      .map((item) => item.trim().toLowerCase())
      .filter(Boolean)
    ruleForm.value.allowedExtensions = allowedExtensions
    ruleForm.value.allowedMimeTypes = allowedMimeTypes
    if (allowedExtensions.length === 0) {
      ruleExtensionsError.value = t('storage.extensionsRequired')
      return null
    }
    ruleExtensionsError.value = ''
    return { allowedExtensions, allowedMimeTypes }
  }

  function toggleAllExtensions(checked: boolean | string | number): void {
    ruleForm.value.allowedExtensions = toggleCommonOptions(
      ruleForm.value.allowedExtensions,
      extensionValues.value,
      checked === true,
    )
    ruleExtensionsError.value = ''
  }

  function toggleAllMimeTypes(checked: boolean | string | number): void {
    ruleForm.value.allowedMimeTypes = toggleCommonOptions(
      ruleForm.value.allowedMimeTypes,
      mimeTypeValues.value,
      checked === true,
    )
  }

  return {
    allExtensionsSelected,
    allMimeTypesSelected,
    configDialog,
    configForm,
    configRules,
    configUrlErrors,
    editingConfig,
    editingRule,
    normalizeRuleValues,
    openConfig,
    openRule,
    ruleDialog,
    ruleExtensionsError,
    ruleForm,
    ruleMaxFileSizeMB,
    ruleRules,
    someExtensionsSelected,
    someMimeTypesSelected,
    toggleAllExtensions,
    toggleAllMimeTypes,
    validateConfigURLField,
    validateConfigURLs,
  }
}
