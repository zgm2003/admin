<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { useI18n } from 'vue-i18n'

import {
  createMailRule,
  deleteMailRule,
  exportMailRuleXlsx,
  mailRuleXlsxMime,
  updateMailRule,
  updateMailRuleStatus,
  type MailRule,
  type MailRuleInput,
} from '@/api/message/mail'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import { MailRuleAction, MailRuleScope } from '@/enums/mailRecipientRule'
import MailRuleImportDialog from './components/MailRuleImportDialog/index.vue'

const props = defineProps<{
  rules: MailRule[]
  loading: boolean
  canList: boolean
  canCreate: boolean
  canUpdate: boolean
  canStatus: boolean
  canDelete: boolean
  canImport: boolean
  canExport: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const dialog = ref(false)
const editing = ref<MailRule | null>(null)
const saving = ref(false)
const importing = ref(false)
const exporting = ref(false)
const exportFailed = ref(false)
let exportSequence = 0
let exportController: AbortController | null = null
onBeforeUnmount(() => {
  exportSequence++
  exportController?.abort()
})
watch(
  () => props.canExport,
  () => {
    exportSequence++
    exportController?.abort()
    exporting.value = false
    exportFailed.value = false
  },
  { flush: 'sync' },
)
const form = ref<MailRuleInput>(blankRule())
const scopeOptions = computed<Array<{ value: MailRuleInput['scope']; label: string }>>(() => [
  { value: MailRuleScope.Email, label: t('mail.email') },
  { value: MailRuleScope.Domain, label: t('mail.domain') },
])
const actionOptions = computed<Array<{ value: MailRuleInput['action']; label: string }>>(() => [
  { value: MailRuleAction.Allow, label: t('mail.allow') },
  { value: MailRuleAction.Deny, label: t('mail.deny') },
])
const columns = computed<TableColumn<MailRule>[]>(() => [
  { key: 'pattern', prop: 'pattern', label: t('mail.rule'), minWidth: 220 },
  { prop: 'action', label: t('mail.action'), width: 120 },
  { prop: 'name', label: t('mail.name'), minWidth: 160 },
  { prop: 'remark', label: t('mail.remark'), minWidth: 200, overflowTooltip: true },
  { key: 'enabled', prop: 'isEnabled', label: t('mail.enabled'), width: 100 },
  { key: 'actions', prop: 'id', label: t('mail.actions'), width: 200, fixed: 'right' },
])

watch(editing, (value) => {
  form.value = value
    ? {
        scope: value.scope,
        pattern: value.pattern,
        action: value.action,
        name: value.name,
        remark: value.remark,
        isEnabled: value.isEnabled,
      }
    : blankRule()
})

function blankRule(): MailRuleInput {
  return {
    scope: MailRuleScope.Email,
    pattern: '',
    action: MailRuleAction.Deny,
    name: '',
    remark: '',
    isEnabled: YesNo.Yes,
  }
}

function create(): void {
  editing.value = null
  dialog.value = true
}

function edit(row: MailRule): void {
  editing.value = row
  dialog.value = true
}

async function toggle(row: MailRule): Promise<void> {
  try {
    await updateMailRuleStatus(row.id, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
    emit('refresh')
  } catch {
    // request.ts owns API error notifications.
  }
}

async function remove(row: MailRule): Promise<void> {
  try {
    await ElMessageBox.confirm(t('mail.deleteRuleConfirm'))
    await deleteMailRule(row.id)
    ElMessage.success(t('mail.deleted'))
    emit('refresh')
  } catch {
    // ElMessageBox cancellation and request errors are handled by their respective layers.
  }
}

async function saveRule(): Promise<void> {
  saving.value = true
  try {
    if (editing.value) await updateMailRule(editing.value.id, form.value)
    else await createMailRule(form.value)
    ElMessage.success(t('mail.updated'))
    dialog.value = false
    emit('refresh')
  } finally {
    saving.value = false
  }
}

async function exportRules(): Promise<void> {
  if (!props.canExport || exporting.value) return
  const sequence = ++exportSequence
  const controller = new AbortController()
  exportController = controller
  exporting.value = true
  exportFailed.value = false
  try {
    const file = await exportMailRuleXlsx(controller.signal)
    if (sequence !== exportSequence || !props.canExport) return
    const url = URL.createObjectURL(new Blob([file.content], { type: mailRuleXlsxMime }))
    const link = document.createElement('a')
    try {
      link.href = url
      link.download = file.fileName
      document.body.appendChild(link)
      link.click()
      ElMessage.success(t('mail.ruleXlsx.exported'))
    } finally {
      link.remove()
      URL.revokeObjectURL(url)
    }
  } catch {
    if (sequence === exportSequence && props.canExport) exportFailed.value = true
  } finally {
    if (sequence === exportSequence) exporting.value = false
  }
}
</script>

<template>
  <div class="table-tab">
    <el-alert
      v-if="exportFailed"
      :title="t('mail.ruleXlsx.exportFailed')"
      type="error"
      :closable="false"
      show-icon
    />
    <el-alert
      class="rule-hint"
      :title="t('mail.ruleHint')"
      type="info"
      show-icon
      :closable="false"
    />
    <AppTable
      v-if="canList"
      :columns="columns"
      :data="rules"
      :loading="loading"
      :aria-label="t('mail.rulesTab')"
      :refresh-label="t('mail.refresh')"
      @refresh="emit('refresh')"
    >
      <template #toolbar-left>
        <el-button v-if="canCreate" data-testid="mail-rule-create" type="primary" @click="create">
          {{ t('mail.createRule') }}
        </el-button>
        <el-button v-if="canImport" data-testid="mail-rule-import" @click="importing = true">{{
          t('mail.ruleXlsx.import')
        }}</el-button>
        <el-button
          v-if="canExport"
          data-testid="mail-rule-export"
          :loading="exporting"
          @click="exportRules"
          >{{ t('mail.ruleXlsx.export') }}</el-button
        >
      </template>
      <template #cell-pattern="{ row }: { row: MailRule }">
        <div class="primary-cell">
          <strong>{{ row.pattern }}</strong>
          <span>{{ row.scope === MailRuleScope.Email ? t('mail.email') : t('mail.domain') }}</span>
        </div>
      </template>
      <template #cell-action="{ row }: { row: MailRule }">
        <el-tag :type="row.action === MailRuleAction.Allow ? 'success' : 'danger'" effect="plain">
          {{ row.action === MailRuleAction.Allow ? t('mail.allow') : t('mail.deny') }}
        </el-tag>
      </template>
      <template #cell-enabled="{ row }: { row: MailRule }">
        <el-switch
          :model-value="row.isEnabled"
          :active-value="YesNo.Yes"
          :inactive-value="YesNo.No"
          :disabled="!canStatus"
          @change="toggle(row)"
        />
      </template>
      <template #cell-actions="{ row }: { row: MailRule }">
        <el-button v-if="canUpdate" text type="primary" @click="edit(row)">
          {{ t('mail.edit') }}
        </el-button>
        <el-button v-if="canDelete" text type="danger" @click="remove(row)">
          {{ t('mail.delete') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('mail.noRules')" />
      </template>
    </AppTable>

    <div v-else>
      <el-alert
        class="rule-hint"
        :title="t('mail.ruleXlsx.noReadAccess')"
        type="info"
        show-icon
        :closable="false"
      />
      <div class="rule-xlsx-actions">
        <el-button v-if="canImport" data-testid="mail-rule-import" @click="importing = true">{{
          t('mail.ruleXlsx.import')
        }}</el-button>
        <el-button
          v-if="canExport"
          data-testid="mail-rule-export"
          :loading="exporting"
          @click="exportRules"
          >{{ t('mail.ruleXlsx.export') }}</el-button
        >
      </div>
    </div>

    <MailRuleImportDialog v-model="importing" :can-import="canImport" @imported="emit('refresh')" />

    <el-dialog
      v-model="dialog"
      :title="editing ? t('mail.editRule') : t('mail.createRule')"
      width="min(560px, 94vw)"
      destroy-on-close
    >
      <el-form :model="form" label-position="top">
        <el-row :gutter="16">
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('mail.ruleType')">
              <el-select-v2
                v-model="form.scope"
                :options="scopeOptions"
                :clearable="false"
                data-testid="mail-rule-scope"
                class="mail-rule-select"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('mail.action')">
              <el-select-v2
                v-model="form.action"
                :options="actionOptions"
                :clearable="false"
                data-testid="mail-rule-action"
                class="mail-rule-select"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item :label="t('mail.rule')">
          <el-input
            v-model="form.pattern"
            :placeholder="
              form.scope === MailRuleScope.Email
                ? t('mail.rulePatternEmailPlaceholder')
                : t('mail.rulePatternDomainPlaceholder')
            "
            data-testid="mail-rule-pattern"
          />
        </el-form-item>
        <el-form-item :label="t('mail.name')">
          <el-input v-model="form.name" :placeholder="t('mail.ruleNamePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('mail.remark')">
          <el-input
            v-model="form.remark"
            type="textarea"
            :rows="3"
            :placeholder="t('mail.ruleRemarkPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('mail.enabled')">
          <el-switch
            v-model="form.isEnabled"
            :active-value="YesNo.Yes"
            :inactive-value="YesNo.No"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">{{ t('mail.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="saveRule">{{
          t('mail.save')
        }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.table-tab {
  min-width: 0;
}

.primary-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.primary-cell strong {
  font-size: 14px;
  font-weight: 600;
}

.primary-cell span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.mail-table {
  border-top: 1px solid var(--el-border-color-lighter);
}

.rule-hint {
  margin-bottom: 12px;
}

.mail-rule-select {
  width: 100%;
}
</style>
