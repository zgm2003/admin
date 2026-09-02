<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { CirclePlus } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import type { FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'

import { AppTable } from '@/components/AppTable'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { AppSearch } from '@/components/AppSearch'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import { YesNo } from '@/enums/yes-no'
import {
  createCosConfig,
  deleteCosConfig,
  listCosConfigs,
  testCosConfig,
  updateCosConfig,
  updateCosConfigStatus,
  type CosConfig,
  type CosConfigQuery,
  type CreateCosConfigInput,
  type UpdateCosConfigInput,
} from '@/api/storage/cosconfig'
import {
  createUploadRule,
  deleteUploadRule,
  getUploadRulePageInit,
  listUploadRules,
  updateUploadRule,
  updateUploadRuleStatus,
  type ConfigSummary,
  type PlatformOption,
  type UploadRule,
  type UploadRuleQuery,
} from '@/api/storage/uploadrule'
import { usePermissionStore } from '@/store/permission'
import ConfigDialog from './components/ConfigDialog/index.vue'
import RuleDialog from './components/RuleDialog/index.vue'
import type { ConfigForm, RuleForm } from './components/types'

const { t } = useI18n()
const access = usePermissionStore()
const activeTab = ref<'config' | 'rules'>('config')
const configs = ref<CosConfig[]>([])
const rules = ref<UploadRule[]>([])
const configTotal = ref(0)
const ruleTotal = ref(0)
const configQuery = ref<CosConfigQuery>({ page: 1, pageSize: 20 })
const ruleQuery = ref<UploadRuleQuery>({ page: 1, pageSize: 20 })
const configKeyword = ref('')
const configStatus = ref<'' | YesNo>('')
const ruleKeyword = ref('')
const ruleStatus = ref<'' | YesNo>('')
const rulePlatform = ref<'' | number>('')
const ruleConfig = ref<'' | number>('')
const platforms = ref<PlatformOption[]>([])
const configOptions = ref<ConfigSummary[]>([])
const loading = ref(false)
const loadError = ref('')
const mutationError = ref('')
const configDialog = ref(false)
const configUrlErrors = ref({ endpoint: '', bucketDomain: '' })
const ruleDialog = ref(false)
const ruleExtensionsError = ref('')
const editingConfig = ref<number | null>(null)
const editingRule = ref<number | null>(null)

const cosRegionOptions = [
  { value: 'ap-guangzhou', label: '广州（ap-guangzhou）' },
  { value: 'ap-shanghai', label: '上海（ap-shanghai）' },
  { value: 'ap-nanjing', label: '南京（ap-nanjing）' },
  { value: 'ap-beijing', label: '北京（ap-beijing）' },
  { value: 'ap-chengdu', label: '成都（ap-chengdu）' },
  { value: 'ap-chongqing', label: '重庆（ap-chongqing）' },
  { value: 'ap-hongkong', label: '中国香港（ap-hongkong）' },
  { value: 'ap-singapore', label: '新加坡（ap-singapore）' },
  { value: 'ap-tokyo', label: '东京（ap-tokyo）' },
  { value: 'ap-seoul', label: '首尔（ap-seoul）' },
  { value: 'eu-frankfurt', label: '法兰克福（eu-frankfurt）' },
  { value: 'na-siliconvalley', label: '硅谷（na-siliconvalley）' },
] as const

const commonExtensionOptions = [
  'jpg',
  'jpeg',
  'png',
  'gif',
  'webp',
  'pdf',
  'doc',
  'docx',
  'xls',
  'xlsx',
  'zip',
]
const commonMimeTypeOptions = [
  'image/jpeg',
  'image/png',
  'image/gif',
  'image/webp',
  'application/pdf',
  'application/zip',
]
const bytesPerMegabyte = 1024 * 1024

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
    )
      throw new Error(message)
    return ''
  } catch {
    return message
  }
}

function validateConfigURLField(field: 'endpoint' | 'bucketDomain'): void {
  const message =
    field === 'endpoint' ? t('storage.endpointHttpsRequired') : t('storage.domainHttpsRequired')
  configUrlErrors.value[field] = httpsURLError(configForm.value[field], message)
}

