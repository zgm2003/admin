<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useI18n } from 'vue-i18n'
import {
  getMailRuleImportTemplate,
  importMailRuleXlsx,
  mailRuleXlsxMaxBytes,
  previewMailRuleXlsx,
  type MailRuleXlsxPreview,
  type MailRuleXlsxRow,
} from '@/api/message/mail'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { requestObjectURL } from '@/api/storage/upload'
import { MailRuleAction, MailRuleScope } from '@/enums/mailRecipientRule'
import { YesNo } from '@/enums/yesNo'
import { readXlsxFile } from './readXlsxFile'

const props = defineProps<{ canImport: boolean }>()
const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{ imported: [] }>()
const { t } = useI18n()
const fileInput = ref<HTMLInputElement | null>(null)
const fileName = ref('')
const content = ref<string | null>(null)
const preview = ref<MailRuleXlsxPreview | null>(null)
const templateURL = ref('')
const templateLoading = ref(false)
const templateError = ref('')
const reading = ref(false)
const previewing = ref(false)
const saving = ref(false)
const error = ref('')
const page = ref(1)
const pageSize = ref(20)
let previewSequence = 0
let templateSequence = 0
let mounted = true
let previewController: AbortController | null = null
let templateController: AbortController | null = null

interface PreviewRow extends MailRuleXlsxRow {
  rowId: string
}
const rows = computed<PreviewRow[]>(() =>
  preview.value === null
    ? []
    : preview.value.rows.map((row) => ({ ...row, rowId: String(row.line) })),
)
const pageRows = computed(() =>
  rows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)
const validCount = computed(() => rows.value.filter((row) => row.errors.length === 0).length)
const canConfirm = computed(
  () =>
    props.canImport &&
    !reading.value &&
    !previewing.value &&
    !saving.value &&
    preview.value !== null &&
    preview.value.errors.length === 0 &&
    rows.value.length > 0 &&
    validCount.value === rows.value.length,
)
const pagination = computed<TablePaginationState>(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: rows.value.length,
}))
const columns = computed<TableColumn<PreviewRow>[]>(() => [
  { prop: 'line', label: t('mail.ruleXlsx.line'), width: 120 },
  { key: 'values', prop: 'rawValues', label: t('mail.ruleXlsx.values'), minWidth: 360 },
  { key: 'errors', prop: 'errors', label: t('mail.ruleXlsx.validation'), minWidth: 260 },
])

function displayValues(row: MailRuleXlsxRow): string {
  const data = row.data
  return (
    data === null
      ? row.rawValues
      : [
          t(data.scope === MailRuleScope.Email ? 'mail.email' : 'mail.domain'),
          data.pattern,
          t(data.action === MailRuleAction.Allow ? 'mail.allow' : 'mail.deny'),
          data.name,
          data.remark,
          t(data.isEnabled === YesNo.Yes ? 'mail.enabled' : 'mail.disabled'),
        ]
  ).join(' | ')
}

function current(sequence: number): boolean {
  return mounted && visible.value && props.canImport && sequence === previewSequence
}

function invalidate(): void {
  previewSequence++
  templateSequence++
  previewController?.abort()
  templateController?.abort()
  content.value = null
  preview.value = null
  fileName.value = ''
  error.value = ''
  reading.value = false
  previewing.value = false
  saving.value = false
  templateURL.value = ''
  templateLoading.value = false
  templateError.value = ''
  page.value = 1
}

async function loadTemplate(): Promise<void> {
  if (!visible.value || !props.canImport || templateLoading.value) return
  const sequence = ++templateSequence
  templateController?.abort()
  const controller = new AbortController()
  templateController = controller
  templateLoading.value = true
  templateError.value = ''
  templateURL.value = ''
  try {
    const result = await getMailRuleImportTemplate(controller.signal)
    if (!mounted || !visible.value || !props.canImport || sequence !== templateSequence) return
    if (result.objectKey === '') return
    const resolved = await requestObjectURL(result.objectKey, controller.signal)
    if (mounted && visible.value && props.canImport && sequence === templateSequence)
      templateURL.value = resolved.url
  } catch {
    if (mounted && visible.value && sequence === templateSequence)
      templateError.value = t('mail.ruleXlsx.templateFailed')
  } finally {
    if (sequence === templateSequence) templateLoading.value = false
  }
}

