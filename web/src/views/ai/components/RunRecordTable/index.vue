<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AppTable from '@/components/AppTable/index.vue'
import type { TableColumn } from '@/components/AppTable'
import type { BackendOption } from '@/types/option'
import type {
  RunRecordKind,
  RunRequest,
  RunToolCall,
  RunCharge,
  RunChargeItem,
  RunOptions,
} from '@/api/ai/run'

const props = withDefaults(
  defineProps<{
    kind: RunRecordKind
    requests?: RunRequest[]
    tools?: RunToolCall[]
    charges?: RunCharge[]
    chargeItems?: RunChargeItem[]
    options: RunOptions | null
    loading?: boolean
  }>(),
  {
    requests: () => [],
    tools: () => [],
    charges: () => [],
    chargeItems: () => [],
    loading: false,
  },
)
defineEmits<{ refresh: [] }>()
const { t } = useI18n()
function statusLabel(items: BackendOption<number>[] | undefined, value: number): string {
  return items?.find((item) => item.value === value)?.label ?? String(value)
}
const requestColumns = computed<TableColumn<RunRequest>[]>(() => [
  { prop: 'id', label: t('aiRun.requestId'), minWidth: 140 },
  { prop: 'sequence', label: t('aiRun.sequence'), width: 80 },
  {
    prop: 'status',
    label: t('aiRun.recordStatus'),
    minWidth: 100,
    formatter: (_row, value) => statusLabel(props.options?.requestStatuses, value),
  },
  {
    prop: 'usageStatus',
    label: t('aiRun.usageStatus'),
    minWidth: 120,
    formatter: (_row, value) => statusLabel(props.options?.requestUsageStatuses, value),
  },
  {
    prop: 'startedAt',
    label: t('aiRun.startedAt'),
    minWidth: 180,
    formatter: (_row, value) => value ?? '—',
  },
  {
    prop: 'finishedAt',
    label: t('aiRun.finishedAt'),
    minWidth: 180,
    formatter: (_row, value) => value ?? '—',
  },
  { key: 'payload', label: t('aiRun.payload'), expand: true },
])
const toolColumns = computed<TableColumn<RunToolCall>[]>(() => [
  { prop: 'id', label: t('aiRun.toolCallId'), minWidth: 140 },
  { prop: 'exposedName', label: t('aiRun.toolName'), minWidth: 160 },
  { prop: 'providerRequestId', label: t('aiRun.requestId'), minWidth: 140 },
  {
    prop: 'status',
    label: t('aiRun.recordStatus'),
    minWidth: 100,
    formatter: (_row, value) => statusLabel(props.options?.toolCallStatuses, value),
  },
  {
    prop: 'startedAt',
    label: t('aiRun.startedAt'),
    minWidth: 180,
    formatter: (_row, value) => value ?? '—',
  },
  {
    prop: 'finishedAt',
    label: t('aiRun.finishedAt'),
    minWidth: 180,
    formatter: (_row, value) => value ?? '—',
  },
])
const chargeColumns = computed<TableColumn<RunCharge>[]>(() => [
  { prop: 'id', label: t('aiRun.chargeId'), minWidth: 140 },
  {
    prop: 'status',
    label: t('aiRun.billingStatus'),
    minWidth: 120,
    formatter: (_row, value) => statusLabel(props.options?.billingStatuses, value),
  },
  { prop: 'currency', label: t('aiRun.currency'), width: 90 },
  {
    prop: 'actualUnits',
    label: t('aiRun.actualUnits'),
    minWidth: 210,
    formatter: (_row, value) => (value === null ? t('aiRun.amountUnknown') : value),
  },
  {
    prop: 'usageComplete',
    label: t('aiRun.usageComplete'),
    minWidth: 100,
    formatter: (_row, value) => statusLabel(props.options?.usageCompleteness, value),
  },
  {
    prop: 'transactionId',
    label: t('aiRun.transactionId'),
    minWidth: 140,
    formatter: (_row, value) => value ?? '—',
  },
  {
    prop: 'settledAt',
    label: t('aiRun.settledAt'),
    minWidth: 180,
    formatter: (_row, value) => value ?? '—',
  },
  { key: 'pricing', label: t('aiRun.pricing'), expand: true },
])
const chargeItemColumns = computed<TableColumn<RunChargeItem>[]>(() => [
  { prop: 'usageKey', label: t('aiRun.usageKey'), minWidth: 140 },
  {
    prop: 'operation',
    label: t('aiRun.operation'),
    minWidth: 140,
    formatter: (_row, value) =>
      props.options?.operations.find((item) => item.value === value)?.label ?? value,
  },
  {
    key: 'quantity',
    label: t('aiRun.quantity'),
    minWidth: 140,
    value: (row) => `${row.quantity} ${row.unit}`,
  },
  {
    key: 'unitQuantity',
    label: t('aiRun.unitQuantity'),
    minWidth: 140,
    value: (row) => `${row.unitQuantity} ${row.unit}`,
  },
  { prop: 'billingUnitPriceUnits', label: t('aiRun.billingUnitPriceUnits'), minWidth: 210 },
  { prop: 'multiplierPpm', label: t('aiRun.multiplierPpm'), minWidth: 120 },
  { prop: 'amountUnits', label: t('aiRun.amountUnits'), minWidth: 210 },
  { key: 'pricing', label: t('aiRun.pricing'), expand: true },
])
</script>