const configRules = computed<FormRules<ConfigForm>>(() => ({
  name: [{ required: true, whitespace: true, message: t('storage.nameRequired'), trigger: 'blur' }],
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

const blankConfig = (): ConfigForm => ({
  name: '',
  appId: '',
  secretId: '',
  secretKey: '',
  bucket: '',
  region: 'ap-guangzhou',
  endpoint: null,
  bucketDomain: null,
  isEnabled: YesNo.Yes,
  remark: '',
})
const blankRule = (): RuleForm => ({
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
})
const configForm = ref<ConfigForm>(blankConfig())
const ruleForm = ref<RuleForm>(blankRule())
const configDialogRef = ref<InstanceType<typeof ConfigDialog>>()
const ruleDialogRef = ref<InstanceType<typeof RuleDialog>>()
const ruleMaxFileSizeMB = computed<number>({
  get: () => ruleForm.value.maxFileSizeBytes / bytesPerMegabyte,
  set: (value) => {
    ruleForm.value.maxFileSizeBytes = Math.round(value * bytesPerMegabyte)
  },
})
const allExtensionsSelected = computed(() =>
  commonExtensionOptions.every((item) => ruleForm.value.allowedExtensions.includes(item)),
)
const someExtensionsSelected = computed(
  () =>
    !allExtensionsSelected.value &&
    commonExtensionOptions.some((item) => ruleForm.value.allowedExtensions.includes(item)),
)
const allMimeTypesSelected = computed(() =>
  commonMimeTypeOptions.every((item) => ruleForm.value.allowedMimeTypes.includes(item)),
)
const someMimeTypesSelected = computed(
  () =>
    !allMimeTypesSelected.value &&
    commonMimeTypeOptions.some((item) => ruleForm.value.allowedMimeTypes.includes(item)),
)

function toggleCommonOptions(current: string[], options: string[], checked: boolean): string[] {
  if (checked) return [...new Set([...current, ...options])]
  return current.filter((item) => !options.includes(item))
}

function toggleAllExtensions(checked: boolean | string | number): void {
  ruleForm.value.allowedExtensions = toggleCommonOptions(
    ruleForm.value.allowedExtensions,
    commonExtensionOptions,
    checked === true,
  )
  ruleExtensionsError.value = ''
}

function toggleAllMimeTypes(checked: boolean | string | number): void {
  ruleForm.value.allowedMimeTypes = toggleCommonOptions(
    ruleForm.value.allowedMimeTypes,
    commonMimeTypeOptions,
    checked === true,
  )
}

const can = (code: string): boolean => access.hasPermission(code)
const canCreateConfig = computed(() => can('storage:cos-config:create'))
const canCreateRule = computed(() => can('storage:upload-rule:create'))
const canAddRule = computed(
  () => canCreateRule.value && platforms.value.length > 0 && configOptions.value.length > 0,
)
const canUpdateConfig = computed(() => can('storage:cos-config:update'))
const canUpdateRule = computed(() => can('storage:upload-rule:update'))

const configPagination = computed<TablePaginationState>(() => ({
  currentPage: configQuery.value.page,
  pageSize: configQuery.value.pageSize,
  total: configTotal.value,
}))
const rulePagination = computed<TablePaginationState>(() => ({
  currentPage: ruleQuery.value.page,
  pageSize: ruleQuery.value.pageSize,
  total: ruleTotal.value,
}))
const configSearchModel = computed<SearchFormModel>({
  get: () => ({ keyword: configKeyword.value, status: configStatus.value }),
  set: (value) => {
    configKeyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    configStatus.value = value.status === YesNo.Yes || value.status === YesNo.No ? value.status : ''
  },
})
const ruleSearchModel = computed<SearchFormModel>({
  get: () => ({
    keyword: ruleKeyword.value,
    status: ruleStatus.value,
    platform: rulePlatform.value,
    config: ruleConfig.value,
  }),
  set: (value) => {
    ruleKeyword.value = typeof value.keyword === 'string' ? value.keyword : ''
    ruleStatus.value = value.status === YesNo.Yes || value.status === YesNo.No ? value.status : ''
    rulePlatform.value = typeof value.platform === 'number' ? value.platform : ''
    ruleConfig.value = typeof value.config === 'number' ? value.config : ''
  },
})
const configSearchFields = computed<SearchField[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('storage.keyword'),
    placeholder: t('storage.keyword'),
    width: 260,
    testId: 'storage-config-keyword',
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('storage.status'),
    options: [
      { label: t('storage.allStatus'), value: '' },
      { label: t('storage.enabled'), value: YesNo.Yes },
      { label: t('storage.disabled'), value: YesNo.No },
    ],
    width: 170,
  },
])
const ruleSearchFields = computed<SearchField[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('storage.keyword'),
    placeholder: t('storage.keyword'),
    width: 220,
    testId: 'storage-rule-keyword',
  },
  {
    key: 'platform',
    type: 'select-v2',
    label: t('storage.platform'),
    options: [
      { label: t('storage.allPlatforms'), value: '' },
      ...platforms.value.map((item) => ({ label: item.name, value: item.id })),
    ],
    width: 180,
  },
  {
    key: 'config',
    type: 'select-v2',
    label: t('storage.config'),
    options: [
      { label: t('storage.allConfigs'), value: '' },
      ...configOptions.value.map((item) => ({ label: item.name, value: item.id })),
    ],
    width: 180,
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('storage.status'),
    options: [
      { label: t('storage.allStatus'), value: '' },
      { label: t('storage.enabled'), value: YesNo.Yes },
      { label: t('storage.disabled'), value: YesNo.No },
    ],
    width: 170,
  },
])
const configColumns = computed<TableColumn<CosConfig>[]>(() => [
  { prop: 'name', label: t('storage.name'), minWidth: 160 },
  { prop: 'bucket', label: t('storage.bucket'), minWidth: 190 },
  { prop: 'region', label: t('storage.region'), width: 150 },
  { key: 'credentials', prop: 'id', label: t('storage.credentials'), width: 130 },
  { key: 'status', prop: 'id', label: t('storage.status'), width: 110 },
  { key: 'actions', prop: 'id', label: t('storage.actions'), width: 310 },
])
const ruleColumns = computed<TableColumn<UploadRule>[]>(() => [
  { prop: 'name', label: t('storage.name'), minWidth: 150 },
  { prop: 'platformName', label: t('storage.platform'), width: 150 },
  { prop: 'cosConfigName', label: t('storage.config'), width: 170 },
  { key: 'codes', prop: 'codes', label: t('storage.code'), minWidth: 220 },
  { key: 'status', prop: 'id', label: t('storage.status'), width: 110 },
  { key: 'actions', prop: 'id', label: t('storage.actions'), width: 250 },
])

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message ? error.message : t('storage.loadFailed')
}
async function loadConfigs(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const page = await listCosConfigs(configQuery.value)
    configs.value = page.list
    configTotal.value = page.total
  } catch (error: unknown) {
    loadError.value = errorMessage(error)
  } finally {
    loading.value = false
  }
}
async function loadRules(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const init = await getUploadRulePageInit()
    platforms.value = init.platforms
    configOptions.value = init.configs
    const page = await listUploadRules(ruleQuery.value)
    rules.value = page.list
    ruleTotal.value = page.total
  } catch (error: unknown) {
    loadError.value = errorMessage(error)
  } finally {
    loading.value = false
  }
}
async function switchTab(tab: string): Promise<void> {
  activeTab.value = tab as 'config' | 'rules'
  if (activeTab.value === 'config') await loadConfigs()
  else await loadRules()
}
function searchConfigs(): void {
  configQuery.value = {
    page: 1,
    pageSize: configQuery.value.pageSize,
    ...(configKeyword.value.trim() ? { keyword: configKeyword.value.trim() } : {}),
    ...(configStatus.value ? { isEnabled: configStatus.value } : {}),
  }
  void loadConfigs()
}
function resetConfigs(): void {
  configKeyword.value = ''
  configStatus.value = ''
  configQuery.value = { page: 1, pageSize: configQuery.value.pageSize }
  void loadConfigs()
}
function searchRules(): void {
  ruleQuery.value = {
    page: 1,
    pageSize: ruleQuery.value.pageSize,
    ...(ruleKeyword.value.trim() ? { keyword: ruleKeyword.value.trim() } : {}),
    ...(rulePlatform.value === '' ? {} : { platformId: rulePlatform.value }),
    ...(ruleConfig.value === '' ? {} : { cosConfigId: ruleConfig.value }),
    ...(ruleStatus.value ? { isEnabled: ruleStatus.value } : {}),
  }
  void loadRules()
}
function resetRules(): void {
  ruleKeyword.value = ''
  ruleStatus.value = ''
  rulePlatform.value = ''
  ruleConfig.value = ''
  ruleQuery.value = { page: 1, pageSize: ruleQuery.value.pageSize }
  void loadRules()
}
function updateConfigPagination(next: TablePaginationState): void {
  configQuery.value = { ...configQuery.value, page: next.currentPage, pageSize: next.pageSize }
  void loadConfigs()
}
function updateRulePagination(next: TablePaginationState): void {
  ruleQuery.value = { ...ruleQuery.value, page: next.currentPage, pageSize: next.pageSize }
  void loadRules()
}
function openConfig(row?: CosConfig): void {
  editingConfig.value = row?.id ?? null
  configForm.value = row ? { ...blankConfig(), ...row, secretId: '', secretKey: '' } : blankConfig()
  configUrlErrors.value = { endpoint: '', bucketDomain: '' }
  mutationError.value = ''
  configDialog.value = true
}
function openRule(row?: UploadRule): void {
  if (!row && !canAddRule.value) {
    ElMessage.warning(t('storage.rulePrerequisite'))
    return
  }
  editingRule.value = row?.id ?? null
  ruleForm.value = row
    ? { ...row }
    : {
        ...blankRule(),
        platformId: platforms.value[0]?.id ?? 0,
        cosConfigId: configOptions.value[0]?.id ?? 0,
      }
  mutationError.value = ''
  ruleExtensionsError.value = ''
  ruleDialog.value = true
}
async function saveConfig(): Promise<void> {
  const valid = await (configDialogRef.value?.validate() ?? Promise.resolve(false)).catch(
    () => false,
  )
  configUrlErrors.value = {
    endpoint: httpsURLError(configForm.value.endpoint, t('storage.endpointHttpsRequired')),
    bucketDomain: httpsURLError(configForm.value.bucketDomain, t('storage.domainHttpsRequired')),
  }
  if (!valid || configUrlErrors.value.endpoint !== '' || configUrlErrors.value.bucketDomain !== '')
    return
  const base = {
    name: configForm.value.name.trim(),
    appId: configForm.value.appId.trim(),
    bucket: configForm.value.bucket.trim(),
    region: configForm.value.region.trim(),
    endpoint: configForm.value.endpoint || null,
    bucketDomain: configForm.value.bucketDomain || null,
    remark: configForm.value.remark.trim(),
  }
  try {
    if (editingConfig.value) {
      const data: UpdateCosConfigInput = {
        ...base,
        ...(configForm.value.secretId ? { secretId: configForm.value.secretId } : {}),
        ...(configForm.value.secretKey ? { secretKey: configForm.value.secretKey } : {}),
      }
      await updateCosConfig(editingConfig.value, data)
    } else {
      const data: CreateCosConfigInput = {
        ...base,
        secretId: configForm.value.secretId,
        secretKey: configForm.value.secretKey,
        isEnabled: configForm.value.isEnabled,
      }
      await createCosConfig(data)
    }
    configDialog.value = false
    ElNotification.success({ title: t('storage.saveSuccess') })
    await loadConfigs()
  } catch (error: unknown) {
    mutationError.value = errorMessage(error)
  }
}
async function saveRule(): Promise<void> {
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
    return
  }
  ruleExtensionsError.value = ''
  const valid = await (ruleDialogRef.value?.validate() ?? Promise.resolve(false)).catch(() => false)
  if (!valid) return
  const normalized = {
    name: ruleForm.value.name.trim(),
    cosConfigId: ruleForm.value.cosConfigId,
    maxFileSizeBytes: ruleForm.value.maxFileSizeBytes,
    allowedExtensions,
    allowedMimeTypes,
    accessMode: ruleForm.value.accessMode,
    remark: ruleForm.value.remark.trim(),
  }
  try {
    if (editingRule.value) await updateUploadRule(editingRule.value, normalized)
    else
      await createUploadRule({
        ...normalized,
        platformId: ruleForm.value.platformId,
        codes: ruleForm.value.codes.map((code) => code.trim().toLowerCase()).filter(Boolean),
        isEnabled: ruleForm.value.isEnabled,
      })
    ruleDialog.value = false
    ElNotification.success({ title: t('storage.saveSuccess') })
    await loadRules()
  } catch (error: unknown) {
    mutationError.value = errorMessage(error)
  }
}
async function toggleConfig(row: CosConfig): Promise<void> {
  try {
    await ElMessageBox.confirm(t('storage.confirmStatus'), t('storage.status'), { type: 'warning' })
    await updateCosConfigStatus(row.id, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
    await loadConfigs()
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') mutationError.value = errorMessage(error)
  }
}
async function toggleRule(row: UploadRule): Promise<void> {
  try {
    await ElMessageBox.confirm(t('storage.confirmStatus'), t('storage.status'), { type: 'warning' })
    await updateUploadRuleStatus(row.id, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
    await loadRules()
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') mutationError.value = errorMessage(error)
  }
}
async function testConfigConnection(row: CosConfig): Promise<void> {
  try {
    await testCosConfig(row.id)
    ElNotification.success({ title: t('storage.testSuccess') })
  } catch {
    /* request.ts provides the single error notification */
  }
}
async function removeConfig(row: CosConfig): Promise<void> {
  try {
    await ElMessageBox.confirm(t('storage.confirmDelete'), t('storage.delete'), { type: 'warning' })
    await deleteCosConfig(row.id)
    await loadConfigs()
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') mutationError.value = errorMessage(error)
  }
}
async function removeRule(row: UploadRule): Promise<void> {
  try {
    await ElMessageBox.confirm(t('storage.confirmDelete'), t('storage.delete'), { type: 'warning' })
    await deleteUploadRule(row.id)
    await loadRules()
  } catch (error: unknown) {
    if (error !== 'cancel' && error !== 'close') mutationError.value = errorMessage(error)
  }
}

onMounted(() => {
  void loadConfigs()
})
</script>

<template>
  <section class="storage-page management-page">
    <el-alert v-if="loadError" :title="loadError" type="error" show-icon />
    <el-alert
      v-if="mutationError"
      :title="mutationError"
      type="error"
      show-icon
      closable
      @close="mutationError = ''"
    />
    <el-tabs v-model="activeTab" @tab-change="switchTab">
      <el-tab-pane name="config" :label="t('storage.configTab')" data-testid="storage-config-tab">
        <AppSearch
          v-model="configSearchModel"
          :fields="configSearchFields"
          :query-label="t('storage.search')"
          :reset-label="t('storage.reset')"
          query-test-id="storage-config-search"
          reset-test-id="storage-config-reset"
          @query="searchConfigs"
          @reset="resetConfigs"
        />
        <AppTable
          :columns="configColumns"
          :data="configs"
          :loading="loading"
          :pagination="configPagination"
          result-state="success"
          :aria-label="t('storage.configTab')"
          :refresh-label="t('storage.refresh')"
          @refresh="loadConfigs"
          @update:pagination="updateConfigPagination"
        >
          <template #toolbar-left
            ><el-button
              v-if="canCreateConfig"
              type="primary"
              :icon="CirclePlus"
              data-testid="storage-add-config"
              @click="openConfig()"
              >{{ t('storage.addConfig') }}</el-button
            ></template
          >
          <template #cell-credentials="{ row }"
            ><el-tag size="small" :type="row.hasCredentials ? 'success' : 'warning'">{{
              row.hasCredentials ? t('storage.configured') : t('storage.missing')
            }}</el-tag></template
          >
          <template #cell-status="{ row }"
            ><el-tag size="small" :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'">{{
              row.isEnabled === YesNo.Yes ? t('storage.enabled') : t('storage.disabled')
            }}</el-tag></template
          >
          <template #cell-actions="{ row }"
            ><el-space wrap :size="4"
              ><el-button v-if="canUpdateConfig" text type="primary" @click="openConfig(row)">{{
                t('storage.edit')
              }}</el-button
              ><el-button
                v-if="can('storage:cos-config:test')"
                text
                type="primary"
                @click="testConfigConnection(row)"
                >{{ t('storage.test') }}</el-button
              ><el-button
                v-if="can('storage:cos-config:status')"
                text
                type="warning"
                @click="toggleConfig(row)"
                >{{
                  row.isEnabled === YesNo.Yes ? t('storage.disable') : t('storage.enable')
                }}</el-button
              ><el-button
                v-if="can('storage:cos-config:delete')"
                text
                type="danger"
                @click="removeConfig(row)"
                >{{ t('storage.delete') }}</el-button
              ></el-space
            ></template
          >
        </AppTable>
      </el-tab-pane>
      <el-tab-pane name="rules" :label="t('storage.rulesTab')" data-testid="storage-rules-tab">
        <AppSearch
          v-model="ruleSearchModel"
          :fields="ruleSearchFields"
          :query-label="t('storage.search')"
          :reset-label="t('storage.reset')"
          query-test-id="storage-rule-search"
          reset-test-id="storage-rule-reset"
          @query="searchRules"
          @reset="resetRules"
        />
        <el-alert
          v-if="canCreateRule && (platforms.length === 0 || configOptions.length === 0)"
          :title="t('storage.rulePrerequisite')"
          type="warning"
          show-icon
        />
        <AppTable
          :columns="ruleColumns"
          :data="rules"
          :loading="loading"
          :pagination="rulePagination"
          result-state="success"
          :aria-label="t('storage.rulesTab')"
          :refresh-label="t('storage.refresh')"
          @refresh="loadRules"
          @update:pagination="updateRulePagination"
        >
          <template #toolbar-left
            ><el-button
              v-if="canCreateRule"
              type="primary"
              :icon="CirclePlus"
              data-testid="storage-add-rule"
              :disabled="!canAddRule"
              @click="openRule()"
              >{{ t('storage.addRule') }}</el-button
            ></template
          >
          <template #cell-codes="{ row }"
            ><el-space wrap :size="4"
              ><el-tag v-for="code in row.codes" :key="code" size="small">{{
                code
              }}</el-tag></el-space
            ></template
          >
          <template #cell-status="{ row }"
            ><el-tag size="small" :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'">{{
              row.isEnabled === YesNo.Yes ? t('storage.enabled') : t('storage.disabled')
            }}</el-tag></template
          >
          <template #cell-actions="{ row }"
            ><el-space wrap :size="4"
              ><el-button v-if="canUpdateRule" text type="primary" @click="openRule(row)">{{
                t('storage.edit')
              }}</el-button
              ><el-button
                v-if="can('storage:upload-rule:status')"
                text
                type="warning"
                @click="toggleRule(row)"
                >{{
                  row.isEnabled === YesNo.Yes ? t('storage.disable') : t('storage.enable')
                }}</el-button
              ><el-button
                v-if="can('storage:upload-rule:delete')"
                text
                type="danger"
                @click="removeRule(row)"
                >{{ t('storage.delete') }}</el-button
              ></el-space
            ></template
          >
        </AppTable>
      </el-tab-pane>
    </el-tabs>

    <ConfigDialog
      ref="configDialogRef"
      v-model="configDialog"
      v-model:form="configForm"
      :editing="editingConfig !== null"
      :rules="configRules"
      :url-errors="configUrlErrors"
      :regions="cosRegionOptions"
      :validate-url-field="validateConfigURLField"
      @save="saveConfig"
    />
    <RuleDialog
      ref="ruleDialogRef"
      v-model="ruleDialog"
      v-model:form="ruleForm"
      :editing="editingRule !== null"
      :rules="ruleRules"
      :platforms="platforms"
      :configs="configOptions"
      :file-size-mb="ruleMaxFileSizeMB"
      :extensions="commonExtensionOptions"
      :mime-types="commonMimeTypeOptions"
      :all-extensions-selected="allExtensionsSelected"
      :some-extensions-selected="someExtensionsSelected"
      :all-mime-types-selected="allMimeTypesSelected"
      :some-mime-types-selected="someMimeTypesSelected"
      :extensions-error="ruleExtensionsError"
      :toggle-all-extensions="toggleAllExtensions"
      :toggle-all-mime-types="toggleAllMimeTypes"
      @update:file-size-mb="ruleMaxFileSizeMB = $event"
      @save="saveRule"
    />
  </section>
</template>

<style scoped>
.storage-page {
  min-width: 0;
}
.storage-page :deep(.el-tabs__content) {
  min-height: 0;
}
.storage-page :deep(.el-select),
.storage-page :deep(.el-input-number) {
  width: 100%;
}
</style>