watch(
  [visible, () => props.canImport],
  ([open, allowed]) => {
    invalidate()
    if (open && allowed) void loadTemplate()
    else if (open && !allowed) visible.value = false
  },
  { flush: 'sync', immediate: true },
)
onBeforeUnmount(() => {
  mounted = false
  invalidate()
})

async function selectFile(event: Event): Promise<void> {
  if (
    !(event.target instanceof HTMLInputElement) ||
    !visible.value ||
    saving.value ||
    !props.canImport
  )
    return
  const file = event.target.files === null ? undefined : event.target.files[0]
  event.target.value = ''
  if (file === undefined || file === null) return
  const sequence = ++previewSequence
  previewController?.abort()
  const controller = new AbortController()
  previewController = controller
  preview.value = null
  content.value = null
  fileName.value = file.name
  page.value = 1
  error.value = ''
  reading.value = false
  previewing.value = false
  if (!file.name.toLowerCase().endsWith('.xlsx')) {
    error.value = t('mail.ruleXlsx.xlsxOnly')
    return
  }
  if (file.size > mailRuleXlsxMaxBytes) {
    error.value = t('mail.ruleXlsx.error.too_large')
    return
  }
  if (file.size === 0) {
    error.value = t('mail.ruleXlsx.error.empty')
    return
  }
  reading.value = true
  try {
    const value = await readXlsxFile(file, controller.signal)
    if (!current(sequence)) return
    content.value = value
    reading.value = false
    await runPreview(sequence, value)
  } catch {
    if (current(sequence)) error.value = t('mail.ruleXlsx.readFailed')
  } finally {
    if (current(sequence)) reading.value = false
  }
}

async function runPreview(sequence: number, value: string): Promise<void> {
  previewing.value = true
  preview.value = null
  error.value = ''
  try {
    const result = await previewMailRuleXlsx(
      { fileName: fileName.value, contentBase64: value },
      previewController?.signal,
    )
    if (current(sequence)) preview.value = result
  } catch {
    if (current(sequence)) error.value = t('mail.ruleXlsx.previewFailed')
  } finally {
    if (current(sequence)) previewing.value = false
  }
}

function retryPreview(): void {
  if (
    content.value === null ||
    !visible.value ||
    !props.canImport ||
    saving.value ||
    reading.value ||
    previewing.value
  )
    return
  previewController?.abort()
  previewController = new AbortController()
  void runPreview(++previewSequence, content.value)
}

async function confirmImport(): Promise<void> {
  if (!canConfirm.value || content.value === null) return
  const sequence = previewSequence
  saving.value = true
  try {
    const result = await importMailRuleXlsx(
      { fileName: fileName.value, contentBase64: content.value },
      previewController?.signal,
    )
    if (current(sequence)) {
      emit('imported')
      ElMessage.success(t('mail.ruleXlsx.imported', { count: result.imported }))
      visible.value = false
    }
  } catch {
    if (current(sequence)) {
      preview.value = null
      error.value = t('mail.ruleXlsx.confirmFailed')
    }
  } finally {
    if (current(sequence)) saving.value = false
  }
}

function changePage(next: TablePaginationState): void {
  page.value = next.currentPage
  pageSize.value = next.pageSize
}
</script>

