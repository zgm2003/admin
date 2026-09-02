<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import * as mailApi from '../../../api/system/mail'
import { YesNo } from '../../../enums/yes-no'
import { usePermissionStore } from '../../../store/permission'
import MailConfigTab from './components/config/index.vue'
import MailLogTab from './components/log/index.vue'
import MailRuleTab from './components/rule/index.vue'
import MailTemplateTab from './components/template/index.vue'

type TabName = 'config' | 'templates' | 'logs' | 'rules'

const access = usePermissionStore()
const { t } = useI18n()
const activeTab = ref<TabName>('config')
const loading = ref(false)
const loadError = ref('')
const config = ref<mailApi.MailConfig>({
  configured: false,
  region: '',
  endpoint: '',
  fromEmail: '',
  fromName: '',
  replyTo: '',
  ttlMinutes: 10,
  isEnabled: YesNo.No,
  lastTestAt: null,
  lastTestError: '',
})
const templates = ref<mailApi.MailTemplate[]>([])
const logs = ref<mailApi.MailLog[]>([])
const rules = ref<mailApi.MailRule[]>([])
const logPage = ref(1)
const logPageSize = ref(20)
const logTotal = ref(0)
const can = (code: string) => access.hasPermission(code)
const visibleTabs = computed(() => [
  { name: 'config' as const, label: t('mail.configTab') },
  { name: 'templates' as const, label: t('mail.templatesTab') },
  ...(can('system:mail:detail') ? [{ name: 'logs' as const, label: t('mail.logsTab') }] : []),
  { name: 'rules' as const, label: t('mail.rulesTab') },
])

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message ? error.message : t('mail.loadFailed')
}

async function loadConfig(): Promise<void> {
  config.value = await mailApi.getMailConfig()
}

async function loadTemplates(): Promise<void> {
  templates.value = await mailApi.listMailTemplates()
}

async function loadRules(): Promise<void> {
  rules.value = await mailApi.listMailRules()
}

async function loadLogs(): Promise<void> {
  const result = await mailApi.listMailLogs({
    page: logPage.value,
    pageSize: logPageSize.value,
  })
  logs.value = result.list
  logTotal.value = result.total
}

async function loadActive(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    if (activeTab.value === 'config') await loadConfig()
    else if (activeTab.value === 'templates') await loadTemplates()
    else if (activeTab.value === 'logs') await loadLogs()
    else await loadRules()
  } catch (error: unknown) {
    loadError.value = errorMessage(error)
  } finally {
    loading.value = false
  }
}

function changeLogPage(page: number): void {
  logPage.value = page
  void loadLogs()
}

watch(activeTab, () => {
  void loadActive()
}, { immediate: true })
</script>

<template>
  <section class="mail-page">
    <el-tabs v-model="activeTab" class="mail-tabs">
      <el-tab-pane v-for="tab in visibleTabs" :key="tab.name" :name="tab.name" :label="tab.label" lazy>
        <el-alert v-if="loadError" class="mail-error" :title="loadError" type="error" show-icon :closable="false" />
        <el-card shadow="never" class="mail-panel">
          <MailConfigTab
            v-if="tab.name === 'config'"
            :config="config"
            :can-update="can('system:mail:config:update')"
            :can-test="can('system:mail:test')"
            :can-delete="can('system:mail:config:delete')"
            @saved="loadConfig"
            @deleted="loadConfig"
          />
          <MailTemplateTab
            v-else-if="tab.name === 'templates'"
            :templates="templates"
            :loading="loading"
            :can-update="can('system:mail:template:update')"
            :can-status="can('system:mail:template:status')"
            @refresh="loadTemplates"
          />
          <MailLogTab
            v-else-if="tab.name === 'logs'"
            :logs="logs"
            :total="logTotal"
            :page="logPage"
            :page-size="logPageSize"
            :loading="loading"
            :can-delete="can('system:mail:log:delete')"
            @refresh="loadLogs"
            @page-change="changeLogPage"
          />
          <MailRuleTab
            v-else
            :rules="rules"
            :loading="loading"
            :can-create="can('system:mail:rule:create')"
            :can-update="can('system:mail:rule:update')"
            :can-status="can('system:mail:rule:status')"
            :can-delete="can('system:mail:rule:delete')"
            @refresh="loadRules"
          />
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </section>
</template>

<style scoped lang="scss">
.mail-page {
  min-width: 0;
  padding: 0 8px 20px;
}

.mail-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.mail-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--el-border-color-lighter);
}

.mail-tabs :deep(.el-tabs__item) {
  height: 46px;
  padding: 0 24px;
  font-weight: 500;
}

.mail-tabs :deep(.el-tabs__content) {
  overflow: visible;
  padding-top: 16px;
}

.mail-panel {
  border-color: var(--el-border-color-light);
}

.mail-panel :deep(.el-card__body) {
  padding: 18px 20px;
}

.mail-error {
  margin-top: 16px;
}

@media (max-width: 640px) {
  .mail-page {
    padding-inline: 0;
  }

  .mail-tabs :deep(.el-tabs__item) {
    padding: 0 14px;
  }

  .mail-panel :deep(.el-card__body) {
    padding: 14px;
  }
}
</style>
