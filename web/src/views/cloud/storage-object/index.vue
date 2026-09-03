<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import { useI18n } from 'vue-i18n'

import type { TablePaginationState } from '@/components/AppTable'
import type { SearchFormModel } from '@/components/AppSearch'
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
import ConfigTable from './components/ConfigTable/index.vue'
import RuleDialog from './components/RuleDialog/index.vue'
import RuleTable from './components/RuleTable/index.vue'
import {
  commonExtensionOptions,
  commonMimeTypeOptions,
  cosRegionOptions,
  useStorageForms,
} from './storage-forms'
import {
  createConfigColumns,
  createConfigSearchFields,
  createRuleColumns,
  createRuleSearchFields,
} from './storage-view'

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
const {
  allExtensionsSelected,
  allMimeTypesSelected,
  configDialog,
  configForm,
  configRules,
  configUrlErrors,
  editingConfig,
  editingRule,
  normalizeRuleValues,
  openConfig: openConfigForm,
  openRule: openRuleForm,
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
} = useStorageForms(t)
const configDialogRef = ref<InstanceType<typeof ConfigDialog>>()
const ruleDialogRef = ref<InstanceType<typeof RuleDialog>>()

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
const configSearchFields = computed(() => createConfigSearchFields(t))
const ruleSearchFields = computed(() =>
  createRuleSearchFields(t, platforms.value, configOptions.value),
)
const configColumns = computed(() => createConfigColumns(t))
const ruleColumns = computed(() => createRuleColumns(t))

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
  openConfigForm(row)
  mutationError.value = ''
}
function openRule(row?: UploadRule): void {
  if (!row && !canAddRule.value) {
    ElMessage.warning(t('storage.rulePrerequisite'))
    return
  }
  openRuleForm(row, {
    platformId: platforms.value[0]?.id ?? 0,
    cosConfigId: configOptions.value[0]?.id ?? 0,
  })
  mutationError.value = ''
}
async function saveConfig(): Promise<void> {
  const valid = await (configDialogRef.value?.validate() ?? Promise.resolve(false)).catch(
    () => false,
  )
  if (!valid || !validateConfigURLs()) return
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
  } catch {
    /* request.ts provides the single error notification */
  }
}
async function saveRule(): Promise<void> {
  const normalizedValues = normalizeRuleValues()
  if (normalizedValues === null) return
  const { allowedExtensions, allowedMimeTypes } = normalizedValues
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
  } catch (error: unknown) {
    mutationError.value = errorMessage(error)
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
        <ConfigTable
          v-model="configSearchModel"
          :columns="configColumns"
          :data="configs"
          :fields="configSearchFields"
          :loading="loading"
          :pagination="configPagination"
          :can="can"
          :can-create="canCreateConfig"
          :can-update="canUpdateConfig"
          @query="searchConfigs"
          @reset="resetConfigs"
          @refresh="loadConfigs"
          @pagination="updateConfigPagination"
          @open="openConfig"
          @test="testConfigConnection"
          @status="toggleConfig"
          @delete="removeConfig"
        />
      </el-tab-pane>
      <el-tab-pane name="rules" :label="t('storage.rulesTab')" data-testid="storage-rules-tab">
        <RuleTable
          v-model="ruleSearchModel"
          :columns="ruleColumns"
          :data="rules"
          :fields="ruleSearchFields"
          :loading="loading"
          :pagination="rulePagination"
          :can="can"
          :can-add="canAddRule"
          :can-create="canCreateRule"
          :can-update="canUpdateRule"
          :missing-prerequisite="platforms.length === 0 || configOptions.length === 0"
          @query="searchRules"
          @reset="resetRules"
          @refresh="loadRules"
          @pagination="updateRulePagination"
          @open="openRule"
          @status="toggleRule"
          @delete="removeRule"
        />
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
