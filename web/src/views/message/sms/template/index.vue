<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'

import * as smsApi from '@/api/message/sms'
import { AppDialog } from '@/components/AppDialog'
import { AppTable, type TableColumn } from '@/components/AppTable'
import { YesNo, type YesNo as YesNoValue } from '@/enums/yesNo'

const props = defineProps<{
  templates: smsApi.SmsTemplate[]
  loading: boolean
  canUpdate: boolean
  canStatus: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const dialogVisible = ref(false)
const saving = ref(false)
const selectedID = ref<number | null>(null)
const form = ref<smsApi.SmsTemplateInput>(blankTemplate())

const columns = computed<TableColumn<smsApi.SmsTemplate>[]>(() => [
  { key: 'name', prop: 'name', label: t('sms.name'), minWidth: 180 },
  { prop: 'tencentTemplateId', label: t('sms.tencentTemplateId'), minWidth: 150 },
  { key: 'parameters', prop: 'id', label: t('sms.parameters'), minWidth: 170 },
  { key: 'status', prop: 'id', label: t('sms.enabled'), width: 100 },
  {
    key: 'actions',
    prop: 'id',
    label: t('sms.actions'),
    width: 100,
    fixed: 'right',
    hidden: !props.canUpdate,
  },
])

function blankTemplate(): smsApi.SmsTemplateInput {
  return {
    scene: 'login',
    name: '',
    tencentTemplateId: '',
    parameterKeys: ['code', 'ttl_minutes'],
    exampleVariables: { code: '123456', ttl_minutes: '5' },
  }
}

function edit(row: smsApi.SmsTemplate): void {
  selectedID.value = row.id
  form.value = {
    scene: row.scene,
    name: row.name,
    tencentTemplateId: row.tencentTemplateId,
    parameterKeys: [...row.parameterKeys],
    exampleVariables: { ...row.exampleVariables },
  }
  dialogVisible.value = true
}

async function save(): Promise<void> {
  if (selectedID.value === null || form.value.name.trim() === '') return
  saving.value = true
  try {
    await smsApi.updateSmsTemplate(selectedID.value, {
      ...form.value,
      name: form.value.name.trim(),
      tencentTemplateId: form.value.tencentTemplateId.trim(),
      exampleVariables: {
        code: form.value.exampleVariables.code.trim(),
        ttl_minutes: form.value.exampleVariables.ttl_minutes.trim(),
      },
    })
    dialogVisible.value = false
    ElMessage.success(t('sms.saved'))
    emit('refresh')
  } catch {
    // request.ts owns the API error notification.
  } finally {
    saving.value = false
  }
}

async function toggle(row: smsApi.SmsTemplate, value: YesNoValue): Promise<void> {
  try {
    await smsApi.updateSmsTemplateStatus(row.id, value)
    emit('refresh')
  } catch {
    // request.ts owns the API error notification.
  }
}
</script>

<template>
  <div class="sms-template">
    <AppTable
      :columns="columns"
      :data="templates"
      :loading="loading"
      :aria-label="t('sms.tab.templates')"
      :refresh-label="t('sms.refresh')"
      @refresh="emit('refresh')"
    >
      <template #cell-name="{ row }: { row: smsApi.SmsTemplate }">
        <div class="sms-template__name">
          <strong>{{ row.name }}</strong>
          <code>{{ row.scene }}</code>
        </div>
      </template>
      <template #cell-parameters="{ row }: { row: smsApi.SmsTemplate }">
        <el-space wrap>
          <el-tag v-for="key in row.parameterKeys" :key="key" size="small" effect="plain">
            {{ key }}
          </el-tag>
        </el-space>
      </template>
      <template #cell-status="{ row }: { row: smsApi.SmsTemplate }">
        <el-switch
          :model-value="row.isEnabled"
          data-testid="sms-template-status"
          :active-value="YesNo.Yes"
          :inactive-value="YesNo.No"
          :disabled="!canStatus"
          @change="toggle(row, $event as YesNoValue)"
        />
      </template>
      <template #cell-actions="{ row }: { row: smsApi.SmsTemplate }">
        <el-button data-testid="sms-template-edit" text type="primary" @click="edit(row)">
          {{ t('sms.edit') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('sms.noTemplates')" />
      </template>
    </AppTable>

    <AppDialog
      v-model="dialogVisible"
      :title="t('sms.editTemplate')"
      width="min(620px, 94vw)"
      append-to-body
    >
      <el-form :model="form" label-position="top" @submit.prevent="save">
        <el-row :gutter="16">
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('sms.sceneLabel')">
              <el-input v-model="form.scene" data-testid="sms-template-scene" disabled />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('sms.tencentTemplateId')">
              <el-input
                v-model="form.tencentTemplateId"
                data-testid="sms-template-id"
                inputmode="numeric"
                :placeholder="t('sms.templateIdPlaceholder')"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('sms.name')">
              <el-input
                v-model="form.name"
                data-testid="sms-template-name"
                :placeholder="t('sms.templateNamePlaceholder')"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('sms.parameters')">
              <el-input
                :model-value="form.parameterKeys.join(', ')"
                :placeholder="t('sms.parametersPlaceholder')"
                disabled
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('sms.exampleCode')">
              <el-input
                v-model="form.exampleVariables.code"
                data-testid="sms-template-example-code"
                :placeholder="t('sms.exampleCodePlaceholder')"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item :label="t('sms.exampleTTL')">
              <el-input
                v-model="form.exampleVariables.ttl_minutes"
                data-testid="sms-template-example-ttl"
                :placeholder="t('sms.exampleTTLPlaceholder')"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('user.cancel') }}</el-button>
        <el-button data-testid="sms-template-save" type="primary" :loading="saving" @click="save">
          {{ t('sms.save') }}
        </el-button>
      </template>
    </AppDialog>
  </div>
</template>

<style scoped>
.sms-template {
  min-width: 0;
}

.sms-template__name {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.sms-template__name code {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
