<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useI18n } from 'vue-i18n'

import {
  updateMailTemplate,
  updateMailTemplateStatus,
  type MailTemplate,
  type MailTemplateInput,
} from '@/api/message/mail'
import type { TableColumn } from '@/components/AppTable'
import { YesNo } from '@/enums/yesNo'
import {
  assertSafeMailHtml,
  replaceMailVariables,
} from './components/MailHtmlEditor/mailHtmlDocument'

const MailHtmlEditor = defineAsyncComponent(
  () => import('@/views/message/mail/template/components/MailHtmlEditor/index.vue'),
)

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
const editorVisible = ref(false)
const form = ref<MailTemplateInput>(blankTemplate())
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
    content: value.content,
    variableKeys: [...value.variableKeys],
    exampleVariables: value.exampleVariables,
  }
})

watch(
  () => form.value.variableKeys,
  (keys) => {
    const next: Record<string, string> = {}
    for (const key of keys) next[key] = form.value.exampleVariables[key] ?? ''
    form.value.exampleVariables = next
  },
  { deep: true },
)

function blankTemplate(): MailTemplateInput {
  return {
    scene: '',
    name: '',
    subject: '',
    tencentTemplateId: null,
    content:
      '<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>{{code}} {{ttl_minutes}}</body></html>',
    variableKeys: ['code', 'ttl_minutes'],
    exampleVariables: {},
  }
}

function edit(row: MailTemplate): void {
  selected.value = row
  dialog.value = true
  editorVisible.value = true
}

const previewHtml = computed(() =>
  replaceMailVariables(form.value.content, form.value.exampleVariables),
)

async function copyHtml(): Promise<void> {
  try {
    await navigator.clipboard.writeText(form.value.content)
    ElMessage.success(t('mail.copied'))
  } catch {
    ElMessage.error(t('mail.copyFailed'))
  }
}

async function toggle(row: MailTemplate): Promise<void> {
  try {
    await updateMailTemplateStatus(row.id, row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes)
    emit('refresh')
  } catch {
    // request.ts owns API error notifications.
  }
}

async function saveTemplate(): Promise<void> {
  if (!selected.value) return
  saving.value = true
  try {
    assertSafeMailHtml(form.value.content)
    await updateMailTemplate(selected.value.id, {
      ...form.value,
      variableKeys: [...form.value.variableKeys],
      exampleVariables: { ...form.value.exampleVariables },
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
          <el-tag v-for="key in row.variableKeys" :key="key" size="small" effect="plain">{{
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
          {{ t('mail.edit') }}
        </el-button>
      </template>
      <template #empty>
        <el-empty :description="t('mail.noTemplates')" />
      </template>
    </AppTable>
    <AppDialog
      v-model="dialog"
      :title="t('mail.editTemplate')"
      width="min(1080px, 96vw)"
      body-padding="0"
      class="mail-template-dialog"
      height="700px"
    >
      <el-form class="mail-template-form" :model="form" label-position="top">
        <section class="mail-template-form__meta">
          <el-row :gutter="16">
            <el-col :xs="24" :sm="8">
              <el-form-item :label="t('mail.scene')">
                <el-input v-model="form.scene" disabled />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="8">
              <el-form-item :label="t('mail.name')">
                <el-input v-model="form.name" :placeholder="t('mail.templateNamePlaceholder')" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="8">
              <el-form-item :label="t('mail.templateId')">
                <el-input-number
                  v-model="form.tencentTemplateId"
                  :min="1"
                  :step="1"
                  controls-position="right"
                />
              </el-form-item>
            </el-col>
            <el-col :xs="24">
              <el-form-item :label="t('mail.subject')">
                <el-input v-model="form.subject" :placeholder="t('mail.subjectPlaceholder')" />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('mail.variables')">
                <el-input-tag v-model="form.variableKeys" draggable />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('mail.exampleVariables')">
                <div class="mail-template-form__examples">
                  <el-input
                    v-for="key in form.variableKeys"
                    :key="key"
                    v-model="form.exampleVariables[key]"
                    :placeholder="key"
                  >
                    <template #prepend>{{ key }}</template>
                  </el-input>
                </div>
              </el-form-item>
            </el-col>
          </el-row>
        </section>
        <section class="mail-template-workbench">
          <div class="mail-template-workbench__pane">
            <header>
              <strong>{{ t('mail.content') }}</strong>
              <span>编辑邮件正文和完整 HTML</span>
            </header>
            <MailHtmlEditor v-if="editorVisible" v-model="form.content" />
          </div>
          <div class="mail-template-workbench__pane mail-template-workbench__preview">
            <header>
              <strong>{{ t('mail.preview') }}</strong>
              <span>示例变量已实时替换</span>
            </header>
            <iframe
              class="mail-preview"
              sandbox=""
              referrerpolicy="no-referrer"
              :srcdoc="previewHtml"
            />
          </div>
        </section>
      </el-form>
      <template #footer>
        <div class="mail-template-dialog__footer">
          <el-button @click="copyHtml">{{ t('mail.copyHtml') }}</el-button>
          <div>
            <el-button @click="dialog = false">{{ t('mail.cancel') }}</el-button>
            <el-button type="primary" :loading="saving" @click="saveTemplate">{{
              t('mail.save')
            }}</el-button>
          </div>
        </div>
      </template>
    </AppDialog>
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

.mail-preview {
  display: block;
  width: 100%;
  height: 408px;
  border: 0;
  background: #f4f7fb;
}
.mail-template-form__meta {
  padding: 20px 24px 4px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.mail-template-form__meta :deep(.el-input-number) {
  width: 100%;
}
.mail-template-form__examples {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  width: 100%;
}
.mail-template-workbench {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(360px, 0.85fr);
  gap: 16px;
  padding: 20px 24px 24px;
}
.mail-template-workbench__pane {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--el-bg-color);
}
.mail-template-workbench__pane > header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.mail-template-workbench__pane > header strong {
  font-size: 14px;
}
.mail-template-workbench__pane > header span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.mail-template-workbench__pane :deep(.mail-html-editor) {
  padding: 14px;
}
.mail-template-workbench__preview {
  background: #f4f7fb;
}
.mail-template-dialog__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}
@media (max-width: 900px) {
  .mail-template-workbench {
    grid-template-columns: 1fr;
  }
  .mail-template-form__examples {
    grid-template-columns: 1fr;
  }
}
</style>
