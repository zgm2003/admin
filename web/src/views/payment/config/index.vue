<!-- 页面：支付配置 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'
import { CirclePlus } from '@element-plus/icons-vue'
import * as api from '@/api/payment/config'
import type { PaymentConfig, PaymentConfigOptions, PaymentConfigQuery } from '@/api/payment/config'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { usePermissionStore } from '@/store/permission'
import { YesNo } from '@/enums/yesNo'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import ConfigDialog from './components/ConfigDialog/index.vue'
import type { ConfigDialogMode, ConfigForm } from './components/ConfigDialog/types'
const { t, locale } = useI18n()
const access = usePermissionStore()
const {
  options,
  loading: optionsLoading,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<PaymentConfigOptions | null>(api.getPaymentConfigOptions, () => null)
const rows = ref<PaymentConfig[]>([])
const total = ref(0)
const query = ref<PaymentConfigQuery>({ page: 1, pageSize: 20 })
const loading = ref(false)
const loadFailed = ref(false)
const busyID = ref<number | null>(null)
const filters = ref<SearchFormModel<{ keyword: string; status: number | '' }>>({
  keyword: '',
  status: '',
})
const searchFields = computed<SearchField<{ keyword: string; status: number | '' }>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('payment.config.keyword'),
    placeholder: t('payment.config.keywordPlaceholder'),
    resetValue: '',
    width: 260,
  },
  {
    key: 'status',
    type: 'select-v2',
    label: t('payment.config.status.label'),
    resetValue: '',
    options: options.value === null ? [] : options.value.statuses,
    width: 160,
  },
])
const canList = computed(() => access.hasPermission('payment:config:list'))
const canCreate = computed(() => access.hasPermission('payment:config:create'))
const canDetail = computed(() => access.hasPermission('payment:config:detail'))
const canUpdate = computed(() => access.hasPermission('payment:config:update'))
const canStatus = computed(() => access.hasPermission('payment:config:status'))
const canDelete = computed(() => access.hasPermission('payment:config:delete'))
const canTest = computed(() => access.hasPermission('payment:config:test'))
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const columns = computed<TableColumn<PaymentConfig>[]>(() => [
  { key: 'identity', label: t('payment.config.channel'), minWidth: 190 },
  { key: 'provider', label: t('payment.config.provider'), minWidth: 130 },
  { prop: 'appId', label: t('payment.config.appId'), minWidth: 165 },
  { key: 'version', label: t('payment.config.currentVersion'), width: 115 },
  { key: 'methods', label: t('payment.config.payMethods'), minWidth: 130 },
  { key: 'status', label: t('payment.config.status.label'), width: 100 },
  { prop: 'sortOrder', label: t('payment.config.sortOrder'), width: 80 },
  { key: 'actions', label: t('payment.config.actions'), minWidth: 300, fixed: 'right' },
])
const dialogVisible = ref(false)
const dialogMode = ref<ConfigDialogMode>('create')
const editingID = ref<number | null>(null)
const submitting = ref(false)
const form = ref<ConfigForm>(emptyForm())
const detailVisible = ref(false)
const detail = ref<PaymentConfig | null>(null)
let sequence = 0
let detailSequence = 0
let alive = true
function emptyForm(): ConfigForm {
  return {
    name: '',
    provider: 0,
    environment: 0,
    appId: '',
    sellerId: '',
    sortOrder: 0,
    isEnabled: YesNo.No,
    notifyUrl: '',
    payMethods: [],
    privateKey: '',
    appCertPEM: '',
    alipayCertPEM: '',
    rootCertPEM: '',
  }
}
async function load(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  const language = locale.value
  const accepted = () => alive && current === sequence && language === locale.value && canList.value
  loading.value = true
  loadFailed.value = false
  try {
    const data = await api.getPaymentConfigs(query.value)
    if (accepted()) {
      rows.value = data.list
      total.value = data.total
    }
  } catch {
    if (accepted()) {
      rows.value = []
      total.value = 0
      loadFailed.value = true
    }
  } finally {
    if (current === sequence) loading.value = false
  }
}
function search(): void {
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(filters.value.keyword.trim() === '' ? {} : { keyword: filters.value.keyword.trim() }),
    ...(filters.value.status === YesNo.No || filters.value.status === YesNo.Yes
      ? { isEnabled: filters.value.status }
      : {}),
  }
  void load()
}
function reset(): void {
  filters.value = { keyword: '', status: '' }
  search()
}
function changePagination(value: TablePaginationState): void {
  query.value = {
    ...query.value,
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}
function openCreate(): void {
  if (!canCreate.value || options.value === null || optionsLoading.value) return
  const defaults = options.value.defaults
  form.value = { ...emptyForm(), ...defaults, payMethods: [...defaults.payMethods] }
  editingID.value = null
  dialogMode.value = 'create'
  dialogVisible.value = true
}
function openEdit(row: PaymentConfig, mode: 'edit' | 'version'): void {
  if (!canUpdate.value || !row.actions.update || options.value === null || optionsLoading.value)
    return
  form.value = {
    ...emptyForm(),
    name: row.name,
    provider: row.provider,
    environment: row.environment,
    appId: row.appId,
    sellerId: row.sellerId,
    sortOrder: row.sortOrder,
    isEnabled: row.isEnabled,
    notifyUrl: row.notifyUrlTemplate,
    payMethods: [...row.payMethods],
  }
  editingID.value = row.id
  dialogMode.value = mode
  dialogVisible.value = true
}
function cancel(): void {
  if (!submitting.value) dialogVisible.value = false
}
async function save(): Promise<void> {
  if (
    submitting.value ||
    options.value === null ||
    optionsLoading.value ||
    optionsError.value !== '' ||
    (dialogMode.value === 'create' ? !canCreate.value : !canUpdate.value)
  )
    return
  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await api.createPaymentConfig({ ...form.value, payMethods: [...form.value.payMethods] })
    } else if (editingID.value !== null && dialogMode.value === 'edit') {
      await api.updatePaymentConfig(editingID.value, {
        name: form.value.name,
        sortOrder: form.value.sortOrder,
      })
    } else if (editingID.value !== null) {
      await api.publishPaymentConfigVersion(editingID.value, {
        privateKey: form.value.privateKey.trim() === '' ? null : form.value.privateKey,
        appCertPEM: form.value.appCertPEM.trim() === '' ? null : form.value.appCertPEM,
        alipayCertPEM: form.value.alipayCertPEM.trim() === '' ? null : form.value.alipayCertPEM,
        rootCertPEM: form.value.rootCertPEM.trim() === '' ? null : form.value.rootCertPEM,
        notifyUrl: form.value.notifyUrl,
        payMethods: [...form.value.payMethods],
      })
    }
    if (alive) {
      dialogVisible.value = false
      ElNotification.success({ title: t('payment.config.saved') })
      await load()
    }
  } catch {
    // request.ts owns the API error notification; keep the dialog editable.
    if (alive) await load()
  } finally {
    submitting.value = false
  }
}
async function showDetail(row: PaymentConfig): Promise<void> {
  if (!canDetail.value) return
  const current = ++detailSequence
  busyID.value = row.id
  try {
    const value = await api.getPaymentConfig(row.id)
    if (alive && current === detailSequence && canDetail.value) {
      detail.value = value
      detailVisible.value = true
    }
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyID.value = null
  }
}
async function act(row: PaymentConfig, action: 'status' | 'delete' | 'test'): Promise<void> {
  const permission =
    action === 'status'
      ? 'payment:config:status'
      : action === 'delete'
        ? 'payment:config:delete'
        : 'payment:config:test'
  if (
    busyID.value !== null ||
    !row.actions[action] ||
    !access.hasPermission(
      action === 'status'
        ? 'payment:config:status'
        : action === 'delete'
          ? 'payment:config:delete'
          : 'payment:config:test',
    )
  )
    return
  const nextStatus = row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes
  busyID.value = row.id
  try {
    if (action !== 'test') {
      try {
        await ElMessageBox.confirm(
          action === 'delete'
            ? t('payment.config.deleteConfirm', { name: row.name })
            : t('confirm.status', {
                name: row.name,
                status: t(
                  nextStatus === YesNo.Yes
                    ? 'payment.config.status.enabled'
                    : 'payment.config.status.disabled',
                ),
              }),
          t('confirm.title'),
          { type: 'warning' },
        )
      } catch {
        return
      }
    }
    if (!alive || !access.hasPermission(permission)) return
    if (action === 'status') await api.updatePaymentConfigStatus(row.id, nextStatus)
    else if (action === 'delete') await api.deletePaymentConfig(row.id)
    else await api.testPaymentConfig(row.id)
    if (alive) {
      ElNotification.success({
        title: t(action === 'test' ? 'payment.config.validated' : 'payment.config.saved'),
      })
      await load()
    }
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyID.value = null
  }
}
watch(dialogVisible, (open) => {
  if (!open) {
    form.value = emptyForm()
    editingID.value = null
  }
})
watch(
  [canList, locale],
  () => {
    ++sequence
    ++detailSequence
    rows.value = []
    total.value = 0
    loading.value = false
    loadFailed.value = false
    detailVisible.value = false
    detail.value = null
    if (canList.value) void load()
  },
  { immediate: true },
)
watch([canCreate, canUpdate], () => {
  if (!submitting.value && (dialogMode.value === 'create' ? !canCreate.value : !canUpdate.value))
    dialogVisible.value = false
})
onBeforeUnmount(() => {
  alive = false
  ++sequence
  ++detailSequence
  form.value = emptyForm()
})
</script>
<template>
  <AppPage class="payment-config-page">
    <AppSearch
      v-model="filters"
      :fields="searchFields"
      class="management-page__filters"
      @query="search"
      @reset="reset"
    />
    <el-alert
      v-if="!canList"
      :title="t('payment.config.noList')"
      type="warning"
      :closable="false"
    />
    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: PaymentConfig) => String(row.id)"
      :result-state="loadFailed ? 'error' : loading ? 'loading' : rows.length ? 'success' : 'empty'"
      @refresh="load"
      @update:pagination="changePagination"
    >
      <template #toolbar-left
        ><el-button
          v-if="canCreate"
          type="primary"
          :icon="CirclePlus"
          :disabled="optionsLoading || options === null || submitting"
          data-testid="payment-config-create"
          @click="openCreate"
          >{{ t('payment.config.create') }}</el-button
        ></template
      >
      <template #toolbar-right>
        <el-button v-if="optionsError" @click="reloadOptions">{{
          t('payment.config.retry')
        }}</el-button>
      </template>
      <template #cell-identity="{ row }: { row: PaymentConfig }">
        <strong>{{ row.name }}</strong>
      </template>
      <template #cell-provider="{ row }: { row: PaymentConfig }"
        ><div>
          <div>{{ row.presentation.provider }}</div>
          <div>
            <el-tag size="small" effect="plain">{{ row.presentation.environment }}</el-tag>
          </div>
        </div></template
      >
      <template #cell-version="{ row }: { row: PaymentConfig }"
        ><code>v{{ row.currentVersion }}</code></template
      >
      <template #cell-methods="{ row }: { row: PaymentConfig }"
        ><div class="payment-config-page__tags">
          <el-tag
            v-for="method in row.presentation.payMethods"
            :key="method.value"
            size="small"
            effect="plain"
            style="margin-right: 10px"
            >{{ method.label }}</el-tag
          >
        </div></template
      >
      <template #cell-status="{ row }: { row: PaymentConfig }"
        ><el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'">{{
          row.presentation.status
        }}</el-tag></template
      >
      <template #cell-actions="{ row }: { row: PaymentConfig }">
        <el-button
          v-if="canDetail"
          link
          type="primary"
          :disabled="busyID !== null"
          data-testid="payment-config-detail"
          @click="showDetail(row)"
          >{{ t('payment.config.detail') }}</el-button
        >
        <el-button
          v-if="canUpdate && row.actions.update"
          link
          type="primary"
          :disabled="optionsLoading || options === null || submitting"
          data-testid="payment-config-edit"
          @click="openEdit(row, 'edit')"
          >{{ t('payment.config.edit') }}</el-button
        >
        <el-button
          v-if="canUpdate && row.actions.update"
          link
          type="primary"
          :disabled="optionsLoading || options === null || submitting"
          data-testid="payment-config-version"
          @click="openEdit(row, 'version')"
          >{{ t('payment.config.version') }}</el-button
        >
        <el-button
          v-if="canStatus && row.actions.status"
          link
          :type="row.isEnabled === YesNo.Yes ? 'warning' : 'success'"
          :disabled="busyID !== null"
          data-testid="payment-config-status"
          @click="act(row, 'status')"
          >{{
            row.isEnabled === YesNo.Yes ? t('payment.config.disable') : t('payment.config.enable')
          }}</el-button
        >
        <el-button
          v-if="canTest && row.actions.test"
          link
          type="primary"
          :disabled="busyID !== null"
          data-testid="payment-config-test"
          @click="act(row, 'test')"
          >{{ t('payment.config.test') }}</el-button
        >
        <el-button
          v-if="canDelete && row.actions.delete"
          link
          type="danger"
          :disabled="busyID !== null"
          data-testid="payment-config-delete"
          @click="act(row, 'delete')"
          >{{ t('payment.config.delete') }}</el-button
        >
      </template>
    </AppTable>
    <ConfigDialog
      v-if="options !== null"
      v-model="dialogVisible"
      v-model:form="form"
      :mode="dialogMode"
      :options="options"
      :submitting="submitting || optionsLoading"
      @save="save"
      @cancel="cancel"
    />
    <AppDialog
      v-model="detailVisible"
      :title="t('payment.config.detail')"
      width="760px"
      :append-to-body="false"
    >
      <template v-if="detail">
        <el-descriptions :column="1" border>
          <el-descriptions-item :label="t('payment.config.name')">{{
            detail.name
          }}</el-descriptions-item>
          <el-descriptions-item :label="t('payment.config.provider')"
            >{{ detail.presentation.provider }} /
            {{ detail.presentation.environment }}</el-descriptions-item
          >
          <el-descriptions-item :label="t('payment.config.appId')">{{
            detail.appId
          }}</el-descriptions-item>
          <el-descriptions-item :label="t('payment.config.sellerId')">{{
            detail.sellerId
          }}</el-descriptions-item>
          <el-descriptions-item :label="t('payment.config.currentVersion')">{{
            detail.currentVersion
          }}</el-descriptions-item>
          <el-descriptions-item :label="t('payment.config.notifyUrl')"
            ><span class="payment-config-page__url">{{
              detail.notifyUrl
            }}</span></el-descriptions-item
          >
        </el-descriptions>
        <section
          v-for="(cert, index) in [
            detail.certificates.app,
            detail.certificates.alipay,
            ...detail.certificates.roots,
          ]"
          :key="index"
          class="payment-config-page__certificate"
        >
          <h3>
            {{
              t(
                index === 0
                  ? 'payment.config.appCertPEM'
                  : index === 1
                    ? 'payment.config.alipayCertPEM'
                    : 'payment.config.rootCertPEM',
              )
            }}
          </h3>
          <p>{{ cert.subject }}</p>
          <p>{{ cert.issuer }}</p>
          <code>{{ cert.serialNumber }}</code>
          <p>{{ cert.notBefore }} — {{ cert.notAfter }}</p>
        </section>
      </template>
    </AppDialog>
  </AppPage>
</template>
<style scoped lang="scss"></style>