<template>
  <section class="ai-run-records" data-testid="ai-run-records">
    <AppTable
      v-if="kind === 'request'"
      :data="requests"
      :columns="requestColumns"
      :loading="loading"
      :aria-label="t('aiRun.requests')"
      @refresh="$emit('refresh')"
    >
      <template #expand="{ row }">
        <div class="ai-run-records__details">
          <h5>{{ t('aiRun.responseItems') }}</h5>
          <pre>{{ JSON.stringify(row.responseItems, null, 2) }}</pre>
          <h5>{{ t('aiRun.rawUsage') }}</h5>
          <pre>{{ JSON.stringify(row.usage, null, 2) }}</pre>
        </div>
      </template>
    </AppTable>
    <AppTable
      v-else-if="kind === 'tool-call'"
      :data="tools"
      :columns="toolColumns"
      :loading="loading"
      :aria-label="t('aiRun.tools')"
      @refresh="$emit('refresh')"
    />
    <AppTable
      v-else-if="kind === 'usage-charge'"
      :data="charges"
      :columns="chargeColumns"
      :loading="loading"
      :aria-label="t('aiRun.charges')"
      @refresh="$emit('refresh')"
    >
      <template #expand="{ row }">
        <dl class="ai-run-records__details">
          <dt>{{ t('aiRun.priceVersionId') }}</dt>
          <dd>{{ row.priceVersionId }}</dd>
          <dt>{{ t('aiRun.multiplierPpm') }}</dt>
          <dd>{{ row.multiplierPpm }}</dd>
        </dl>
      </template>
    </AppTable>
    <AppTable
      v-else
      :data="chargeItems"
      :columns="chargeItemColumns"
      :loading="loading"
      :aria-label="t('aiRun.chargeItems')"
      @refresh="$emit('refresh')"
    >
      <template #expand="{ row }">
        <div class="ai-run-records__details">
          <dl>
            <dt>{{ t('aiRun.chargeItemId') }}</dt>
            <dd>{{ row.id }}</dd>
            <dt>{{ t('aiRun.chargeId') }}</dt>
            <dd>{{ row.chargeId }}</dd>
            <dt>{{ t('aiRun.requestId') }}</dt>
            <dd>{{ row.providerRequestId }}</dd>
            <dt>{{ t('aiRun.priceRateId') }}</dt>
            <dd>{{ row.priceRateId }}</dd>
          </dl>
          <h5>{{ t('aiRun.selector') }}</h5>
          <pre>{{ JSON.stringify(row.selector, null, 2) }}</pre>
        </div>
      </template>
    </AppTable>
  </section>
</template>

<style scoped lang="scss">
.ai-run-records {
  min-width: 0;
  margin: 12px 0;

  &__details {
    margin: 0;
    padding: 16px;
  }

  dl {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: 8px 16px;
  }

  dt {
    color: var(--el-text-color-secondary);
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  h5 {
    margin: 0 0 8px;
  }
  pre {
    padding: 12px;
    margin: 0 0 16px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: var(--el-fill-color-light);
  }
}
</style>
