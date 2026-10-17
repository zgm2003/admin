<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FormInstance, FormRules } from 'element-plus'
import { CirclePlus } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { ElNotification } from 'element-plus/es/components/notification/index'

import * as api from '@/api/payment/rechargePackage'
import type {
  RechargePackage,
  RechargePackageOptions,
  RechargePackageQuery,
} from '@/api/payment/rechargePackage'
import { useLocalizedOptions } from '@/composables/useLocalizedOptions'
import { YesNo } from '@/enums/yesNo'
import { usePermissionStore } from '@/store/permission'
import type { TableColumn, TablePaginationState } from '@/components/AppTable'
import type { SearchField, SearchFormModel } from '@/components/AppSearch'
import AppDialog from '@/components/AppDialog/index.vue'

const { t, locale } = useI18n()
const access = usePermissionStore()
const {
  options,
  loading: optionsLoading,
  error: optionsError,
  reload: reloadOptions,
} = useLocalizedOptions<RechargePackageOptions | null>(api.getRechargePackageOptions, () => null)

interface FormState {
  name: string
  amount: string
  badge: string
  sortOrder: number
  isEnabled: YesNo
}

const emptyForm = (): FormState => ({
  name: '',
  amount: '',
  badge: '',
  sortOrder: 0,
  isEnabled: YesNo.No,
})
const rows = ref<RechargePackage[]>([])
const total = ref(0)
const query = ref<RechargePackageQuery>({ page: 1, pageSize: 20 })
const filters = ref<SearchFormModel<{ keyword: string; isEnabled: YesNo | '' }>>({
  keyword: '',
  isEnabled: '',
})
const loading = ref(false)
const loadError = ref('')
const busyID = ref<number | null>(null)
const dialogVisible = ref(false)
const editingID = ref<number | null>(null)
const submitting = ref(false)
const formError = ref('')
const form = ref<FormState>(emptyForm())
const formRef = ref<FormInstance>()
const formRules = computed<FormRules<FormState>>(() => ({
  name: [
    {
      required: true,
      whitespace: true,
      message: t('payment.rechargePackage.nameRule'),
      trigger: 'blur',
    },
  ],
  amount: [
    {
      required: true,
      whitespace: true,
      message: t('payment.rechargePackage.amountRequired'),
      trigger: 'blur',
    },
  ],
}))
let sequence = 0
let alive = true

const canList = computed(() => access.hasPermission('payment:rechargePackage:list'))
const canCreate = computed(() => access.hasPermission('payment:rechargePackage:create'))
const canUpdate = computed(() => access.hasPermission('payment:rechargePackage:update'))
const canStatus = computed(() => access.hasPermission('payment:rechargePackage:status'))
const canDelete = computed(() => access.hasPermission('payment:rechargePackage:delete'))
const pagination = computed<TablePaginationState>(() => ({
  currentPage: query.value.page,
  pageSize: query.value.pageSize,
  total: total.value,
}))
const searchFields = computed<SearchField<{ keyword: string; isEnabled: YesNo | '' }>[]>(() => [
  {
    key: 'keyword',
    type: 'input',
    label: t('payment.rechargePackage.keyword'),
    placeholder: t('payment.rechargePackage.keywordPlaceholder'),
    resetValue: '',
    width: 260,
  },
  {
    key: 'isEnabled',
    type: 'select-v2',
    label: t('payment.rechargePackage.status'),
    placeholder: t('payment.rechargePackage.allStatuses'),
    resetValue: '',
    options: options.value?.statuses ?? [],
    width: 160,
  },
])
const columns = computed<TableColumn<RechargePackage>[]>(() => [
  { prop: 'name', label: t('payment.rechargePackage.name'), minWidth: 180 },
  { prop: 'amount', label: t('payment.rechargePackage.amount'), width: 130 },
  { prop: 'badge', label: t('payment.rechargePackage.badge'), width: 140 },
  { prop: 'sortOrder', label: t('payment.rechargePackage.sortOrder'), width: 90 },
  { key: 'status', label: t('payment.rechargePackage.status'), width: 110 },
  { key: 'actions', label: t('payment.rechargePackage.actions'), width: 230, fixed: 'right' },
])

