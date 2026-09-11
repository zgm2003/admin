<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as smsApi from '@/api/message/sms'
import type { TablePaginationState } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'
import SmsConfigTab from './config/index.vue'
import SmsLogTab, { type SmsLogFilter } from './log/index.vue'
import SmsRateLimitPolicyTab from './rateLimitPolicy/index.vue'
import SmsRecipientRuleTab from './recipientRule/index.vue'
import SmsTemplateTab from './template/index.vue'

type TabName = 'config' | 'templates' | 'logs' | 'rules' | 'policies'

const access = usePermissionStore()
const { t } = useI18n()
const activeTab = ref<TabName>('config')
const catalogReady = ref(false)
const catalogError = ref('')
const config = ref<smsApi.SmsConfig>({
  configured: false,
  smsSdkAppId: '',
  signName: '',
  region: '',
  endpoint: '',
  ttlMinutes: 5,
  isEnabled: YesNo.No,
  lastTestAt: null,
  lastTestError: '',
})
const templates = ref<smsApi.SmsTemplate[]>([])
const rules = ref<smsApi.SmsRule[]>([])
const logs = ref<smsApi.SmsLog[]>([])
const policies = ref<smsApi.SmsRateLimitPlatform[]>([])
const logPage = ref(1)
const logPageSize = ref(20)
const logTotal = ref(0)
const logFilter = ref<SmsLogFilter>(blankLogFilter())
const loading = reactive<Record<TabName, boolean>>({
  config: false,
  templates: false,
  logs: false,
  rules: false,
  policies: false,
})
const errors = reactive<Record<TabName, string>>({
  config: '',
  templates: '',
  logs: '',
  rules: '',
  policies: '',
})
let logRequestSequence = 0

const can = (code: string) => access.hasPermission(code)
const canList = computed(() => can('message:sms:list'))
const visibleTabs = computed(() =>
  canList.value
    ? [
        { name: 'config' as const, label: t('sms.tab.config') },
        { name: 'templates' as const, label: t('sms.tab.templates') },
        { name: 'logs' as const, label: t('sms.tab.logs') },
        { name: 'rules' as const, label: t('sms.tab.rules') },
        { name: 'policies' as const, label: t('sms.tab.policies') },
      ]
    : [],
)
const sceneOptions = computed<Array<{ label: string; value: smsApi.SmsScene }>>(() => [
  { value: 'login', label: t('sms.scene.login') },
  { value: 'forget', label: t('sms.scene.forget') },
  { value: 'bind_phone', label: t('sms.scene.bindPhone') },
  { value: 'change_password', label: t('sms.scene.changePassword') },
])

function blankLogFilter(): SmsLogFilter {
  return { platform: '', toPhone: '', scene: '', status: '', timeRange: [] }
}

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message !== '' ? error.message : t('sms.loadFailed')
}

async function loadCatalog(): Promise<void> {
  catalogError.value = ''
  try {
    await smsApi.getSmsPageInit()
    catalogReady.value = true
  } catch (error: unknown) {
    catalogReady.value = false
    catalogError.value = errorMessage(error)
  }
}

async function loadConfig(): Promise<void> {
  await loadInto('config', async () => {
    config.value = await smsApi.getSmsConfig()
  })
}

async function loadTemplates(): Promise<void> {
  await loadInto('templates', async () => {
    templates.value = await smsApi.listSmsTemplates()
  })
}

async function loadRules(): Promise<void> {
  await loadInto('rules', async () => {
    rules.value = await smsApi.listSmsRules()
  })
}

async function loadPolicies(): Promise<void> {
  await loadInto('policies', async () => {
    policies.value = (await smsApi.listSmsRateLimitPolicies()).platforms
  })
}

async function loadLogs(): Promise<void> {
  const sequence = ++logRequestSequence
  loading.logs = true
  errors.logs = ''
  const filter = logFilter.value
  const [from, to] = filter.timeRange
  try {
    const result = await smsApi.listSmsLogs({
      page: logPage.value,
      pageSize: logPageSize.value,
      ...(filter.platform.trim() === '' ? {} : { platform: filter.platform.trim() }),
      ...(filter.toPhone.trim() === '' ? {} : { toPhone: filter.toPhone.trim() }),
      ...(filter.scene === '' ? {} : { scene: filter.scene }),
      ...(filter.status === '' ? {} : { status: filter.status }),
      ...(from === undefined ? {} : { from }),
      ...(to === undefined ? {} : { to }),
    })
    if (sequence !== logRequestSequence) return
    logs.value = result.list
    logTotal.value = result.total
  } catch (error: unknown) {
    if (sequence !== logRequestSequence) return
    errors.logs = errorMessage(error)
  } finally {
    if (sequence === logRequestSequence) loading.logs = false
  }
}

