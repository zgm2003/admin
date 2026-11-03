<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import * as api from '@/api/ai/provider'
import type { JsonValue, Provider, ProviderOptions, ProviderUpdate } from '@/api/ai/provider'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { usePermissionStore } from '@/store/permission'
import ModelsDialog from './components/ModelsDialog/index.vue'

const { t, locale } = useI18n()
const access = usePermissionStore()
const allowed = (action: string) => access.hasPermission(`ai:provider:${action}`)
const canList = computed(() => allowed('list'))
const {
  options,
  loading: optionsLoading,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<ProviderOptions | null>(api.getProviderOptions, () => null)
const rows = ref<Provider[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const pagination = computed(() => ({
  currentPage: page.value,
  pageSize: pageSize.value,
  total: total.value,
}))
const columns = computed<TableColumn<Provider>[]>(() => [
  { prop: 'code', label: t('aiProvider.code'), minWidth: 130 },
  { prop: 'name', label: t('aiProvider.name'), minWidth: 150 },
  { prop: 'description', label: t('aiProvider.description'), minWidth: 180 },
  { prop: 'currentVersion', label: t('aiProvider.version'), width: 110 },
  { key: 'status', label: t('aiProvider.status'), width: 95 },
  { key: 'actions', label: t('common.actions'), minWidth: 430, fixed: 'right' },
])
let sequence = 0
let detailSequence = 0
let alive = true
async function load(): Promise<void> {
  if (!canList.value) return
  const token = ++sequence
  loading.value = true
  loadFailed.value = false
  try {
    const result = await api.getProviders({ page: page.value, pageSize: pageSize.value })
    if (alive && token === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && token === sequence) {
      rows.value = []
      total.value = 0
      loadFailed.value = true
    }
  } finally {
    if (token === sequence) loading.value = false
  }
}
function changePage(value: TablePaginationState): void {
  page.value = value.pageSize === pageSize.value ? value.currentPage : 1
  pageSize.value = value.pageSize
  void load()
}
function statusLabel(status: number): string {
  return options.value?.statuses.find((v) => v.value === status)?.label ?? String(status)
}
const visible = ref(false)
const editingId = ref<string | null>(null)
const readonly = ref(false)
const saving = ref(false)
const formError = ref('')
const form = ref({
  code: '',
  name: '',
  description: '',
  baseUrl: '',
  apiKey: '',
  headers: '',
  mediaHosts: '',
  requestTimeoutMs: 0,
  status: 0,
})
function openCreate(): void {
  if (!allowed('create') || options.value === null) return
  ++detailSequence
  editingId.value = null
  readonly.value = false
  formError.value = ''
  form.value = {
    code: '',
    name: '',
    description: '',
    baseUrl: '',
    apiKey: '',
    headers: '',
    mediaHosts: '',
    requestTimeoutMs: options.value.defaults.requestTimeoutMs,
    status: options.value.defaults.status,
  }
  visible.value = true
}
async function openDetail(row: Provider, edit: boolean): Promise<void> {
  if (!allowed('detail') || (edit && !allowed('update'))) return
  const token = ++detailSequence
  try {
    const detail = await api.getProvider(row.id)
    if (!alive || token !== detailSequence || !allowed('detail') || (edit && !allowed('update')))
      return
    editingId.value = row.id
    readonly.value = !edit
    formError.value = ''
    form.value = {
      code: detail.provider.code,
      name: detail.provider.name,
      description: detail.provider.description,
      baseUrl: detail.baseUrl,
      apiKey: '',
      headers: '',
      mediaHosts: detail.mediaHostAllowlist.join('\n'),
      requestTimeoutMs: detail.requestTimeoutMs,
      status: detail.provider.status,
    }
    visible.value = true
  } catch {
    /* Request layer owns the error notification. */
  }
}
async function save(): Promise<void> {
  if (
    saving.value ||
    readonly.value ||
    options.value === null ||
    optionsLoading.value ||
    optionsError.value !== '' ||
    !allowed(editingId.value === null ? 'create' : 'update')
  )
    return
  formError.value = ''
  if (
    !form.value.code.trim() ||
    !form.value.name.trim() ||
    !form.value.baseUrl.trim() ||
    (editingId.value === null && !form.value.apiKey.trim())
  ) {
    formError.value = t('aiProvider.required')
    return
  }
  let headers: Record<string, string> | null = null
  try {
    if (form.value.headers.trim() !== '') {
      const parsed: JsonValue = JSON.parse(form.value.headers)
      if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed))
        throw new Error('headers')
      headers = {}
      for (const [key, value] of Object.entries(parsed)) {
        if (typeof value !== 'string') throw new Error('header')
        headers[key] = value
      }
    }
  } catch {
    formError.value = t('aiProvider.invalidJson')
    return
  }
  const input: ProviderUpdate = {
    name: form.value.name,
    description: form.value.description,
    baseUrl: form.value.baseUrl,
    apiKey: form.value.apiKey,
    headers,
    mediaHostAllowlist: form.value.mediaHosts
      .split('\n')
      .map((v) => v.trim())
      .filter((v) => v !== ''),
    requestTimeoutMs: form.value.requestTimeoutMs,
  }
  saving.value = true
  try {
    if (editingId.value === null)
      await api.createProvider({ ...input, code: form.value.code, status: form.value.status })
    else await api.updateProvider(editingId.value, input)
    if (alive) {
      visible.value = false
      ElNotification.success({ title: t('aiProvider.saved') })
      await load()
    }
  } catch {
    /* Preserve the form for retry. */
  } finally {
    saving.value = false
  }
}
const statusVisible = ref(false)
const statusId = ref<string | null>(null)
const nextStatus = ref(0)
function openStatus(row: Provider): void {
  if (!allowed('status')) return
  statusId.value = row.id
  nextStatus.value = row.status
  statusVisible.value = true
}
async function saveStatus(): Promise<void> {
  if (!allowed('status') || statusId.value === null || saving.value) return
  saving.value = true
  try {
    await api.updateProviderStatus(statusId.value, nextStatus.value)
    statusVisible.value = false
    await load()
  } catch {
    /* Request layer notifies. */
  } finally {
    saving.value = false
  }
}
const busyId = ref<string | null>(null)
async function remove(row: Provider): Promise<void> {
  if (!allowed('delete') || busyId.value !== null) return
  try {
    await ElMessageBox.confirm(t('aiProvider.confirmDelete'), t('aiProvider.delete'), {
      type: 'warning',
    })
  } catch {
    return
  }
  if (!allowed('delete')) return
  busyId.value = row.id
  try {
    await api.deleteProvider(row.id)
    await load()
  } catch {
    /* Request layer notifies. */
  } finally {
    busyId.value = null
  }
}
async function test(row: Provider): Promise<void> {
  if (!allowed('test') || busyId.value !== null) return
  busyId.value = row.id
  try {
    const result = await api.testProvider(row.id)
    if (alive)
      ElNotification.success({ title: t('aiProvider.tested', { count: result.modelCount }) })
  } catch {
    /* Request layer notifies. */
  } finally {
    busyId.value = null
  }
}
const modelsVisible = ref(false)
const selectedProvider = ref<Provider | null>(null)
function openModels(row: Provider): void {
  if (!allowed('detail') && !allowed('sync')) return
  selectedProvider.value = row
  modelsVisible.value = true
}
watch(
  [canList, locale],
  () => {
    ++sequence
    rows.value = []
    total.value = 0
    if (canList.value) void load()
  },
  { immediate: true },
)
watch(
  () => access.permissionCodes,
  () => {
    ++detailSequence
    if (
      visible.value &&
      !allowed(readonly.value ? 'detail' : editingId.value === null ? 'create' : 'update')
    )
      visible.value = false
  },
  { deep: true },
)
onBeforeUnmount(() => {
  alive = false
  ++sequence
  ++detailSequence
})
</script>