function state(): 'loading' | 'error' | 'empty' | 'success' {
  if (loading.value) return 'loading'
  if (loadError.value) return 'error'
  return rows.value.length > 0 ? 'success' : 'empty'
}
async function load(): Promise<void> {
  if (!canList.value) return
  const current = ++sequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await api.getRechargePackages(query.value)
    if (alive && current === sequence && canList.value) {
      rows.value = result.list
      total.value = result.total
    }
  } catch {
    if (alive && current === sequence) {
      rows.value = []
      total.value = 0
      loadError.value = t('payment.rechargePackage.requestFailed')
    }
  } finally {
    if (alive && current === sequence) loading.value = false
  }
}
function search(): void {
  query.value = {
    page: 1,
    pageSize: query.value.pageSize,
    ...(filters.value.keyword.trim() ? { keyword: filters.value.keyword.trim() } : {}),
    ...(filters.value.isEnabled === '' ? {} : { isEnabled: filters.value.isEnabled }),
  }
  void load()
}
function reset(): void {
  filters.value = { keyword: '', isEnabled: '' }
  search()
}
function updatePagination(value: TablePaginationState): void {
  query.value = {
    ...query.value,
    page: value.pageSize === query.value.pageSize ? value.currentPage : 1,
    pageSize: value.pageSize,
  }
  void load()
}
function openCreate(): void {
  if (!canCreate.value || options.value === null) return
  editingID.value = null
  form.value = { ...emptyForm(), ...options.value.defaults }
  formError.value = ''
  dialogVisible.value = true
}
function openEdit(row: RechargePackage): void {
  if (!canUpdate.value || !row.actions.update) return
  editingID.value = row.id
  form.value = {
    name: row.name,
    amount: row.amount,
    badge: row.badge ?? '',
    sortOrder: row.sortOrder,
    isEnabled: row.isEnabled,
  }
  formError.value = ''
  dialogVisible.value = true
}
function validateForm(): string {
  const value = form.value
  if (value.name.trim().length > (options.value?.limits.name ?? 128)) {
    return t('payment.rechargePackage.nameTooLong')
  }
  if (!/^\d+(\.\d{1,2})?$/.test(value.amount.trim()) || Number(value.amount) <= 0) {
    return t('payment.rechargePackage.amountRule')
  }
  if (value.badge.trim().length > (options.value?.limits.badge ?? 32)) {
    return t('payment.rechargePackage.badgeRule')
  }
  return ''
}
async function save(): Promise<void> {
  if (submitting.value || options.value === null) return
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  const error = validateForm()
  if (error) {
    formError.value = error
    return
  }
  submitting.value = true
  formError.value = ''
  try {
    const input = {
      name: form.value.name.trim(),
      amount: form.value.amount.trim(),
      badge: form.value.badge.trim() || null,
      sortOrder: form.value.sortOrder,
    }
    if (editingID.value === null) {
      await api.createRechargePackage({ ...input, isEnabled: form.value.isEnabled })
    } else {
      await api.updateRechargePackage(editingID.value, input)
    }
    dialogVisible.value = false
    ElNotification.success({ title: t('payment.rechargePackage.saved') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    submitting.value = false
  }
}
async function mutate(row: RechargePackage, action: 'status' | 'delete'): Promise<void> {
  const allowed =
    action === 'status'
      ? canStatus.value && row.actions.status
      : canDelete.value && row.actions.delete
  if (!allowed || busyID.value !== null) return
  try {
    await ElMessageBox.confirm(
      t(`payment.rechargePackage.${action}Confirm`, { name: row.name }),
      t('confirm.title'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  busyID.value = row.id
  try {
    if (action === 'status') {
      await api.updateRechargePackageStatus(
        row.id,
        row.isEnabled === YesNo.Yes ? YesNo.No : YesNo.Yes,
      )
    } else {
      await api.deleteRechargePackage(row.id)
    }
    ElNotification.success({ title: t('payment.rechargePackage.saved') })
    await load()
  } catch {
    // request.ts owns the API error notification.
  } finally {
    busyID.value = null
  }
}

watch(
  [canList, locale],
  () => {
    sequence++
    rows.value = []
    total.value = 0
    if (canList.value) void load()
  },
  { immediate: true },
)
watch(dialogVisible, (visible) => {
  if (!visible) {
    form.value = emptyForm()
    editingID.value = null
    formError.value = ''
  }
})
onBeforeUnmount(() => {
  alive = false
  sequence++
})
</script>

<template>
  <AppPage class="payment-recharge-package-page">
    <AppSearch
      v-model="filters"
      class="management-page__filters"
      :fields="searchFields"
      :query-label="t('search.query')"
      :reset-label="t('search.reset')"
      @query="search"
      @reset="reset"
    />
    <div v-if="optionsError">
      <el-button text @click="reloadOptions">{{ t('payment.rechargePackage.retry') }}</el-button>
    </div>

    <AppTable
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: RechargePackage) => String(row.id)"
      :result-state="state()"
      :status-message="loadError"
      :aria-label="t('payment.rechargePackage.title')"
      @refresh="load"
      @update:pagination="updatePagination"
    >
      <template #toolbar-left>
        <el-button
          v-if="canCreate"
          type="primary"
          :icon="CirclePlus"
          :disabled="optionsLoading || options === null || submitting"
          data-testid="payment-recharge-package-create"
          @click="openCreate"
        >
          {{ t('payment.rechargePackage.create') }}
        </el-button>
      </template>
      <template #cell-status="{ row }: { row: RechargePackage }">
        <el-tag :type="row.isEnabled === YesNo.Yes ? 'success' : 'info'">{{
          row.presentation.status
        }}</el-tag>
      </template>
      <template #cell-actions="{ row }: { row: RechargePackage }">
        <el-button
          v-if="canUpdate && row.actions.update"
          link
          type="primary"
          :disabled="busyID !== null"
          @click="openEdit(row)"
        >
          {{ t('payment.rechargePackage.edit') }}
        </el-button>
        <el-button
          v-if="canStatus && row.actions.status"
          link
          type="primary"
          :disabled="busyID !== null"
          @click="mutate(row, 'status')"
        >
          {{ t('payment.rechargePackage.toggleStatus') }}
        </el-button>
        <el-button
          v-if="canDelete && row.actions.delete"
          link
          type="danger"
          :disabled="busyID !== null"
          @click="mutate(row, 'delete')"
        >
          {{ t('payment.rechargePackage.delete') }}
        </el-button>
      </template>
    </AppTable>

    <AppDialog
      v-model="dialogVisible"
      :title="
        t(
          editingID === null
            ? 'payment.rechargePackage.createTitle'
            : 'payment.rechargePackage.editTitle',
        )
      "
      width="560px"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="formRules"
        label-position="top"
        class="payment-recharge-package-form"
        @submit.prevent="save"
      >
        <el-form-item prop="name" :label="t('payment.rechargePackage.name')">
          <el-input
            v-model="form.name"
            :placeholder="t('payment.rechargePackage.namePlaceholder')"
            data-testid="payment-recharge-package-name"
          />
        </el-form-item>
        <el-form-item prop="amount" :label="t('payment.rechargePackage.amount')">
          <el-input
            v-model="form.amount"
            inputmode="decimal"
            :placeholder="t('payment.rechargePackage.amountPlaceholder')"
            data-testid="payment-recharge-package-amount"
          />
        </el-form-item>
        <el-form-item :label="t('payment.rechargePackage.badge')">
          <el-input
            v-model="form.badge"
            :placeholder="t('payment.rechargePackage.badgePlaceholder')"
            data-testid="payment-recharge-package-badge"
          />
        </el-form-item>
        <div class="payment-recharge-package-form__grid">
          <el-form-item :label="t('payment.rechargePackage.sortOrder')">
            <el-input-number
              v-model="form.sortOrder"
              :min="options?.limits.sortMinimum"
              :max="options?.limits.sortMaximum"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item v-if="editingID === null" :label="t('payment.rechargePackage.status')">
            <el-select-v2 v-model="form.isEnabled" :options="options?.statuses ?? []" />
          </el-form-item>
        </div>
        <el-alert v-if="formError" :title="formError" type="error" :closable="false" />
      </el-form>
      <template #footer>
        <el-button :disabled="submitting" @click="dialogVisible = false">{{
          t('payment.rechargePackage.cancel')
        }}</el-button>
        <el-button
          type="primary"
          :loading="submitting"
          data-testid="payment-recharge-package-save"
          @click="save"
          >{{ t('payment.rechargePackage.save') }}</el-button
        >
      </template>
    </AppDialog>
  </AppPage>
</template>

<style scoped lang="scss">
.payment-recharge-package-page {
  min-width: 0;
}
.payment-recharge-package-form__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
@media (max-width: 680px) {
  .payment-recharge-package-form__grid {
    grid-template-columns: 1fr;
  }
}
</style>
