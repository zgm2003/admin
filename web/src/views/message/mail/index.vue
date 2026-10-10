<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as mailApi from '@/api/message/mail'
import {
  useMessageAggregateTabs,
  type MessageTabLoadContext,
} from '@/composables/useMessageAggregateTabs'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'
import type { MailLogFilter } from './log/index.vue'
import MailConfigTab from './config/index.vue'
import MailLogTab from './log/index.vue'
import MailRateLimitTab from './rateLimitPolicy/index.vue'
import MailRuleTab from './recipientRule/index.vue'
import MailTemplateTab from './template/index.vue'

type TabName = 'config' | 'templates' | 'logs' | 'rules' | 'rateLimits'

const access = usePermissionStore()
const { t, locale } = useI18n()
const activeTab = ref<TabName>('config')
const config = ref<mailApi.MailConfig>({
  configured: false,
  region: '',
  endpoint: '',
  fromEmail: '',
  fromName: '',
  replyTo: '',
  ttlMinutes: 0,
  isEnabled: YesNo.No,
  lastTestAt: null,
  lastTestError: '',
})
const templates = ref<mailApi.MailTemplate[]>([])
const logs = ref<mailApi.MailLog[]>([])
const rules = ref<mailApi.MailRule[]>([])
const rateLimitPolicies = ref<mailApi.MailRateLimitPolicy[]>([])
const logPage = ref(1)
const logPageSize = ref(20)
const logTotal = ref(0)
const logFilter = ref<MailLogFilter>({
  platform: '',
  toEmail: '',
  scene: '',
  status: '',
  timeRange: [],
})
const options = ref<mailApi.MailOptions | null>(null)
const optionsLoading = ref(false)
const optionsError = ref('')
let optionsSequence = 0
async function loadOptions(): Promise<void> {
  const sequence = ++optionsSequence
  options.value = null
  optionsLoading.value = true
  optionsError.value = ''
  try {
    const result = await mailApi.getMailOptions()
    if (sequence === optionsSequence) options.value = result
  } catch (error: unknown) {
    if (sequence === optionsSequence) optionsError.value = errorMessage(error)
  } finally {
    if (sequence === optionsSequence) optionsLoading.value = false
  }
}
watch(locale, () => void loadOptions(), { immediate: true, flush: 'sync' })
onBeforeUnmount(() => {
  optionsSequence++
})
const can = (code: string) => access.hasPermission(code)
const canList = computed(() => can('message:mail:list'))
const visibleTabs = computed(() => [
  ...(canList.value ? [{ name: 'config' as const, label: t('mail.configTab') }] : []),
  ...(canList.value ? [{ name: 'templates' as const, label: t('mail.templatesTab') }] : []),
  ...(canList.value && can('message:mail:detail')
    ? [{ name: 'logs' as const, label: t('mail.logsTab') }]
    : []),
  ...(canList.value || can('message:mail:rule:import') || can('message:mail:rule:export')
    ? [{ name: 'rules' as const, label: t('mail.rulesTab') }]
    : []),
  ...(canList.value ? [{ name: 'rateLimits' as const, label: t('mail.rateLimitsTab') }] : []),
])

watch(
  visibleTabs,
  (tabs) => {
    if (!tabs.some((tab) => tab.name === activeTab.value) && tabs[0]) {
      activeTab.value = tabs[0].name
    }
  },
  { immediate: true },
)

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message ? error.message : t('mail.loadFailed')
}

async function loadConfig(context?: MessageTabLoadContext): Promise<void> {
  const result = await mailApi.getMailConfig()
  if (context === undefined || context.isCurrent()) config.value = result
}

async function loadTemplates(context?: MessageTabLoadContext): Promise<void> {
  const result = await mailApi.listMailTemplates()
  if (context === undefined || context.isCurrent()) templates.value = result
}

async function loadRules(context?: MessageTabLoadContext): Promise<void> {
  const result = await mailApi.listMailRules()
  if (context === undefined || context.isCurrent()) rules.value = result
}

