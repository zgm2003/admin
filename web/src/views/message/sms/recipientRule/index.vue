<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import * as smsApi from '@/api/message/sms'
import { AppDialog } from '@/components/AppDialog'
import { AppTable, type TableColumn } from '@/components/AppTable'
import { YesNo, type YesNo as YesNoValue } from '@/enums/yesNo'

const props = defineProps<{
  rules: smsApi.SmsRule[]
  loading: boolean
  canCreate: boolean
  canUpdate: boolean
  canStatus: boolean
  canDelete: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const dialogVisible = ref(false)
const editingID = ref<number | null>(null)
const saving = ref(false)
const form = ref<smsApi.SmsRuleInput>(blankRule())

const scopeOptions = computed(() => [
  { value: 'phone', label: t('sms.scope.phone') },
  { value: 'prefix', label: t('sms.scope.prefix') },
])
const actionOptions = computed(() => [
  { value: 'allow', label: t('sms.action.allow') },
  { value: 'deny', label: t('sms.action.deny') },
])
const hasRowActions = computed(() => props.canUpdate || props.canStatus || props.canDelete)
const columns = computed<TableColumn<smsApi.SmsRule>[]>(() => [
  { prop: 'patternHint', label: t('sms.rulePattern'), minWidth: 160 },
  { key: 'scope', prop: 'scope', label: t('sms.ruleScope'), width: 120 },
  { key: 'action', prop: 'action', label: t('sms.ruleAction'), width: 100 },
  { prop: 'name', label: t('sms.name'), minWidth: 160 },
  { prop: 'remark', label: t('sms.remark'), minWidth: 160, overflowTooltip: true },
  { key: 'status', prop: 'id', label: t('sms.enabled'), width: 100 },
  {
    key: 'actions',
    prop: 'id',
    label: t('sms.actions'),
    minWidth: 150,
    fixed: 'right',
    hidden: !hasRowActions.value,
  },
])

function blankRule(): smsApi.SmsRuleInput {
  return {
    scope: 'phone',
    pattern: '',
    action: 'deny',
    name: '',
    remark: '',
    isEnabled: YesNo.Yes,
  }
}

function create(): void {
  editingID.value = null
  form.value = blankRule()
  dialogVisible.value = true
}

function edit(row: smsApi.SmsRule): void {
  editingID.value = row.id
  form.value = {
    scope: row.scope,
    pattern: '',
    action: row.action,
    name: row.name,
    remark: row.remark,
    isEnabled: row.isEnabled,
  }
  dialogVisible.value = true
}

async function save(): Promise<void> {
  const pattern = form.value.pattern.trim()
  const name = form.value.name.trim()
  if (name === '' || (editingID.value === null && pattern === '')) return
  saving.value = true
  try {
    if (editingID.value === null) {
      await smsApi.createSmsRule({ ...form.value, pattern, name })
    } else {
      await smsApi.updateSmsRule(editingID.value, {
        scope: form.value.scope,
        action: form.value.action,
        name,
        remark: form.value.remark.trim(),
        isEnabled: form.value.isEnabled,
        ...(pattern === '' ? {} : { pattern }),
      })
    }
    dialogVisible.value = false
    ElMessage.success(t('sms.saved'))
    emit('refresh')
  } catch {
    // request.ts owns the API error notification.
  } finally {
    saving.value = false
  }
}

async function toggle(row: smsApi.SmsRule, value: YesNoValue): Promise<void> {
  try {
    await smsApi.updateSmsRuleStatus(row.id, value)
    emit('refresh')
  } catch {
    // request.ts owns the API error notification.
  }
}

async function remove(row: smsApi.SmsRule): Promise<void> {
  try {
    await ElMessageBox.confirm(t('sms.ruleDeleteConfirm'))
    await smsApi.deleteSmsRule(row.id)
    ElMessage.success(t('sms.ruleDeleted'))
    emit('refresh')
  } catch {
    // Dialog cancellation and request errors are handled by their respective layers.
  }
}
</script>

<template>
  <div class="sms-rule">
    <AppTable
      :columns="columns"
      :data="rules"
      :loading="loading"
      :aria-label="t('sms.tab.rules')"
      :refresh-label="t('sms.refresh')"
      @refresh="emit('refresh')"
    >
      <template #toolbar-left>
        <el-button v-if="canCreate" data-testid="sms-rule-create" type="primary" @click="create">
          <Plus :size="16" />
          {{ t('sms.create') }}
        </el-button>
      </template>
      <template #cell-scope="{ row }: { row: smsApi.SmsRule | undefined }">
        <template v-if="row">{{ t(`sms.scope.${row.scope}`) }}</template>
      </template>
      <template #cell-action="{ row }: { row: smsApi.SmsRule | undefined }">
        <el-tag v-if="row" :type="row.action === 'deny' ? 'danger' : 'success'">
          {{ t(`sms.action.${row.action}`) }}
        </el-tag>
      </template>
      <template #cell-status="{ row }: { row: smsApi.SmsRule }">
        <el-switch
          :model-value="row.isEnabled"
          :data-testid="`sms-rule-status-${row.id}`"
          :active-value="YesNo.Yes"
          :inactive-value="YesNo.No"
          :disabled="!canStatus"
          @change="toggle(row, $event as YesNoValue)"
        />
      </template>
      <template #cell-actions="{ row }: { row: smsApi.SmsRule }">
        <el-button
          v-if="canUpdate"
          :data-testid="`sms-rule-edit-${row.id}`"
          text
          type="primary"
          @click="edit(row)"
        >
          {{ t('sms.edit') }}
        </el-button>
        <el-button
          v-if="canDelete"
          :data-testid="`sms-rule-delete-${row.id}`"
          text
          type="danger"
          @click="remove(row)"
        >
          {{ t('sms.delete') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('sms.noRules')" />
      </template>
    </AppTable>

    <AppDialog
      v-model="dialogVisible"
      :title="t(editingID === null ? 'sms.createRule' : 'sms.editRule')"
      width="min(560px, 94vw)"
      append-to-body
    >
      <el-form :model="form" label-position="top" @submit.prevent="save">
        <el-row :gutter="16">
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('sms.ruleScope')">
              <el-select-v2
                v-model="form.scope"
                data-testid="sms-rule-scope"
                :options="scopeOptions"
                :placeholder="t('sms.ruleScopePlaceholder')"
                class="sms-rule__full"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('sms.ruleAction')">
              <el-select-v2
                v-model="form.action"
                data-testid="sms-rule-action"
                :options="actionOptions"
                :placeholder="t('sms.ruleActionPlaceholder')"
                class="sms-rule__full"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item :label="t('sms.rulePattern')">
          <el-input
            v-model="form.pattern"
            data-testid="sms-rule-pattern"
            :placeholder="
              editingID === null
                ? t('sms.rulePatternPlaceholder')
                : t('sms.rulePatternKeepPlaceholder')
            "
          />
        </el-form-item>
        <el-form-item :label="t('sms.name')">
          <el-input
            v-model="form.name"
            data-testid="sms-rule-name"
            :placeholder="t('sms.namePlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('sms.remark')">
          <el-input
            v-model="form.remark"
            data-testid="sms-rule-remark"
            type="textarea"
            :rows="3"
            :placeholder="t('sms.remarkPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('sms.enabled')">
          <el-switch
            v-model="form.isEnabled"
            :active-value="YesNo.Yes"
            :inactive-value="YesNo.No"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('user.cancel') }}</el-button>
        <el-button data-testid="sms-rule-save" type="primary" :loading="saving" @click="save">
          {{ t('sms.save') }}
        </el-button>
      </template>
    </AppDialog>
  </div>
</template>

<style scoped>
.sms-rule,
.sms-rule__full {
  min-width: 0;
  width: 100%;
}
</style>
