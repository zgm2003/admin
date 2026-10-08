<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { useI18n } from 'vue-i18n'
import {
  getMailRuleImportTemplate,
  importMailRules,
  mailRuleCSVMaxBytes,
  previewMailRuleImport,
  type MailRuleCSVPreview,
  type MailRuleCSVRow,
} from '@/api/message/mail'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { requestObjectURL } from '@/api/storage/upload'

const props = defineProps<{ canImport: boolean }>()
const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{ imported: [] }>()
const { t } = useI18n()
const fileInput = ref<HTMLInputElement | null>(null)
const fileName = ref('')
const content = ref<string | null>(null)
const preview = ref<MailRuleCSVPreview | null>(null)
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

interface PreviewRow extends MailRuleCSVRow {
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
  { prop: 'line', label: t('mail.ruleCSV.line'), width: 80 },
  { key: 'values', prop: 'values', label: t('mail.ruleCSV.values'), minWidth: 360 },
  { key: 'errors', prop: 'errors', label: t('mail.ruleCSV.validation'), minWidth: 260 },
])

function current(sequence: number): boolean {
  return mounted && visible.value && props.canImport && sequence === previewSequence
}

function invalidate(): void {
  previewSequence++
  templateSequence++
  content.value = null
  preview.value = null
  fileName.value = ''
  error.value = ''
  reading.value = false
  previewing.value = false
  templateURL.value = ''
  templateLoading.value = false
  templateError.value = ''
  page.value = 1
}

async function loadTemplate(): Promise<void> {
  const sequence = ++templateSequence
  templateLoading.value = true
  try {
    const result = await getMailRuleImportTemplate()
    if (!mounted || !visible.value || !props.canImport || sequence !== templateSequence) return
    if (result.objectKey === '') return
    const resolved = await requestObjectURL(result.objectKey)
    if (mounted && visible.value && props.canImport && sequence === templateSequence)
      templateURL.value = resolved.url
  } catch {
    if (mounted && visible.value && sequence === templateSequence)
      templateError.value = t('mail.ruleCSV.templateFailed')
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
  { flush: 'sync' },
)
onBeforeUnmount(() => {
  mounted = false
  invalidate()
})

function readFile(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error(t('mail.ruleCSV.readFailed')))
    reader.onabort = () => reject(new Error(t('mail.ruleCSV.readFailed')))
    reader.onload = () => {
      if (!(reader.result instanceof ArrayBuffer)) {
        reject(new Error(t('mail.ruleCSV.readFailed')))
        return
      }
      try {
        resolve(new TextDecoder('utf-8', { fatal: true }).decode(new Uint8Array(reader.result)))
      } catch {
        reject(new Error(t('mail.ruleCSV.error.invalid_encoding')))
      }
    }
    reader.readAsArrayBuffer(file)
  })
}

async function selectFile(event: Event): Promise<void> {
  if (!(event.target instanceof HTMLInputElement) || saving.value || !props.canImport) return
  const file = event.target.files === null ? undefined : event.target.files[0]
  event.target.value = ''
  if (file === undefined || file === null) return
  const sequence = ++previewSequence
  preview.value = null
  content.value = null
  fileName.value = file.name
  page.value = 1
  error.value = ''
  reading.value = false
  previewing.value = false
  if (!file.name.toLowerCase().endsWith('.csv')) {
    error.value = t('mail.ruleCSV.csvOnly')
    return
  }
  if (file.size > mailRuleCSVMaxBytes) {
    error.value = t('mail.ruleCSV.error.too_large')
    return
  }
  reading.value = true
  try {
    const value = await readFile(file)
    if (!current(sequence)) return
    content.value = value
    reading.value = false
    await runPreview(sequence, value)
  } catch (cause: unknown) {
    if (current(sequence))
      error.value = cause instanceof Error ? cause.message : t('mail.ruleCSV.readFailed')
  } finally {
    if (current(sequence)) reading.value = false
  }
}

async function runPreview(sequence: number, value: string): Promise<void> {
  previewing.value = true
  preview.value = null
  error.value = ''
  try {
    const result = await previewMailRuleImport(value)
    if (current(sequence)) preview.value = result
  } catch {
    if (current(sequence)) error.value = t('mail.ruleCSV.previewFailed')
  } finally {
    if (current(sequence)) previewing.value = false
  }
}

function retryPreview(): void {
  if (content.value !== null && props.canImport && !saving.value && !reading.value)
    void runPreview(++previewSequence, content.value)
}

