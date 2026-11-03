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
  createAgent,
  deleteAgent,
  getAgent,
  getAgentCandidates,
  getAgentOptions,
  getAgents,
  switchAgentBinding,
  updateAgent,
  updateAgentStatus,
} from '@/api/ai/agent'
import type {
  AgentBinding,
  AgentCandidate,
  AgentItem,
  AgentOptions,
  AgentUpdateInput,
} from '@/api/ai/agent'
import type { JsonObject, JsonValue } from '@/api/ai/tool'
const { t, locale } = useI18n()
const access = usePermissionStore()
const {
  options,
  loading: optionsLoading,
  error: optionsError,
} = useLocalizedOptions<AgentOptions | null>(getAgentOptions, () => null)
const canList = computed(() => access.hasPermission('ai:agent:list'))
const canCreate = computed(() => access.hasPermission('ai:agent:create'))
const canDetail = computed(() => access.hasPermission('ai:agent:detail'))
const canUpdate = computed(() => access.hasPermission('ai:agent:update') && canDetail.value)
const canSwitch = computed(() => access.hasPermission('ai:agent:switchBinding') && canDetail.value)
const canStatus = computed(() => access.hasPermission('ai:agent:status'))
const canDelete = computed(() => access.hasPermission('ai:agent:delete'))
const rows = ref<AgentItem[]>([])
const pagination = ref<TablePaginationState>({ currentPage: 1, pageSize: 20, total: 0 })
const search = ref({ keyword: '' })
const searchFields = computed<SearchField<{ keyword: string }>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('aiAgent.keyword'),
    placeholder: t('aiAgent.keyword'),
    resetValue: '',
    width: 260,
  },
])
const columns = computed<TableColumn<AgentItem>[]>(() => [
  { prop: 'code', label: t('aiAgent.code'), minWidth: 120 },
  { prop: 'name', label: t('aiAgent.name'), minWidth: 160 },
  { prop: 'capabilities', label: t('aiAgent.capabilities'), minWidth: 160 },
  { prop: 'multiplierPpm', label: t('aiAgent.multiplier'), minWidth: 180 },
  { prop: 'activeBindingId', label: t('aiAgent.current'), minWidth: 130 },
  { prop: 'status', label: t('aiAgent.status'), width: 100 },
  { key: 'actions', label: t('common.actions'), width: 390, fixed: 'right' },
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
    const result = await getAgents({
      page: pagination.value.currentPage,
      pageSize: pagination.value.pageSize,
      ...(search.value.keyword.trim() ? { keyword: search.value.keyword.trim() } : {}),
    })
    if (token !== listSequence || !canList.value) return
    rows.value = result.list
    pagination.value.total = result.total
  } catch {
    if (token === listSequence) loadError.value = t('aiAgent.loadFailed')
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
interface BindingForm {
  id?: string
  providerModelId: string
  textProtocol: number
  parameters: string
  status: number
}
interface ToolForm {
  toolId: string
  exposedName: string
  constraints: string
  status: number
}
interface AgentForm {
  code: string
  name: string
  description: string
  avatarFileId: string
  capabilities: string[]
  systemPrompt: string
  multiplierPpm: number
  generationParams: string
  status: number
  activeProviderModelId: string | null
  bindings: BindingForm[]
  tools: ToolForm[]
}
const form = ref<AgentForm>({
  code: '',
  name: '',
  description: '',
  avatarFileId: '',
  capabilities: [],
  systemPrompt: '',
  multiplierPpm: 0,
  generationParams: '{}',
  status: 0,
  activeProviderModelId: null,
  bindings: [],
  tools: [],
})
const visible = ref(false)
const editingId = ref<string | null>(null)
const readonly = ref(false)
const saving = ref(false)
const formError = ref('')
const currentBinding = ref<string | null>(null)
const modelCandidates = ref<AgentCandidate[]>([])
const toolCandidates = ref<AgentCandidate[]>([])
const modelPage = ref(1)
const toolPage = ref(1)
const modelTotal = ref(0)
const toolTotal = ref(0)
const modelKeyword = ref('')
const toolKeyword = ref('')
const candidateLoading = ref(false)
let candidateSequence = 0
const providerOptions = computed(() => {
  const values = new Map(
    modelCandidates.value.map((c) => [c.id, { value: c.id, label: c.name, disabled: !c.valid }]),
  )
  for (const b of form.value.bindings) {
    if (b.providerModelId && !values.has(b.providerModelId))
      values.set(b.providerModelId, {
        value: b.providerModelId,
        label: b.providerModelId,
        disabled: false,
      })
  }
  return [...values.values()]
})
const toolOptions = computed(() => {
  const values = new Map(
    toolCandidates.value.map((c) => [c.id, { value: c.id, label: c.name, disabled: !c.valid }]),
  )
  for (const b of form.value.tools) {
    if (b.toolId && !values.has(b.toolId))
      values.set(b.toolId, { value: b.toolId, label: b.toolId, disabled: false })
  }
  return [...values.values()]
})
const initialOptions = computed(() =>
  form.value.bindings
    .filter((b) => b.providerModelId !== '')
    .map((b) => ({
      value: b.providerModelId,
      label:
        providerOptions.value.find((o) => o.value === b.providerModelId)?.label ??
        b.providerModelId,
    })),
)
async function candidates(
  kind: 'providerModel' | 'tool',
  keyword = '',
  more = false,
): Promise<void> {
  if (readonly.value) return
  const token = candidateSequence
  const id = editingId.value
  candidateLoading.value = true
  const page = kind === 'providerModel' ? modelPage : toolPage
  const queryText = kind === 'providerModel' ? modelKeyword : toolKeyword
  if (!more) {
    page.value = 1
    queryText.value = keyword
  } else page.value += 1
  try {
    const result = await getAgentCandidates(id, {
      kind,
      keyword: queryText.value,
      page: page.value,
      pageSize: 20,
    })
    if (token !== candidateSequence || id !== editingId.value || !visible.value) return
    const target = kind === 'providerModel' ? modelCandidates : toolCandidates
    target.value = more
      ? [...new Map([...target.value, ...result.list].map((v) => [v.id, v])).values()]
      : result.list
    if (kind === 'providerModel') modelTotal.value = result.total
    else toolTotal.value = result.total
  } catch {
    formError.value = t('aiAgent.loadFailed')
  } finally {
    candidateLoading.value = false
  }
}
function openCreate(): void {
  if (!canCreate.value || options.value === null) return
  ++candidateSequence
  editingId.value = null
  readonly.value = false
  formError.value = ''
  currentBinding.value = null
  form.value = {
    code: '',
    name: '',
    description: '',
    avatarFileId: '',
    capabilities: [],
    systemPrompt: '',
    multiplierPpm: options.value.defaults.multiplierPpm,
    generationParams: '{}',
    status: options.value.defaults.status,
    activeProviderModelId: null,
    bindings: [],
    tools: [],
  }
  modelCandidates.value = []
  toolCandidates.value = []
  visible.value = true
  void candidates('providerModel')
  void candidates('tool')
}
async function openDetail(row: AgentItem, edit: boolean): Promise<void> {
  if (!canDetail.value || (edit && !canUpdate.value)) return
  loading.value = true
  try {
    const detail = await getAgent(row.id)
    if (!canDetail.value || (edit && !canUpdate.value)) return
    ++candidateSequence
    editingId.value = row.id
    readonly.value = !edit
    formError.value = ''
    const a = detail.agent
    currentBinding.value = a.activeBindingId
    form.value = {
      code: a.code,
      name: a.name,
      description: a.description,
      avatarFileId: a.avatarFileId === null ? '' : a.avatarFileId,
      capabilities: [...a.capabilities],
      systemPrompt: a.systemPrompt,
      multiplierPpm: a.multiplierPpm,
      generationParams: JSON.stringify(a.generationParams, null, 2),
      status: a.status,
      activeProviderModelId: null,
      bindings: detail.bindings.map((b) => ({
        id: b.id,
        providerModelId: b.providerModelId,
        textProtocol: b.textProtocol,
        parameters: JSON.stringify(b.parameters, null, 2),
        status: b.status,
      })),
      tools: detail.tools.map((b) => ({
        toolId: b.toolId,
        exposedName: b.exposedName,
        constraints: JSON.stringify(b.parameterConstraints, null, 2),
        status: b.status,
      })),
    }
    modelCandidates.value = []
    toolCandidates.value = []
    visible.value = true
    if (edit) {
      void candidates('providerModel')
      void candidates('tool')
    }
  } catch {
    loadError.value = t('aiAgent.loadFailed')
  } finally {
    loading.value = false
  }
}
function addBinding(): void {
  if (options.value === null) return
  form.value.bindings.push({
    providerModelId: '',
    textProtocol: options.value.defaults.textProtocol,
    parameters: '{}',
    status: 1,
  })
}
function addTool(): void {
  form.value.tools.push({
    toolId: '',
    exposedName: '',
    constraints: '{"maxCalls":1,"allowedParameters":{}}',
    status: 1,
  })
}
function object(raw: string): JsonObject {
  const v: JsonValue = JSON.parse(raw)
  if (v === null || typeof v !== 'object' || Array.isArray(v)) throw new Error('object')
  return v
}
async function save(): Promise<void> {
  if (
    saving.value ||
    readonly.value ||
    (editingId.value === null ? !canCreate.value : !canUpdate.value)
  )
    return
  formError.value = ''
  if (
    !form.value.name.trim() ||
    !form.value.code.trim() ||
    form.value.capabilities.length === 0 ||
    form.value.bindings.some((b) => b.providerModelId === '') ||
    form.value.tools.some((b) => b.toolId === '' || !b.exposedName.trim())
  ) {
    formError.value = t('aiAgent.required')
    return
  }
  let input: AgentUpdateInput
  try {
    input = {
      name: form.value.name,
      description: form.value.description,
      avatarFileId: form.value.avatarFileId.trim() === '' ? null : form.value.avatarFileId.trim(),
      capabilities: [...form.value.capabilities],
      systemPrompt: form.value.systemPrompt,
      multiplierPpm: form.value.multiplierPpm,
      generationParams: object(form.value.generationParams),
      bindings: form.value.bindings.map((b) => ({ ...b, parameters: object(b.parameters) })),
      tools: form.value.tools.map((b) => ({
        toolId: b.toolId,
        exposedName: b.exposedName,
        parameterConstraints: object(b.constraints),
        status: b.status,
      })),
    }
  } catch {
    formError.value = t('aiAgent.invalidJson')
    return
  }
  saving.value = true
  try {
    if (editingId.value === null)
      await createAgent({
        ...input,
        code: form.value.code,
        status: form.value.status,
        activeProviderModelId: form.value.activeProviderModelId,
      })
    else await updateAgent(editingId.value, input)
    visible.value = false
    ElNotification.success(t('aiAgent.saved'))
    await load()
  } catch {
    formError.value = t('aiAgent.saveFailed')
  } finally {
    saving.value = false
  }
}
const switchVisible = ref(false)
const switchAgent = ref<string | null>(null)
const switchBindings = ref<AgentBinding[]>([])
const nextBinding = ref<string | null>(null)
const switchOptions = computed(() =>
  switchBindings.value.map((b) => ({
    value: b.id,
    label: `${b.providerModelId} · ${b.id}`,
    disabled: b.status !== 1,
  })),
)
async function openSwitch(row: AgentItem): Promise<void> {
  if (!canSwitch.value) return
  try {
    const d = await getAgent(row.id)
    if (!canSwitch.value) return
    switchAgent.value = row.id
    switchBindings.value = d.bindings
    nextBinding.value = d.agent.activeBindingId
    switchVisible.value = true
  } catch {
    loadError.value = t('aiAgent.loadFailed')
  }
}
async function saveSwitch(): Promise<void> {
  if (!canSwitch.value || switchAgent.value === null || nextBinding.value === null || saving.value)
    return
  saving.value = true
  try {
    await switchAgentBinding(switchAgent.value, nextBinding.value)
    switchVisible.value = false
    await load()
  } catch {
  } finally {
    saving.value = false
  }
}
const statusVisible = ref(false)
const statusAgent = ref<string | null>(null)
const nextStatus = ref(0)
function openStatus(row: AgentItem): void {
  statusAgent.value = row.id
  nextStatus.value = row.status
  statusVisible.value = true
}
async function saveStatus(): Promise<void> {
  if (!canStatus.value || statusAgent.value === null || saving.value) return
  saving.value = true
  try {
    await updateAgentStatus(statusAgent.value, nextStatus.value)
    statusVisible.value = false
    await load()
  } catch {
  } finally {
    saving.value = false
  }
}
async function remove(row: AgentItem): Promise<void> {
  if (!canDelete.value) return
  try {
    await ElMessageBox.confirm(t('aiAgent.confirmDelete'), t('aiAgent.title'), { type: 'warning' })
  } catch {
    return
  }
  try {
    await deleteAgent(row.id)
    await load()
  } catch {}
}
function statusLabel(value: number): string {
  const o = options.value?.statuses.find((o) => o.value === value)
  return o === undefined ? String(value) : o.label
}
function capabilityLabel(value: string): string {
  const o = options.value?.capabilities.find((o) => o.value === value)
  return o === undefined ? value : o.label
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
  if (!value) ++candidateSequence
})
</script>
<template>
  <AppPage class="ai-agent-page">
    <AppSearch v-model="search" :fields="searchFields" @query="query" @reset="reset" />
    <el-alert
      v-if="optionsError"
      :title="t('aiAgent.optionsFailed')"
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
        ><h2>{{ t('aiAgent.title') }}</h2>
        <el-button
          v-if="canCreate"
          data-testid="ai-agent-create"
          type="primary"
          :disabled="optionsLoading || options === null"
          @click="openCreate"
          >{{ t('aiAgent.create') }}</el-button
        ></template
      >
      <template #cell-capabilities="{ row }: { row: AgentItem }"
        ><el-tag v-for="capability in row.capabilities" :key="capability" size="small">{{
          capabilityLabel(capability)
        }}</el-tag></template
      >
      <template #cell-status="{ row }: { row: AgentItem }">{{ statusLabel(row.status) }}</template>
      <template #cell-actions="{ row }: { row: AgentItem }"
        ><el-button
          v-if="canDetail"
          link
          data-testid="ai-agent-detail"
          @click="openDetail(row, false)"
          >{{ t('aiAgent.detail') }}</el-button
        ><el-button
          v-if="canUpdate"
          link
          data-testid="ai-agent-edit"
          @click="openDetail(row, true)"
          >{{ t('aiAgent.edit') }}</el-button
        ><el-button v-if="canSwitch" link data-testid="ai-agent-switch" @click="openSwitch(row)">{{
          t('aiAgent.switchCurrent')
        }}</el-button
        ><el-button v-if="canStatus" link @click="openStatus(row)">{{
          t('aiAgent.changeStatus')
        }}</el-button
        ><el-button v-if="canDelete" link type="danger" @click="remove(row)">{{
          t('aiAgent.delete')
        }}</el-button></template
      >
    </AppTable>
    <AppDialog
      v-model="visible"
      width="1000px"
      :title="
        t(readonly ? 'aiAgent.detail' : editingId === null ? 'aiAgent.create' : 'aiAgent.edit')
      "
    >
      <el-alert v-if="formError" :title="formError" type="error" :closable="false" />
      <el-form
        :model="form"
        label-position="top"
        :disabled="readonly || saving"
        class="ai-agent-form"
      >
        <div class="ai-agent-form__grid">
          <el-form-item :label="t('aiAgent.code')" required
            ><el-input
              v-model="form.code"
              :disabled="editingId !== null"
              :placeholder="t('aiAgent.codePlaceholder')"
              data-testid="ai-agent-code"
          /></el-form-item>
          <el-form-item :label="t('aiAgent.name')" required
            ><el-input
              v-model="form.name"
              :placeholder="t('aiAgent.namePlaceholder')"
              data-testid="ai-agent-name"
          /></el-form-item>
          <el-form-item :label="t('aiAgent.avatar')"
            ><el-input v-model="form.avatarFileId" :placeholder="t('aiAgent.avatarPlaceholder')"
          /></el-form-item>
          <el-form-item :label="t('aiAgent.capabilities')" required
            ><el-select-v2
              v-model="form.capabilities"
              multiple
              :options="options === null ? [] : options.capabilities"
              :placeholder="t('aiAgent.capabilities')"
          /></el-form-item>
          <el-form-item :label="t('aiAgent.multiplier')" required
            ><el-input-number
              v-model="form.multiplierPpm"
              :min="options?.constraints.minMultiplierPpm"
              :max="options?.constraints.maxMultiplierPpm"
              :precision="0"
          /></el-form-item>
          <el-form-item v-if="editingId === null" :label="t('aiAgent.status')"
            ><el-select-v2
              v-model="form.status"
              :options="options === null ? [] : options.statuses"
          /></el-form-item>
        </div>
        <el-form-item :label="t('aiAgent.description')"
          ><el-input
            v-model="form.description"
            type="textarea"
            :rows="2"
            :placeholder="t('aiAgent.descriptionPlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiAgent.prompt')"
          ><el-input
            v-model="form.systemPrompt"
            type="textarea"
            :rows="5"
            :placeholder="t('aiAgent.promptPlaceholder')"
            data-testid="ai-agent-prompt"
        /></el-form-item>
        <el-form-item :label="t('aiAgent.parameters')"
          ><el-input
            v-model="form.generationParams"
            type="textarea"
            :rows="3"
            :placeholder="t('aiAgent.jsonPlaceholder')"
        /></el-form-item>
        <section class="ai-agent-form__section">
          <div class="ai-agent-form__section-title">
            <h3>{{ t('aiAgent.bindings') }}</h3>
            <el-button v-if="!readonly" @click="addBinding">{{
              t('aiAgent.addBinding')
            }}</el-button>
          </div>
          <article
            v-for="(binding, index) in form.bindings"
            :key="binding.id === undefined ? index : binding.id"
            class="ai-agent-form__binding"
          >
            <div class="ai-agent-form__grid">
              <el-form-item :label="t('aiAgent.model')" required
                ><el-select-v2
                  v-model="binding.providerModelId"
                  :options="providerOptions"
                  filterable
                  remote
                  :remote-method="(keyword: string) => candidates('providerModel', keyword)"
                  :loading="candidateLoading"
                  :placeholder="t('aiAgent.modelPlaceholder')" /></el-form-item
              ><el-form-item :label="t('aiAgent.protocol')"
                ><el-select-v2
                  v-model="binding.textProtocol"
                  :options="options === null ? [] : options.textProtocols" /></el-form-item
              ><el-form-item :label="t('aiAgent.status')"
                ><el-select-v2
                  v-model="binding.status"
                  :options="options === null ? [] : options.statuses"
              /></el-form-item>
            </div>
            <el-form-item :label="t('aiAgent.bindingParameters')"
              ><el-input
                v-model="binding.parameters"
                type="textarea"
                :rows="2"
                :placeholder="t('aiAgent.jsonPlaceholder')"
            /></el-form-item>
            <el-tag v-if="binding.id === currentBinding">{{ t('aiAgent.currentLabel') }}</el-tag
            ><el-button
              v-if="!readonly"
              link
              type="danger"
              @click="form.bindings.splice(index, 1)"
              >{{ t('aiAgent.remove') }}</el-button
            >
          </article>
          <el-button
            v-if="!readonly && modelCandidates.length < modelTotal"
            link
            @click="candidates('providerModel', '', true)"
            >{{ t('aiAgent.more') }}</el-button
          >
          <el-form-item v-if="editingId === null" :label="t('aiAgent.initialCurrent')"
            ><el-select-v2
              v-model="form.activeProviderModelId"
              clearable
              :options="initialOptions"
              :placeholder="t('aiAgent.currentPlaceholder')"
          /></el-form-item>
        </section>
        <section class="ai-agent-form__section">
          <div class="ai-agent-form__section-title">
            <h3>{{ t('aiAgent.tools') }}</h3>
            <el-button v-if="!readonly" @click="addTool">{{ t('aiAgent.addTool') }}</el-button>
          </div>
          <article v-for="(tool, index) in form.tools" :key="index" class="ai-agent-form__binding">
            <div class="ai-agent-form__grid">
              <el-form-item :label="t('aiAgent.tools')"
                ><el-select-v2
                  v-model="tool.toolId"
                  :options="toolOptions"
                  filterable
                  remote
                  :remote-method="(keyword: string) => candidates('tool', keyword)"
                  :loading="candidateLoading"
                  :placeholder="t('aiAgent.toolPlaceholder')" /></el-form-item
              ><el-form-item :label="t('aiAgent.exposedName')"
                ><el-input
                  v-model="tool.exposedName"
                  :placeholder="t('aiAgent.exposedNamePlaceholder')" /></el-form-item
              ><el-form-item :label="t('aiAgent.status')"
                ><el-select-v2
                  v-model="tool.status"
                  :options="options === null ? [] : options.statuses"
              /></el-form-item>
            </div>
            <el-form-item :label="t('aiAgent.allowedParameters')"
              ><el-input
                v-model="tool.constraints"
                type="textarea"
                :rows="3"
                :placeholder="t('aiAgent.jsonPlaceholder')" /></el-form-item
            ><el-button v-if="!readonly" link type="danger" @click="form.tools.splice(index, 1)">{{
              t('aiAgent.remove')
            }}</el-button>
          </article>
          <el-button
            v-if="!readonly && toolCandidates.length < toolTotal"
            link
            @click="candidates('tool', '', true)"
            >{{ t('aiAgent.more') }}</el-button
          >
        </section>
      </el-form>
      <template #footer
        ><el-button @click="visible = false">{{ t('aiAgent.cancel') }}</el-button
        ><el-button
          v-if="!readonly"
          type="primary"
          :loading="saving"
          data-testid="ai-agent-save"
          @click="save"
          >{{ t('aiAgent.save') }}</el-button
        ></template
      >
    </AppDialog>
    <AppDialog v-model="switchVisible" width="520px" :title="t('aiAgent.switchCurrent')"
      ><el-alert :title="t('aiAgent.switchHint')" type="info" :closable="false" /><el-select-v2
        v-model="nextBinding"
        :options="switchOptions"
        :placeholder="t('aiAgent.currentPlaceholder')"
      /><template #footer
        ><el-button @click="switchVisible = false">{{ t('aiAgent.cancel') }}</el-button
        ><el-button
          type="primary"
          :loading="saving"
          data-testid="ai-agent-switch-save"
          @click="saveSwitch"
          >{{ t('aiAgent.setCurrent') }}</el-button
        ></template
      ></AppDialog
    >
    <AppDialog v-model="statusVisible" width="420px" :title="t('aiAgent.changeStatus')"
      ><el-select-v2
        v-model="nextStatus"
        :options="options === null ? [] : options.statuses"
      /><template #footer
        ><el-button @click="statusVisible = false">{{ t('aiAgent.cancel') }}</el-button
        ><el-button type="primary" :loading="saving" @click="saveStatus">{{
          t('aiAgent.save')
        }}</el-button></template
      ></AppDialog
    >
  </AppPage>
</template>
<style scoped lang="scss">
.ai-agent-form {
  &__grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0 16px;
  }
  &__section {
    border-top: 1px solid var(--el-border-color);
    padding-top: 16px;
    margin-top: 16px;
  }
  &__section-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }
  &__binding {
    padding: 16px;
    margin-bottom: 12px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: var(--el-border-radius-base);
  }
}
@media (max-width: 640px) {
  .ai-agent-form__grid {
    grid-template-columns: 1fr;
  }
}
</style>
