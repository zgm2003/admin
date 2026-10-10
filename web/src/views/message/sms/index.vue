<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as smsApi from '@/api/message/sms'
import type { TablePaginationState } from '@/components/AppTable'
import {
  useMessageAggregateTabs,
  type MessageTabLoadContext,
} from '@/composables/useMessageAggregateTabs'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'
import SmsConfigTab from './config/index.vue'
import SmsLogTab, { type SmsLogFilter } from './log/index.vue'
import SmsRateLimitPolicyTab from './rateLimitPolicy/index.vue'
import SmsRecipientRuleTab from './recipientRule/index.vue'
import SmsTemplateTab from './template/index.vue'

type TabName = 'config' | 'templates' | 'logs' | 'rules' | 'policies'

const access = usePermissionStore()
const { t, locale } = useI18n()
const activeTab = ref<TabName>('config')
const options = ref<smsApi.SmsOptions | null>(null)
const optionsLoading = ref(false)
const catalogReady = computed(() => options.value !== null && !optionsLoading.value)
let catalogSequence = 0
const catalogError = ref('')
const config = ref<smsApi.SmsConfig>({
  configured: false,
  smsSdkAppId: '',
  signName: '',
  region: '',
  endpoint: '',
  ttlMinutes: 0,
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
const sceneOptions = computed(() => options.value?.scenes ?? [])

function blankLogFilter(): SmsLogFilter {
  return { platform: '', toPhone: '', scene: '', status: '', timeRange: [] }
}

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message !== '' ? error.message : t('sms.loadFailed')
}

async function loadCatalog(): Promise<void> {
  const sequence = ++catalogSequence
  options.value = null
  catalogError.value = ''
  optionsLoading.value = canList.value
  if (!canList.value) return
  try {
    const result = await smsApi.getSmsOptions()
    if (sequence === catalogSequence) options.value = result
  } catch (error: unknown) {
    if (sequence === catalogSequence) catalogError.value = errorMessage(error)
  } finally {
    if (sequence === catalogSequence) optionsLoading.value = false
  }
}

async function loadConfig(context?: MessageTabLoadContext): Promise<void> {
  const result = await smsApi.getSmsConfig()
  if (context === undefined || context.isCurrent()) config.value = result
}

async function loadTemplates(context?: MessageTabLoadContext): Promise<void> {
  const result = await smsApi.listSmsTemplates()
  if (context === undefined || context.isCurrent()) templates.value = result
}

async function loadRules(context?: MessageTabLoadContext): Promise<void> {
  const result = await smsApi.listSmsRules()
  if (context === undefined || context.isCurrent()) rules.value = result
}

async function loadPolicies(context?: MessageTabLoadContext): Promise<void> {
  const result = await smsApi.listSmsRateLimitPolicies()
  if (context === undefined || context.isCurrent()) policies.value = result.platforms
}

async function loadLogs(context: MessageTabLoadContext): Promise<void> {
  const filter = logFilter.value
  const [from, to] = filter.timeRange
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
  if (context.isCurrent()) {
    logs.value = result.list
    logTotal.value = result.total
  }
}

const { loading, errors, load } = useMessageAggregateTabs<TabName>({
  activeTab,
  canList,
  loaders: {
    config: loadConfig,
    templates: loadTemplates,
    logs: loadLogs,
    rules: loadRules,
    policies: loadPolicies,
  },
  errorMessage,
})

function searchLogs(value: SmsLogFilter): void {
  logFilter.value = value
  logPage.value = 1
  void load('logs')
}

function changeLogPage(value: TablePaginationState): void {
  logPage.value = value.currentPage
  logPageSize.value = value.pageSize
  void load('logs')
}

watch([locale, canList], () => void loadCatalog(), { immediate: true, flush: 'sync' })
onBeforeUnmount(() => {
  catalogSequence++
})
</script>

<template>
  <AppPage class="sms-page" data-testid="sms-page">
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
              <span>{{ t('sms.loadFailed') }}</span>
              <el-button data-testid="sms-tab-retry" type="primary" plain @click="load()">
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
              @saved="load('config')"
              @deleted="load('config')"
            />
            <SmsTemplateTab
              v-else-if="tab.name === 'templates' && !errors.templates"
              :templates="templates"
              :loading="loading.templates"
              :can-update="can('message:sms:template:update')"
              :can-status="can('message:sms:template:status')"
              @refresh="load('templates')"
            />
            <SmsLogTab
              v-else-if="tab.name === 'logs' && !errors.logs"
              :logs="logs"
              :options="options"
              :scene-options="sceneOptions"
              :total="logTotal"
              :page="logPage"
              :page-size="logPageSize"
              :loading="loading.logs"
              :can-detail="can('message:sms:detail')"
              @refresh="load('logs')"
              @search="searchLogs"
              @page-change="changeLogPage"
            />
            <SmsRecipientRuleTab
              v-else-if="tab.name === 'rules' && !errors.rules"
              :rules="rules"
              :options="options"
              :loading="loading.rules"
              :can-create="can('message:sms:rule:create')"
              :can-update="can('message:sms:rule:update')"
              :can-status="can('message:sms:rule:status')"
              :can-delete="can('message:sms:rule:delete')"
              @refresh="load('rules')"
            />
            <SmsRateLimitPolicyTab
              v-else-if="tab.name === 'policies' && !errors.policies"
              :platforms="policies"
              :options="options"
              :loading="loading.policies"
              :can-update="can('message:sms:rate-limit:update')"
              @refresh="load('policies')"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </template>
  </AppPage>
</template>

<style scoped lang="scss">
.sms-page {
  min-width: 0;

  &__catalog-error {
    margin-bottom: 12px;
  }
}

.sms-tabs {
  :deep(.el-tabs__content) {
    overflow: visible;
    padding-top: 12px;
  }

  :deep(.el-tabs__header) {
    margin: 0;
  }

  :deep(.el-tabs__nav-wrap::after) {
    height: 1px;
    background: var(--el-border-color-lighter);
  }

  :deep(.el-tabs__item) {
    height: 40px;
    padding: 0 20px;
    font-weight: 500;
  }
}

.sms-panel {
  min-height: 240px;

  &__error {
    display: flex;
    min-height: 180px;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    gap: 12px;
    color: var(--el-color-danger);
    text-align: center;
  }
}

@media (max-width: 640px) {
  .sms-tabs {
    :deep(.el-tabs__item) {
      padding: 0 14px;
    }
  }
}
</style>