<template>
  <AppDialog
    v-model="visible"
    :title="t('mail.ruleXlsx.importTitle')"
    width="min(1000px, 96vw)"
    height="560px"
    :show-close="!saving"
    :close-on-press-escape="!saving"
    :close-on-click-modal="false"
  >
    <el-alert :title="t('mail.ruleXlsx.instructions')" type="info" :closable="false" show-icon />
    <p class="xlsx-format">{{ t('mail.ruleXlsx.format') }}</p>
    <div class="xlsx-toolbar">
      <a
        v-if="templateURL"
        class="el-button"
        :href="templateURL"
        target="_blank"
        rel="noopener noreferrer"
        download="mail-recipient-rule-import.xlsx"
        data-testid="mail-rule-template-download"
        >{{ t('mail.ruleXlsx.downloadTemplate') }}</a
      >
      <el-button v-else :loading="templateLoading" disabled>{{
        t('mail.ruleXlsx.downloadTemplate')
      }}</el-button>
      <el-button :disabled="saving || !canImport" @click="fileInput?.click()">{{
        t('mail.ruleXlsx.selectFile')
      }}</el-button>
      <input
        ref="fileInput"
        type="file"
        accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
        class="xlsx-file-input"
        data-testid="mail-rule-import-file"
        :disabled="saving || !canImport"
        :aria-label="t('mail.ruleXlsx.selectFile')"
        @change="selectFile"
      />
      <span class="xlsx-file-name">{{ fileName }}</span>
    </div>
    <p v-if="!templateLoading && !templateURL" class="xlsx-template-hint">
      {{ templateError || t('mail.ruleXlsx.templateMissing') }}
      <el-button
        v-if="templateError"
        text
        data-testid="mail-rule-template-retry"
        @click="loadTemplate"
        >{{ t('mail.ruleXlsx.retryTemplate') }}</el-button
      >
    </p>
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      show-icon
      class="xlsx-alert"
    />
    <el-alert
      v-for="code in preview === null ? [] : preview.errors"
      :key="code"
      :title="t(`mail.ruleXlsx.error.${code}`)"
      type="error"
      :closable="false"
      show-icon
      class="xlsx-alert"
    />
    <p v-if="preview !== null" data-testid="mail-rule-import-summary">
      {{
        t('mail.ruleXlsx.summary', {
          total: rows.length,
          valid: validCount,
          invalid: rows.length - validCount,
        })
      }}
    </p>
    <AppTable
      v-if="content !== null || reading || previewing"
      :columns="columns"
      :data="pageRows"
      :loading="reading || previewing"
      row-key="rowId"
      :pagination="pagination"
      :aria-label="t('mail.ruleXlsx.preview')"
      :refresh-label="t('mail.ruleXlsx.preview')"
      @refresh="retryPreview"
      @update:pagination="changePage"
    >
      <template #cell-values="{ row }: { row: PreviewRow }"
        ><span class="xlsx-values">{{ displayValues(row) }}</span></template
      >
      <template #cell-errors="{ row }: { row: PreviewRow }">
        <el-tag v-if="row.errors.length === 0" type="success">{{
          t('mail.ruleXlsx.valid')
        }}</el-tag>
        <ul v-else class="xlsx-errors">
          <li v-for="code in row.errors" :key="code">{{ t(`mail.ruleXlsx.error.${code}`) }}</li>
        </ul>
      </template>
    </AppTable>
    <p v-else-if="!error" class="xlsx-template-hint">{{ t('mail.ruleXlsx.emptyPreview') }}</p>
    <template #footer>
      <el-button
        :disabled="saving"
        data-testid="mail-rule-import-cancel"
        @click="visible = false"
        >{{ t('mail.cancel') }}</el-button
      >
      <el-button
        type="primary"
        :disabled="!canConfirm"
        :loading="saving"
        data-testid="mail-rule-import-confirm"
        @click="confirmImport"
        >{{ t('mail.ruleXlsx.confirm') }}</el-button
      >
    </template>
  </AppDialog>
</template>

<style scoped src="./MailRuleImportDialog.css"></style>