async function confirmImport(): Promise<void> {
  if (!canConfirm.value || content.value === null) return
  const sequence = previewSequence
  saving.value = true
  try {
    const result = await importMailRules(content.value)
    emit('imported')
    if (current(sequence)) {
      ElMessage.success(t('mail.ruleCSV.imported', { count: result.imported }))
      visible.value = false
    }
  } catch {
    if (current(sequence)) {
      preview.value = null
      error.value = t('mail.ruleCSV.confirmFailed')
    }
  } finally {
    saving.value = false
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
    :title="t('mail.ruleCSV.importTitle')"
    width="min(1000px, 96vw)"
    height="560px"
    :show-close="!saving"
    :close-on-press-escape="!saving"
    :close-on-click-modal="false"
  >
    <el-alert :title="t('mail.ruleCSV.instructions')" type="info" :closable="false" show-icon />
    <p class="csv-format">{{ t('mail.ruleCSV.format') }}</p>
    <div class="csv-toolbar">
      <a
        v-if="templateURL"
        class="el-button"
        :href="templateURL"
        target="_blank"
        rel="noopener noreferrer"
        download="mail-recipient-rule-import.csv"
        data-testid="mail-rule-template-download"
        >{{ t('mail.ruleCSV.downloadTemplate') }}</a>
      <el-button v-else :loading="templateLoading" disabled>{{
        t('mail.ruleCSV.downloadTemplate')
      }}</el-button>
      <el-button :disabled="saving || !canImport" @click="fileInput?.click()">{{
        t('mail.ruleCSV.selectFile')
      }}</el-button>
      <input
        ref="fileInput"
        type="file"
        accept=".csv,text/csv"
        class="csv-file-input"
        data-testid="mail-rule-import-file"
        :disabled="saving || !canImport"
        :aria-label="t('mail.ruleCSV.selectFile')"
        @change="selectFile"
      />
      <span class="csv-file-name">{{ fileName }}</span>
    </div>
    <p v-if="!templateLoading && !templateURL" class="csv-template-hint">
      {{ templateError || t('mail.ruleCSV.templateMissing') }}
    </p>
    <el-alert
      v-if="error"
      :title="error"
      type="error"
      :closable="false"
      show-icon
      class="csv-alert"
    />
    <el-alert
      v-for="code in preview === null ? [] : preview.errors"
      :key="code"
      :title="t(`mail.ruleCSV.error.${code}`)"
      type="error"
      :closable="false"
      show-icon
      class="csv-alert"
    />
    <p v-if="preview !== null" data-testid="mail-rule-import-summary">
      {{
        t('mail.ruleCSV.summary', {
          total: rows.length,
          valid: validCount,
          invalid: rows.length - validCount,
        })
      }}
    </p>
    <AppTable
      v-if="preview !== null || reading || previewing"
      :columns="columns"
      :data="pageRows"
      :loading="reading || previewing"
      row-key="rowId"
      :pagination="pagination"
      :aria-label="t('mail.ruleCSV.preview')"
      :refresh-label="t('mail.ruleCSV.preview')"
      @refresh="retryPreview"
      @update:pagination="changePage"
    >
      <template #cell-values="{ row }: { row: PreviewRow }"
        ><span class="csv-values">{{ row.values.join(' | ') }}</span></template
      >
      <template #cell-errors="{ row }: { row: PreviewRow }">
        <el-tag v-if="row.errors.length === 0" type="success">{{ t('mail.ruleCSV.valid') }}</el-tag>
        <ul v-else class="csv-errors">
          <li v-for="code in row.errors" :key="code">{{ t(`mail.ruleCSV.error.${code}`) }}</li>
        </ul>
      </template>
    </AppTable>
    <template #footer>
      <el-button :disabled="saving" @click="visible = false">{{ t('mail.cancel') }}</el-button>
      <el-button
        :disabled="content === null || reading || previewing || saving"
        :loading="previewing"
        data-testid="mail-rule-import-preview"
        @click="retryPreview"
        >{{ t('mail.ruleCSV.preview') }}</el-button
      >
      <el-button
        type="primary"
        :disabled="!canConfirm"
        :loading="saving"
        data-testid="mail-rule-import-confirm"
        @click="confirmImport"
        >{{ t('mail.ruleCSV.confirm') }}</el-button
      >
    </template>
  </AppDialog>
</template>

<style scoped>
.csv-format,
.csv-template-hint {
  color: var(--el-text-color-secondary);
  line-height: 1.7;
  font-size: 13px;
}
.csv-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin: 14px 0;
}
.csv-file-input {
  display: none;
}
.csv-file-name {
  overflow-wrap: anywhere;
}
.csv-alert {
  margin: 12px 0;
}
.csv-values {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.csv-errors {
  margin: 0;
  padding-left: 16px;
  color: var(--el-color-danger);
}
</style>