<template>
  <AppPage class="ai-provider-page">
    <el-alert
      v-if="optionsError"
      :title="t('aiProvider.optionsFailed')"
      type="error"
      :closable="false"
      ><el-button link @click="reloadOptions">{{ t('aiProvider.retry') }}</el-button></el-alert
    >
    <el-alert v-if="loadFailed" :title="t('aiProvider.loadFailed')" type="error" :closable="false"
      ><el-button data-testid="ai-provider-retry" link @click="load">{{
        t('aiProvider.retry')
      }}</el-button></el-alert
    >
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :result-state="loadFailed ? 'error' : loading ? 'loading' : rows.length ? 'success' : 'empty'"
      :status-message="loadFailed ? t('aiProvider.loadFailed') : ''"
      :aria-label="t('aiProvider.title')"
      @refresh="load"
      @update:pagination="changePage"
    >
      <template #toolbar-left
        ><el-button
          v-if="allowed('create')"
          data-testid="ai-provider-create"
          type="primary"
          :disabled="optionsLoading || options === null"
          @click="openCreate"
          >{{ t('aiProvider.create') }}</el-button
        ></template
      >
      <template #cell-status="{ row }: { row: Provider }">{{ statusLabel(row.status) }}</template>
      <template #cell-actions="{ row }: { row: Provider }">
        <el-button v-if="allowed('detail')" link @click="openDetail(row, false)">{{
          t('aiProvider.detail')
        }}</el-button>
        <el-button
          v-if="allowed('detail') && allowed('update')"
          data-testid="ai-provider-edit"
          link
          type="primary"
          :disabled="options === null"
          @click="openDetail(row, true)"
          >{{ t('aiProvider.edit') }}</el-button
        >
        <el-button
          v-if="allowed('detail') || allowed('sync')"
          data-testid="ai-provider-models"
          link
          type="primary"
          @click="openModels(row)"
          >{{ t('aiProvider.models') }}</el-button
        >
        <el-button v-if="allowed('test')" link :loading="busyId === row.id" @click="test(row)">{{
          t('aiProvider.test')
        }}</el-button>
        <el-button
          v-if="allowed('status')"
          link
          :disabled="options === null"
          @click="openStatus(row)"
          >{{ t('aiProvider.changeStatus') }}</el-button
        >
        <el-button
          v-if="allowed('delete')"
          link
          type="danger"
          :disabled="busyId !== null"
          @click="remove(row)"
          >{{ t('aiProvider.delete') }}</el-button
        >
      </template>
      <template #empty
        ><el-empty v-if="!loadFailed" :description="t('aiProvider.empty')"
      /></template>
    </AppTable>
    <AppDialog
      v-model="visible"
      :title="
        t(
          readonly
            ? 'aiProvider.detail'
            : editingId === null
              ? 'aiProvider.create'
              : 'aiProvider.edit',
        )
      "
      width="700px"
      :close-on-press-escape="!saving"
    >
      <el-alert v-if="formError" :title="formError" type="error" :closable="false" />
      <el-form label-position="top" :disabled="readonly || saving">
        <el-form-item :label="t('aiProvider.code')" required
          ><el-input
            v-model="form.code"
            :disabled="editingId !== null"
            :placeholder="t('aiProvider.codePlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiProvider.name')" required
          ><el-input
            v-model="form.name"
            :maxlength="options?.constraints.maxNameLength"
            :placeholder="t('aiProvider.namePlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiProvider.description')"
          ><el-input
            v-model="form.description"
            type="textarea"
            :maxlength="options?.constraints.maxDescriptionLength"
            :placeholder="t('aiProvider.descriptionPlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiProvider.baseUrl')" required
          ><el-input v-model="form.baseUrl" :placeholder="t('aiProvider.urlPlaceholder')"
        /></el-form-item>
        <el-form-item
          v-if="!readonly"
          :label="t('aiProvider.apiKey')"
          :required="editingId === null"
          ><el-input
            v-model="form.apiKey"
            data-testid="ai-provider-api-key"
            type="password"
            show-password
            autocomplete="new-password"
            :placeholder="t('aiProvider.keyPlaceholder')"
        /></el-form-item>
        <el-alert
          v-if="!readonly"
          :title="t('aiProvider.secretHint')"
          type="info"
          :closable="false"
        />
        <el-form-item v-if="!readonly" :label="t('aiProvider.headers')"
          ><el-input
            v-model="form.headers"
            type="textarea"
            :placeholder="t('aiProvider.headersPlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiProvider.mediaHosts')"
          ><el-input
            v-model="form.mediaHosts"
            type="textarea"
            :placeholder="t('aiProvider.hostsPlaceholder')"
        /></el-form-item>
        <el-form-item :label="t('aiProvider.timeout')"
          ><el-input-number
            v-model="form.requestTimeoutMs"
            :min="options?.constraints.requestTimeoutMs.min"
            :max="options?.constraints.requestTimeoutMs.max"
        /></el-form-item>
        <el-form-item v-if="editingId === null" :label="t('aiProvider.status')"
          ><el-select-v2 v-model="form.status" :options="options === null ? [] : options.statuses"
        /></el-form-item>
      </el-form>
      <template #footer
        ><el-button :disabled="saving" @click="visible = false">{{
          t('aiProvider.cancel')
        }}</el-button
        ><el-button
          v-if="!readonly"
          data-testid="ai-provider-save"
          type="primary"
          :loading="saving"
          :disabled="options === null || optionsLoading"
          @click="save"
          >{{ t('aiProvider.save') }}</el-button
        ></template
      >
    </AppDialog>
    <AppDialog v-model="statusVisible" :title="t('aiProvider.changeStatus')" width="420px"
      ><el-select-v2
        v-model="nextStatus"
        :options="options === null ? [] : options.statuses"
      /><template #footer
        ><el-button :disabled="saving" @click="statusVisible = false">{{
          t('aiProvider.cancel')
        }}</el-button
        ><el-button
          type="primary"
          :loading="saving"
          :disabled="options === null"
          @click="saveStatus"
          >{{ t('aiProvider.save') }}</el-button
        ></template
      ></AppDialog
    >
    <ModelsDialog
      v-if="selectedProvider"
      v-model="modelsVisible"
      :provider-id="selectedProvider.id"
      :provider-name="selectedProvider.name"
    />
  </AppPage>
</template>

<style scoped lang="scss">
.ai-provider-page {
  min-width: 0;
  .el-alert {
    margin-bottom: 12px;
  }
}
</style>
