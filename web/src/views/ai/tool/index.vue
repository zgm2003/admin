<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import AppPage from '@/components/AppPage/index.vue'
import AppTable from '@/components/AppTable/index.vue'
import AppDialog from '@/components/AppDialog/index.vue'
import AppSearch from '@/components/AppSearch/index.vue'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField } from '@/components/AppSearch'
import { usePermissionStore } from '@/store/permission'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import {
  createTool,
  deleteTool,
  getTool,
  getToolOptions,
  getTools,
  updateTool,
  updateToolStatus,
} from '@/api/ai/tool'
import type { JsonObject, JsonValue, ToolItem, ToolOptions, VersionInput } from '@/api/ai/tool'
const { t, locale } = useI18n()
const access = usePermissionStore()
const {
  options,
  loading: optionsLoading,
  error: optionsError,
} = useLocalizedOptions<ToolOptions | null>(getToolOptions, () => null)
const canList = computed(() => access.hasPermission('ai:tool:list'))
const canCreate = computed(() => access.hasPermission('ai:tool:create'))
const canDetail = computed(() => access.hasPermission('ai:tool:detail'))
const canUpdate = computed(() => access.hasPermission('ai:tool:update') && canDetail.value)
const canStatus = computed(() => access.hasPermission('ai:tool:status'))
const canDelete = computed(() => access.hasPermission('ai:tool:delete'))
const rows = ref<ToolItem[]>([])
const pagination = ref<TablePaginationState>({ currentPage: 1, pageSize: 20, total: 0 })
const search = ref({ keyword: '' })
const searchFields = computed<SearchField<{ keyword: string }>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('aiTool.keyword'),
    placeholder: t('aiTool.keyword'),
    resetValue: '',
    width: 260,
  },
])
const columns = computed<TableColumn<ToolItem>[]>(() => [
  { prop: 'code', label: t('aiTool.code'), minWidth: 130 },
  { prop: 'name', label: t('aiTool.name'), minWidth: 180 },
  { prop: 'type', label: t('aiTool.type'), width: 120 },
  { prop: 'currentVersion', label: t('aiTool.version'), width: 110 },
  { prop: 'status', label: t('aiTool.status'), width: 100 },
  { prop: 'updatedAt', label: t('common.updatedAt'), minWidth: 180 },
  { key: 'actions', label: t('common.actions'), width: 290, fixed: 'right' },
])
const loading = ref(false)
const loadError = ref('')
let listSequence = 0
async function load(): Promise<void> {
  const token = ++listSequence
  if (!canList.value) {
    rows.value = []
    pagination.value.total = 0
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const result = await getTools({
      page: pagination.value.currentPage,
      pageSize: pagination.value.pageSize,
      ...(search.value.keyword.trim() ? { keyword: search.value.keyword.trim() } : {}),
    })
    if (token !== listSequence || !canList.value) return
    rows.value = result.list
    pagination.value.total = result.total
  } catch {
    if (token === listSequence) loadError.value = t('aiTool.loadFailed')
  } finally {
    if (token === listSequence) loading.value = false
  }
}
function query(): void {
  pagination.value.currentPage = 1
  void load()
}
function reset(): void {
  search.value.keyword = ''
  query()
}
function changePage(value: TablePaginationState): void {
  pagination.value = value
  void load()
}
const visible = ref(false)
const editingId = ref<string | null>(null)
const readonly = ref(false)
const saving = ref(false)
const formError = ref('')
interface ToolForm {
  code: string
  name: string
  type: number
  description: string
  builtinKey: string
  inputSchema: string
  outputSchema: string
  httpDefinition: string
  secretHeaders: string
  clearSecrets: boolean
  timeoutMs: number
  outputLimitBytes: number
  hasSideEffect: number
  idempotencyMode: number
  status: number
}
const form = ref<ToolForm>({
  code: '',
  name: '',
  type: 0,
  description: '',
  builtinKey: '',
  inputSchema: '{}',
  outputSchema: '{}',
  httpDefinition: '{}',
  secretHeaders: '',
  clearSecrets: false,
  timeoutMs: 0,
  outputLimitBytes: 0,
  hasSideEffect: 0,
  idempotencyMode: 0,
  status: 0,
})
function openCreate(): void {
  if (!canCreate.value || options.value === null) return
  editingId.value = null
  readonly.value = false
  formError.value = ''
  form.value = {
    code: '',
    name: '',
    type: 0,
    description: '',
    builtinKey: '',
    inputSchema: '{"type":"object"}',
    outputSchema: '{"type":"object"}',
    httpDefinition: '{}',
    secretHeaders: '',
    clearSecrets: false,
    timeoutMs: options.value.defaults.timeoutMs,
    outputLimitBytes: options.value.defaults.outputLimitBytes,
    hasSideEffect: 0,
    idempotencyMode: 0,
    status: options.value.defaults.status,
  }
  visible.value = true
}
async function openDetail(row: ToolItem, edit: boolean): Promise<void> {
  if (!canDetail.value || (edit && !canUpdate.value)) return
  loading.value = true
  try {
    const detail = await getTool(row.id)
    if (!canDetail.value || (edit && !canUpdate.value)) return
    const v = detail.version
    editingId.value = row.id
    readonly.value = !edit
    formError.value = ''
    form.value = {
      code: detail.tool.code,
      name: detail.tool.name,
      type: v.type,
      description: v.description,
      builtinKey: v.builtinKey === null ? '' : v.builtinKey,
      inputSchema: JSON.stringify(v.inputSchema, null, 2),
      outputSchema: JSON.stringify(v.outputSchema, null, 2),
      httpDefinition: JSON.stringify(v.httpDefinition === null ? {} : v.httpDefinition, null, 2),
      secretHeaders: '',
      clearSecrets: false,
      timeoutMs: v.timeoutMs,
      outputLimitBytes: v.outputLimitBytes,
      hasSideEffect: v.hasSideEffect,
      idempotencyMode: v.idempotencyMode,
      status: detail.tool.status,
    }
    visible.value = true
  } catch {
    loadError.value = t('aiTool.loadFailed')
  } finally {
    loading.value = false
  }
}
function object(raw: string): JsonObject {
  const value: JsonValue = JSON.parse(raw)
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('object')
  return value
}
async function save(): Promise<void> {
  if (
    saving.value ||
    readonly.value ||
    (editingId.value === null ? !canCreate.value : !canUpdate.value)
  )
    return
  formError.value = ''
  if (!form.value.code.trim() || !form.value.name.trim() || form.value.type === 0) {
    formError.value = t('aiTool.required')
    return
  }
  let version: VersionInput
  try {
    let secretHeaders: Record<string, string> | undefined
    if (form.value.clearSecrets) secretHeaders = {}
    else if (form.value.secretHeaders.trim()) {
      const raw = object(form.value.secretHeaders)
      secretHeaders = {}
      for (const [k, v] of Object.entries(raw)) {
        if (typeof v !== 'string') throw new Error('headers')
        secretHeaders[k] = v
      }
    }
    version = {
      type: form.value.type,
      description: form.value.description,
      inputSchema: object(form.value.inputSchema),
      outputSchema: object(form.value.outputSchema),
      builtinKey: form.value.type === 2 ? form.value.builtinKey : '',
      httpDefinition: form.value.type === 1 ? object(form.value.httpDefinition) : null,
      timeoutMs: form.value.timeoutMs,
      outputLimitBytes: form.value.outputLimitBytes,
      hasSideEffect: form.value.hasSideEffect,
      idempotencyMode: form.value.idempotencyMode,
      ...(secretHeaders === undefined ? {} : { secretHeaders }),
    }
  } catch {
    formError.value = t('aiTool.invalidJson')
    return
  }
  saving.value = true
  try {
    if (editingId.value === null)
      await createTool({
        code: form.value.code,
        name: form.value.name,
        status: form.value.status,
        version,
      })
    else await updateTool(editingId.value, { name: form.value.name, version })
    form.value.secretHeaders = ''
    visible.value = false
    ElNotification.success(t('aiTool.saved'))
    await load()
  } catch {
    formError.value = t('aiTool.saveFailed')
  } finally {
    saving.value = false
  }
}
const statusVisible = ref(false)
const statusRow = ref<ToolItem | null>(null)
const nextStatus = ref<number>(0)
function openStatus(row: ToolItem): void {
  statusRow.value = row
  nextStatus.value = row.status
  statusVisible.value = true
}
async function saveStatus(): Promise<void> {
  if (!canStatus.value || statusRow.value === null || saving.value) return
  saving.value = true
  try {
    await updateToolStatus(statusRow.value.id, nextStatus.value)
    statusVisible.value = false
    await load()
  } catch {
  } finally {
    saving.value = false
  }
}
async function remove(row: ToolItem): Promise<void> {
  if (!canDelete.value) return
  try {
    await ElMessageBox.confirm(t('aiTool.confirmDelete'), t('aiTool.title'), { type: 'warning' })
  } catch {
    return
  }
  try {
    await deleteTool(row.id)
    await load()
  } catch {}
}
function statusLabel(value: number): string {
  const found = options.value?.statuses.find((o) => o.value === value)
  return found === undefined ? String(value) : found.label
}
function typeLabel(value: number): string {
  const found = options.value?.types.find((o) => o.value === value)
  return found === undefined ? String(value) : found.label
}
watch(
  [canList, locale],
  () => {
    ++listSequence
    rows.value = []
    pagination.value.total = 0
    void load()
  },
  { immediate: true },
)
watch(visible, (value) => {
  if (!value) form.value.secretHeaders = ''
})
</script>

