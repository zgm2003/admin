<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import {
  createMailRule,
  deleteMailRule,
  updateMailRule,
  updateMailRuleStatus,
  type MailRule,
  type MailRuleInput,
} from '@/api/system/mail'
import { AppTable, type TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yes-no'

const props = defineProps<{
  rules: MailRule[]
  loading: boolean
  canCreate: boolean
  canUpdate: boolean
  canStatus: boolean
  canDelete: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const dialog = ref(false)
const editing = ref<MailRule | null>(null)
const saving = ref(false)
const form = ref<MailRuleInput>(blankRule())
const allowCount = computed(
  () =>
    props.rules.filter((item) => item.action === 'allow' && item.isEnabled === YesNo.Yes).length,
)
const columns = computed<TableColumn<MailRule>[]>(() => [
  { key: 'pattern', prop: 'pattern', label: t('mail.rule'), minWidth: 220 },
  { prop: 'action', label: t('mail.action'), width: 120 },
  { prop: 'name', label: t('mail.name'), minWidth: 160 },
  { prop: 'remark', label: t('mail.remark'), minWidth: 200, overflowTooltip: true },
  { key: 'enabled', prop: 'isEnabled', label: t('mail.enabled'), width: 100 },
  { key: 'actions', prop: 'id', label: t('mail.actions'), width: 150, fixed: 'right' },
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
  return { scope: 'email', pattern: '', action: 'deny', name: '', remark: '', isEnabled: YesNo.Yes }
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
  await updateMailRuleStatus(row.id, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
  emit('refresh')
}

async function remove(row: MailRule): Promise<void> {
  await ElMessageBox.confirm(t('mail.deleteRuleConfirm'))
  await deleteMailRule(row.id)
  ElMessage.success(t('mail.deleted'))
  emit('refresh')
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
</script>

<template>
  <div class="table-tab">
    <AppTable
      :columns="columns"
      :data="rules"
      :loading="loading"
      result-state="success"
      :aria-label="t('mail.rulesTab')"
      :refresh-label="t('mail.refresh')"
      @refresh="emit('refresh')"
    >
      <template #toolbar-left>
        <div class="table-summary">
          <strong>{{ t('mail.ruleSummary', { count: rules.length }) }}</strong>
          <span>{{ t('mail.ruleAllowCount', { count: allowCount }) }}</span>
        </div>
        <el-button v-if="canCreate" data-testid="mail-rule-create" type="primary" @click="create">
          <Plus :size="16" />
          {{ t('mail.createRule') }}
        </el-button>
      </template>
      <template #cell-pattern="{ row }: { row: MailRule }">
        <div class="primary-cell">
          <strong>{{ row.pattern }}</strong>
          <span>{{ row.scope === 'email' ? t('mail.email') : t('mail.domain') }}</span>
        </div>
      </template>
      <template #cell-action="{ row }: { row: MailRule }">
        <el-tag :type="row.action === 'allow' ? 'success' : 'danger'" effect="plain">
          {{ row.action === 'allow' ? t('mail.allow') : t('mail.deny') }}
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
          <Pencil :size="15" />
          {{ t('mail.edit') }}
        </el-button>
        <el-button v-if="canDelete" text type="danger" @click="remove(row)">
          <Trash2 :size="15" />
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('mail.noRules')" />
      </template>
    </AppTable>

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
              <el-select v-model="form.scope">
                <el-option value="email" :label="t('mail.email')" />
                <el-option value="domain" :label="t('mail.domain')" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('mail.action')">
              <el-select v-model="form.action">
                <el-option value="allow" :label="t('mail.allow')" />
                <el-option value="deny" :label="t('mail.deny')" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item :label="t('mail.rule')"><el-input v-model="form.pattern" /></el-form-item>
        <el-form-item :label="t('mail.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('mail.remark')">
          <el-input v-model="form.remark" type="textarea" :rows="3" />
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

.table-summary,
.primary-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.table-summary strong,
.primary-cell strong {
  font-size: 14px;
  font-weight: 600;
}

.table-summary span,
.primary-cell span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.mail-table {
  border-top: 1px solid var(--el-border-color-lighter);
}
</style>