async function loadRateLimitPolicies(context?: MessageTabLoadContext): Promise<void> {
  const result = await mailApi.listMailRateLimitPolicies()
  const next = result.platforms.flatMap((platform) =>
    platform.policies.map((policy) => ({
      ...policy,
      platformId: platform.platformId,
      platformCode: platform.platformCode,
      platformName: platform.platformName,
      rowId: `${platform.platformId}:${policy.key}`,
    })),
  )
  if (context === undefined || context.isCurrent()) rateLimitPolicies.value = next
}

async function loadLogs(context: MessageTabLoadContext): Promise<void> {
  const filter = logFilter.value
  const [from, to] = filter.timeRange
  const result = await mailApi.listMailLogs({
    page: logPage.value,
    pageSize: logPageSize.value,
    ...(filter.platform.trim() === '' ? {} : { platform: filter.platform.trim() }),
    ...(filter.toEmail.trim() === '' ? {} : { toEmail: filter.toEmail.trim() }),
    ...(filter.scene === '' ? {} : { scene: filter.scene }),
    ...(filter.status === '' ? {} : { status: filter.status }),
    ...(filter.timeRange.length === 0 ? {} : { from, to }),
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
    rateLimits: loadRateLimitPolicies,
  },
  errorMessage,
})

function changeLogPage(next: { currentPage: number; pageSize: number }): void {
  logPage.value = next.currentPage
  logPageSize.value = next.pageSize
  void load('logs')
}

function searchLogs(filter: MailLogFilter): void {
  logFilter.value = filter
  logPage.value = 1
  void load('logs')
}
</script>

<template>
  <AppPage class="mail-page">
    <el-alert v-if="optionsError" :title="optionsError" type="error" :closable="false" show-icon />
    <el-tabs v-model="activeTab" class="mail-tabs">
      <el-tab-pane
        v-for="tab in visibleTabs"
        :key="tab.name"
        :name="tab.name"
        :label="tab.label"
        lazy
      >
        <el-alert
          v-if="errors[tab.name]"
          class="mail-error"
          :title="errors[tab.name]"
          type="error"
          show-icon
          :closable="false"
        />
        <MailConfigTab
          v-if="tab.name === 'config'"
          :config="config"
          :scene-options="options?.scenes ?? []"
          :catalog-ready="options !== null && !optionsLoading"
          :can-update="can('message:mail:config:update')"
          :can-test="can('message:mail:test')"
          :can-delete="can('message:mail:config:delete')"
          @saved="load('config')"
          @deleted="load('config')"
          @tested="load('config')"
        />
        <MailTemplateTab
          v-else-if="tab.name === 'templates'"
          :templates="templates"
          :loading="loading.templates"
          :can-update="can('message:mail:template:update')"
          :can-status="can('message:mail:template:status')"
          @refresh="load('templates')"
        />
        <MailLogTab
          v-else-if="tab.name === 'logs'"
          :logs="logs"
          :options="options"
          :total="logTotal"
          :page="logPage"
          :page-size="logPageSize"
          :loading="loading.logs"
          @refresh="() => load('logs')"
          @page-change="changeLogPage"
          @search="searchLogs"
        />
        <MailRuleTab
          v-else-if="tab.name === 'rules'"
          :rules="rules"
          :options="options"
          :loading="loading.rules"
          :can-list="canList"
          :can-create="can('message:mail:rule:create')"
          :can-update="can('message:mail:rule:update')"
          :can-status="can('message:mail:rule:status')"
          :can-delete="can('message:mail:rule:delete')"
          :can-import="can('message:mail:rule:import')"
          :can-export="can('message:mail:rule:export')"
          @refresh="load('rules')"
        />
        <MailRateLimitTab
          v-else
          :policies="rateLimitPolicies"
          :options="options"
          :loading="loading.rateLimits"
          :can-update="can('message:mail:rate-limit:update')"
          @refresh="load('rateLimits')"
        />
      </el-tab-pane>
    </el-tabs>
  </AppPage>
</template>

<style scoped lang="scss">
.mail-page {
  min-width: 0;
}

.mail-tabs {
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

  :deep(.el-tabs__content) {
    overflow: visible;
    padding-top: 12px;
  }
}

.mail-error {
  margin-top: 16px;
}

@media (max-width: 640px) {
  .mail-tabs {
    :deep(.el-tabs__item) {
      padding: 0 14px;
    }
  }
}
</style>