<template>
  <AppPage class="ai-tool-page">
    <AppSearch v-model="search" :fields="searchFields" @query="query" @reset="reset" />
    <el-alert
      v-if="optionsError"
      :title="t('aiTool.optionsFailed')"
      type="error"
      :closable="false"
    />
    <AppTable
      :data="rows"
      :columns="columns"
      :loading="loading"
      :pagination="pagination"
      :result-state="loadError ? 'error' : loading ? 'loading' : rows.length ? 'success' : 'empty'"
      :status-message="loadError"
      @refresh="load"
      @update:pagination="changePage"
    >
      <template #toolbar-left
        ><h2>{{ t('aiTool.title') }}</h2>
        <el-button
          v-if="canCreate"
          data-testid="ai-tool-create"
          type="primary"
          :disabled="optionsLoading || options === null"
          @click="openCreate"
          >{{ t('aiTool.create') }}</el-button
        ></template
      >
      <template #cell-type="{ row }: { row: ToolItem }">{{ typeLabel(row.type) }}</template>
      <template #cell-status="{ row }: { row: ToolItem }">{{ statusLabel(row.status) }}</template>
      <template #cell-actions="{ row }: { row: ToolItem }">
        <el-button
          v-if="canDetail"
          link
          data-testid="ai-tool-detail"
          @click="openDetail(row, false)"
          >{{ t('aiTool.detail') }}</el-button
        >
        <el-button
          v-if="canUpdate"
          link
          data-testid="ai-tool-edit"
          @click="openDetail(row, true)"
          >{{ t('aiTool.edit') }}</el-button
        >
        <el-button v-if="canStatus" link @click="openStatus(row)">{{
          t('aiTool.changeStatus')
        }}</el-button>
        <el-button v-if="canDelete" link type="danger" @click="remove(row)">{{
          t('aiTool.delete')
        }}</el-button>
      </template>
    </AppTable>
    <AppDialog
      v-model="visible"
      width="860px"
      :title="t(readonly ? 'aiTool.detail' : editingId === null ? 'aiTool.create' : 'aiTool.edit')"
    >
      <el-alert :title="t('aiTool.versionHint')" type="info" :closable="false" />
      <el-alert v-if="formError" :title="formError" type="error" :closable="false" />
      <el-form
        :model="form"
        label-position="top"
        :disabled="readonly || saving"
        class="ai-tool-form"
      >
        <div class="ai-tool-form__grid">
          <el-form-item :label="t('aiTool.code')" required
            ><el-input
              v-model="form.code"
              :disabled="editingId !== null"
              :placeholder="t('aiTool.codePlaceholder')"
              data-testid="ai-tool-code"
          /></el-form-item>
          <el-form-item :label="t('aiTool.name')" required
            ><el-input
              v-model="form.name"
              :placeholder="t('aiTool.namePlaceholder')"
              data-testid="ai-tool-name"
          /></el-form-item>
          <el-form-item :label="t('aiTool.type')" required
            ><el-select-v2
              v-model="form.type"
              :options="options === null ? [] : options.types"
              :placeholder="t('aiTool.type')"
          /></el-form-item>
          <el-form-item v-if="editingId === null" :label="t('aiTool.status')"
            ><el-select-v2
              v-model="form.status"
              :options="options === null ? [] : options.statuses"
          /></el-form-item>
          <el-form-item v-if="form.type === 2" :label="t('aiTool.builtin')" required
            ><el-select-v2
              v-model="form.builtinKey"
              :options="options === null ? [] : options.builtins"
              :placeholder="t('aiTool.builtin')"
          /></el-form-item>
          <el-form-item :label="t('aiTool.timeout')"
            ><el-input-number
              v-model="form.timeoutMs"
              :min="options?.constraints.minTimeoutMs"
              :max="options?.constraints.maxTimeoutMs"
              :precision="0"
          /></el-form-item>
          <el-form-item :label="t('aiTool.outputLimit')"
            ><el-input-number
              v-model="form.outputLimitBytes"
              :min="1"
              :max="options?.constraints.maxOutputLimitBytes"
              :precision="0"
          /></el-form-item>
          <el-form-item :label="t('aiTool.sideEffect')"
            ><el-switch v-model="form.hasSideEffect" :active-value="1" :inactive-value="0"
          /></el-form-item>
          <el-form-item :label="t('aiTool.idempotency')"
            ><el-select-v2
              v-model="form.idempotencyMode"
              :options="options === null ? [] : options.idempotencyModes"
          /></el-form-item>
        </div>
        <el-form-item :label="t('aiTool.description')"
          ><el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            :placeholder="t('aiTool.descriptionPlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiTool.inputSchema')" required
          ><el-input
            v-model="form.inputSchema"
            type="textarea"
            :rows="5"
            :placeholder="t('aiTool.jsonPlaceholder')"
            data-testid="ai-tool-input-schema"
        /></el-form-item>
        <el-form-item :label="t('aiTool.outputSchema')" required
          ><el-input
            v-model="form.outputSchema"
            type="textarea"
            :rows="5"
            :placeholder="t('aiTool.jsonPlaceholder')"
        /></el-form-item>
        <template v-if="form.type === 1">
          <el-form-item :label="t('aiTool.httpDefinition')" required
            ><el-input
              v-model="form.httpDefinition"
              type="textarea"
              :rows="6"
              :placeholder="t('aiTool.httpPlaceholder')"
          /></el-form-item>
          <el-form-item :label="t('aiTool.secretHeaders')"
            ><el-input
              v-model="form.secretHeaders"
              type="password"
              show-password
              autocomplete="new-password"
              :placeholder="t('aiTool.secretPlaceholder')"
              data-testid="ai-tool-secret"
            /><small>{{ t('aiTool.secretHint') }}</small></el-form-item
          >
          <el-checkbox v-if="editingId !== null" v-model="form.clearSecrets">{{
            t('aiTool.clearSecrets')
          }}</el-checkbox>
        </template>
      </el-form>
      <template #footer
        ><el-button @click="visible = false">{{ t('aiTool.cancel') }}</el-button
        ><el-button
          v-if="!readonly"
          type="primary"
          :loading="saving"
          data-testid="ai-tool-save"
          @click="save"
          >{{ t('aiTool.save') }}</el-button
        ></template
      >
    </AppDialog>
    <AppDialog v-model="statusVisible" :title="t('aiTool.changeStatus')" width="420px"
      ><el-select-v2
        v-model="nextStatus"
        :options="options === null ? [] : options.statuses"
      /><template #footer
        ><el-button @click="statusVisible = false">{{ t('aiTool.cancel') }}</el-button
        ><el-button type="primary" :loading="saving" @click="saveStatus">{{
          t('aiTool.save')
        }}</el-button></template
      ></AppDialog
    >
  </AppPage>
</template>
<style scoped lang="scss">
.ai-tool-form {
  margin-top: 16px;
  &__grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 16px;
  }
  small {
    color: var(--el-text-color-secondary);
  }
}
@media (max-width: 640px) {
  .ai-tool-form__grid {
    grid-template-columns: 1fr;
  }
}
</style>
