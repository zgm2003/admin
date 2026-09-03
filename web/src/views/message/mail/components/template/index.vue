<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Pencil } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

import {
  updateMailTemplate,
  updateMailTemplateStatus,
  type MailTemplate,
  type MailTemplateInput,
} from '@/api/message/mail'
import { AppTable, type TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yes-no'

const props = defineProps<{
  templates: MailTemplate[]
  loading: boolean
  canUpdate: boolean
  canStatus: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const selected = ref<MailTemplate | null>(null)
const dialog = ref(false)
const saving = ref(false)
const form = ref<MailTemplateInput>(blankTemplate())
const variables = ref('{}')
const examples = ref('{}')
const enabledCount = computed(
  () => props.templates.filter((item) => item.isEnabled === YesNo.Yes).length,
)
const columns = computed<TableColumn<MailTemplate>[]>(() => [
  { key: 'name', prop: 'name', label: t('mail.name'), minWidth: 170 },
  { prop: 'subject', label: t('mail.subject'), minWidth: 190, overflowTooltip: true },
  { key: 'templateId', prop: 'tencentTemplateId', label: t('mail.templateId'), width: 180 },
  { key: 'variables', prop: 'id', label: t('mail.variables'), minWidth: 180 },
  { key: 'status', prop: 'id', label: t('mail.status'), width: 100 },
  {
    key: 'actions',
    prop: 'id',
    label: t('mail.actions'),
    width: 100,
    fixed: 'right',
    hidden: !props.canUpdate,
  },
])

watch(selected, (value) => {
  if (!value) return
  form.value = {
    scene: value.scene,
    name: value.name,
    subject: value.subject,
    tencentTemplateId: value.tencentTemplateId,
    variables: value.variables,
    exampleVariables: value.exampleVariables,
  }
  variables.value = JSON.stringify(value.variables, null, 2)
  examples.value = JSON.stringify(value.exampleVariables, null, 2)
})

function blankTemplate(): MailTemplateInput {
  return {
    scene: '',
    name: '',
    subject: '',
    tencentTemplateId: 0,
    variables: {},
    exampleVariables: {},
  }
}

function edit(row: MailTemplate): void {
  selected.value = row
  dialog.value = true
}

async function toggle(row: MailTemplate): Promise<void> {
  await updateMailTemplateStatus(row.id, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
  emit('refresh')
}

function parseMap(value: string): Record<string, string> {
  const data: unknown = JSON.parse(value)
  if (
    typeof data !== 'object' ||
    data === null ||
    Array.isArray(data) ||
    !Object.values(data).every((item) => typeof item === 'string')
  ) {
    throw new Error(t('mail.variablesInvalid'))
  }
  return data as Record<string, string>
}

async function saveTemplate(): Promise<void> {
  if (!selected.value) return
  saving.value = true
  try {
    await updateMailTemplate(selected.value.id, {
      ...form.value,
      variables: parseMap(variables.value),
      exampleVariables: parseMap(examples.value),
    })
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
      :data="templates"
      :loading="loading"
      result-state="success"
      :aria-label="t('mail.templatesTab')"
      :refresh-label="t('mail.refresh')"
      @refresh="emit('refresh')"
    >
      <template #toolbar-left>
        <div class="table-summary">
          <strong>{{ t('mail.templateSummary', { count: templates.length }) }}</strong>
          <span>{{ t('mail.templateEnabled', { count: enabledCount }) }}</span>
        </div>
      </template>
      <template #cell-name="{ row }: { row: MailTemplate }">
        <div class="primary-cell">
          <strong>{{ row.name }}</strong
          ><span>{{ row.scene }}</span>
        </div>
      </template>
      <template #cell-templateId="{ row }: { row: MailTemplate }"
        ><code>{{ row.tencentTemplateId }}</code></template
      >
      <template #cell-variables="{ row }: { row: MailTemplate }">
        <el-space wrap>
          <el-tag v-for="(_, key) in row.variables" :key="key" size="small" effect="plain">{{
            key
          }}</el-tag>
        </el-space>
      </template>
      <template #cell-status="{ row }: { row: MailTemplate }">
        <el-switch
          :model-value="row.isEnabled"
          :active-value="YesNo.Yes"
          :inactive-value="YesNo.No"
          :disabled="!canStatus"
          @change="toggle(row)"
        />
      </template>
      <template #cell-actions="{ row }: { row: MailTemplate }">
        <el-button data-testid="mail-template-edit" text type="primary" @click="edit(row)">
          <Pencil :size="15" />
          {{ t('mail.edit') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('mail.noTemplates')" />
      </template>
    </AppTable>
    <el-dialog
      v-model="dialog"
      :title="t('mail.editTemplate')"
      width="min(680px, 94vw)"
      destroy-on-close
    >
      <el-form :model="form" label-position="top">
        <el-row :gutter="16">
          <el-col :xs="24" :sm="12"
            ><el-form-item :label="t('mail.scene')"
              ><el-input v-model="form.scene" disabled /></el-form-item
          ></el-col>
          <el-col :xs="24" :sm="12"
            ><el-form-item :label="t('mail.templateId')"
              ><el-input-number v-model="form.tencentTemplateId" disabled /></el-form-item
          ></el-col>
          <el-col :xs="24" :sm="12"
            ><el-form-item :label="t('mail.name')"><el-input v-model="form.name" /></el-form-item
          ></el-col>
          <el-col :xs="24" :sm="12"
            ><el-form-item :label="t('mail.subject')"
              ><el-input v-model="form.subject" /></el-form-item
          ></el-col>
          <el-col :xs="24" :sm="12"
            ><el-form-item :label="t('mail.variables')"
              ><el-input v-model="variables" type="textarea" :rows="6" /></el-form-item
          ></el-col>
          <el-col :xs="24" :sm="12"
            ><el-form-item :label="t('mail.exampleVariables')"
              ><el-input v-model="examples" type="textarea" :rows="6" /></el-form-item
          ></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">{{ t('mail.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="saveTemplate">{{
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

.table-summary {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.table-summary strong {
  font-size: 14px;
}

.table-summary span,
.primary-cell span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.primary-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.primary-cell strong {
  font-weight: 600;
}
</style>