async function loadInto(tab: Exclude<TabName, 'logs'>, load: () => Promise<void>): Promise<void> {
  loading[tab] = true
  errors[tab] = ''
  try {
    await load()
  } catch (error: unknown) {
    errors[tab] = errorMessage(error)
  } finally {
    loading[tab] = false
  }
}

function loadActiveTab(): Promise<void> {
  if (!canList.value) return Promise.resolve()
  if (activeTab.value === 'config') return loadConfig()
  if (activeTab.value === 'templates') return loadTemplates()
  if (activeTab.value === 'logs') return loadLogs()
  if (activeTab.value === 'rules') return loadRules()
  return loadPolicies()
}

function searchLogs(value: SmsLogFilter): void {
  logFilter.value = value
  logPage.value = 1
  void loadLogs()
}

function changeLogPage(value: TablePaginationState): void {
  logPage.value = value.currentPage
  logPageSize.value = value.pageSize
  void loadLogs()
}

watch(activeTab, () => void loadActiveTab())
onMounted(() => {
  if (!canList.value) return
  void loadCatalog()
  void loadActiveTab()
})
</script>

<template>
  <section class="sms-page management-page" data-testid="sms-page">
    <el-empty v-if="!canList" :description="t('sms.listPermissionRequired')" />
    <template v-else>
      <el-alert
        v-if="catalogError"
        class="sms-page__catalog-error"
        :title="catalogError"
        type="warning"
        :closable="false"
        show-icon
      />
      <el-tabs v-model="activeTab" class="sms-tabs">
        <el-tab-pane v-for="tab in visibleTabs" :key="tab.name" :name="tab.name" :label="tab.label">
          <div class="sms-panel">
            <div v-if="errors[tab.name]" class="sms-panel__error" role="alert">
              <span>{{ errors[tab.name] }}</span>
              <el-button data-testid="sms-tab-retry" type="primary" plain @click="loadActiveTab">
                {{ t('sms.retry') }}
              </el-button>
            </div>
            <SmsConfigTab
              v-if="tab.name === 'config' && !errors.config"
              :config="config"
              :scene-options="sceneOptions"
              :catalog-ready="catalogReady"
              :loading="loading.config"
              :can-update="can('message:sms:config:update')"
              :can-delete="can('message:sms:config:delete')"
              :can-test="can('message:sms:test')"
              @saved="loadConfig"
              @deleted="loadConfig"
            />
            <SmsTemplateTab
              v-else-if="tab.name === 'templates' && !errors.templates"
              :templates="templates"
              :loading="loading.templates"
              :can-update="can('message:sms:template:update')"
              :can-status="can('message:sms:template:status')"
              @refresh="loadTemplates"
            />
            <SmsLogTab
              v-else-if="tab.name === 'logs' && !errors.logs"
              :logs="logs"
              :scene-options="sceneOptions"
              :total="logTotal"
              :page="logPage"
              :page-size="logPageSize"
              :loading="loading.logs"
              :can-detail="can('message:sms:detail')"
              @refresh="loadLogs"
              @search="searchLogs"
              @page-change="changeLogPage"
            />
            <SmsRecipientRuleTab
              v-else-if="tab.name === 'rules' && !errors.rules"
              :rules="rules"
              :loading="loading.rules"
              :can-create="can('message:sms:rule:create')"
              :can-update="can('message:sms:rule:update')"
              :can-status="can('message:sms:rule:status')"
              :can-delete="can('message:sms:rule:delete')"
              @refresh="loadRules"
            />
            <SmsRateLimitPolicyTab
              v-else-if="tab.name === 'policies' && !errors.policies"
              :platforms="policies"
              :loading="loading.policies"
              :can-update="can('message:sms:rate-limit:update')"
              @refresh="loadPolicies"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </template>
  </section>
</template>

<style scoped>
.sms-page {
  min-width: 0;
}

.sms-page__catalog-error {
  margin-bottom: 12px;
}

.sms-tabs :deep(.el-tabs__content) {
  padding-top: 16px;
}

.sms-panel {
  min-height: 240px;
}

.sms-panel__error {
  display: flex;
  min-height: 180px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 12px;
  color: var(--el-color-danger);
  text-align: center;
}
</style>
