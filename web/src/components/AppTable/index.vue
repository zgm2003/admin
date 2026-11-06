<!-- 组件：通用数据表格 -->
<script setup lang="ts" generic="Row extends TableRow">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

import {
  formatTableColumnValue,
  tableColumnKey,
  tableColumnProp,
  tableColumnValue,
  type TableColumn,
  type TableColumnKey,
  type TablePaginationState,
  type TableRow,
} from './types'

defineOptions({ name: 'AppTable' })

type TableResultState = 'idle' | 'loading' | 'refreshing' | 'success' | 'empty' | 'error'

const props = withDefaults(
  defineProps<{
    columns: TableColumn<Row>[]
    data: Row[]
    loading?: boolean
    rowKey?: string | ((row: Row) => string)
    selectable?: boolean
    selectionSelectable?: (row: Row, index: number) => boolean
    pagination?: TablePaginationState | null
    resultState?: TableResultState
    statusMessage?: string
    ariaLabel?: string
    fixedFooter?: boolean
    refreshLabel?: string
  }>(),
  {
    loading: false,
    rowKey: 'id',
    selectable: false,
    selectionSelectable: undefined,
    pagination: null,
    resultState: 'idle',
    statusMessage: '',
    ariaLabel: undefined,
    fixedFooter: false,
    refreshLabel: undefined,
  },
)

const emit = defineEmits<{
  refresh: []
  'row-click': [row: Row]
  'selection-change': [selection: Row[]]
  'update:pagination': [pagination: TablePaginationState]
}>()

const { t } = useI18n()

const paginationState = ref<TablePaginationState | null>(
  props.pagination ? { ...props.pagination } : null,
)
const isMobile = ref(false)

function updateMobile(): void {
  isMobile.value = window.innerWidth <= 768
}
onMounted(() => {
  updateMobile()
  window.addEventListener('resize', updateMobile)
})
onBeforeUnmount(() => window.removeEventListener('resize', updateMobile))

watch(
  () => props.pagination,
  (value) => {
    paginationState.value = value === null ? null : { ...value }
  },
  { deep: true, immediate: true },
)

const visibleColumns = computed(() => props.columns.filter((column) => !column.hidden))
const dataColumns = computed(() => visibleColumns.value.filter((column) => !column.expand))
const expandColumns = computed(() => visibleColumns.value.filter((column) => column.expand))
const busy = computed(
  () => props.loading || props.resultState === 'loading' || props.resultState === 'refreshing',
)
const failed = computed(() => props.resultState === 'error')
const tableClasses = computed(() => ({ 'app-table__table--fixed': props.fixedFooter }))
const paginationLayout = computed(() =>
  isMobile.value ? 'total, prev, pager, next' : 'total, sizes, prev, pager, next',
)
const pageSizes = computed(() => (isMobile.value ? [20, 50] : [20, 50, 100]))
const tableHeaderCellStyle = { background: 'var(--el-fill-color-light)' }
const resolvedAriaLabel = computed(() => props.ariaLabel ?? t('appTable.ariaLabel'))
const resolvedRefreshLabel = computed(() => props.refreshLabel ?? t('common.refresh'))
const resolvedErrorMessage = computed(() => props.statusMessage || t('appTable.requestFailed'))
const resolvedEmptyMessage = computed(() => props.statusMessage || t('appTable.empty'))

function columnKey(column: TableColumn<Row>): TableColumnKey {
  return tableColumnKey<Row>(column)
}
function cellValue(row: Row, column: TableColumn<Row>, index: number): unknown {
  return tableColumnValue(row, column, index)
}
function cellText(row: Row, column: TableColumn<Row>, index: number): unknown {
  return formatTableColumnValue(row, column, index)
}
function columnBindings(column: TableColumn<Row>): Record<string, unknown> {
  const prop = tableColumnProp<Row>(column)
  return {
    ...(prop === undefined ? {} : { prop }),
    label: column.label,
    align: 'center',
    headerAlign: 'center',
    width: column.width,
    minWidth: column.minWidth,
    fixed: column.fixed,
    showOverflowTooltip: column.overflowTooltip ?? Boolean(column.width || column.minWidth),
    ...column.elementProps,
  }
}
function onPageChange(currentPage: number): void {
  if (paginationState.value === null) return
  const next = { ...paginationState.value, currentPage }
  paginationState.value = next
  emit('update:pagination', next)
}
function onPageSizeChange(pageSize: number): void {
  if (paginationState.value === null) return
  const next = { ...paginationState.value, pageSize, currentPage: 1 }
  paginationState.value = next
  emit('update:pagination', next)
}
function onRowClick(row: Row): void {
  emit('row-click', row)
}
function onSelectionChange(selection: Row[]): void {
  emit('selection-change', selection)
}
</script>

<template>
  <div
    class="app-table"
    :class="{ 'app-table--fixed-footer': fixedFooter }"
    role="region"
    :aria-label="resolvedAriaLabel"
    :aria-busy="busy"
  >
    <div class="app-table__toolbar">
      <el-space class="app-table__toolbar-left" wrap size="small"
        ><slot name="toolbar-left"
      /></el-space>
      <el-space class="app-table__toolbar-right" wrap size="small">
        <slot name="toolbar-right" />
        <el-button
          data-testid="app-table-refresh"
          type="primary"
          :icon="Refresh"
          :loading="busy"
          @click="emit('refresh')"
        >
          {{ resolvedRefreshLabel }}
        </el-button>
      </el-space>
    </div>
    <el-table
      v-loading="busy"
      border
      :data="data"
      :header-cell-style="tableHeaderCellStyle"
      :row-key="rowKey"
      :class="tableClasses"
      @row-click="onRowClick"
      @selection-change="onSelectionChange"
    >
      <el-table-column
        v-if="selectable"
        type="selection"
        width="48"
        align="center"
        header-align="center"
        :selectable="selectionSelectable"
      />
      <el-table-column
        v-for="column in dataColumns"
        :key="columnKey(column)"
        v-bind="columnBindings(column)"
      >
        <template #default="{ row, $index }: { row: Row; $index: number }">
          <div class="app-table__cell-slot">
            <slot
              :name="`cell-${columnKey(column)}`"
              :row="row"
              :col="column"
              :value="cellValue(row, column, $index)"
              :index="$index"
            >
              {{ cellText(row, column, $index) }}
            </slot>
          </div>
        </template>
      </el-table-column>
      <el-table-column
        v-for="column in expandColumns"
        :key="`expand-${columnKey(column)}`"
        type="expand"
      >
        <template #default="{ row }: { row: Row }">
          <slot name="expand" :row="row" />
        </template>
      </el-table-column>
      <template #empty>
        <div v-if="failed" class="app-table__error" role="alert">{{ resolvedErrorMessage }}</div>
        <slot v-else name="empty"><el-empty :description="resolvedEmptyMessage" /></slot>
      </template>
    </el-table>
    <div
      v-if="paginationState !== null"
      class="app-table__pagination app-table__pagination--distributed"
    >
      <el-pagination
        background
        :layout="paginationLayout"
        :small="isMobile"
        :current-page="paginationState.currentPage"
        :page-size="paginationState.pageSize"
        :page-sizes="pageSizes"
        :total="paginationState.total"
        @current-change="onPageChange"
        @size-change="onPageSizeChange"
      />
    </div>
  </div>
</template>

<style scoped lang="scss">
.app-table {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 12px;

  &--fixed-footer {
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }
  &__toolbar {
    display: flex;
    min-height: 32px;
    align-items: center;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 12px;
  }
  &__toolbar-left,
  &__toolbar-right {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 8px;
  }

  &__toolbar-right {
    margin-left: auto;
  }

  > :deep(.el-table) {
    --el-table-border-color: var(--admin-border);
    --el-table-text-color: var(--admin-text);
    --el-table-header-text-color: var(--admin-text-soft);
    --el-table-row-hover-bg-color: var(--el-color-primary-light-9);

    border-radius: var(--admin-radius-sm, 8px);

    .el-table__cell {
      padding-top: 10px;
      padding-bottom: 10px;
    }

    th.el-table__cell {
      font-size: 13px;
      font-weight: 500;
    }

    .el-table__empty-block {
      min-height: 160px;
    }

    .el-button.is-link,
    .el-button.is-text {
      min-height: 28px;
      padding: 4px 0;
      border-radius: 4px;
    }
  }

  &__table--fixed {
    flex: 1 1 auto;
    min-height: 0;
  }
  &__pagination {
    display: flex;
    flex-shrink: 0;
    justify-content: flex-end;
    padding-top: 14px;
    border-top: 1px solid var(--admin-border);

    :deep(.el-pagination) {
      --el-pagination-button-bg-color: var(--admin-surface-soft);
      --el-pagination-hover-color: var(--el-color-primary);
    }
  }

  &__error {
    padding: 20px;
    color: var(--el-color-danger);
    line-height: 1.5;
  }

  &__cell-slot {
    min-width: 0;
  }
}
@media (max-width: 768px) {
  .app-table {
    &__toolbar {
      align-items: stretch;
      flex-direction: column;
    }
    &__toolbar-left,
    &__toolbar-right {
      flex-wrap: wrap;
    }

    &__toolbar-right {
      margin-left: 0;
    }

    &__pagination {
      justify-content: space-between;
      overflow-x: auto;
    }
    &__pagination--distributed :deep(.el-pagination) {
      width: 100%;
      justify-content: flex-start;
    }
    &__pagination--distributed :deep(.el-pagination__total) {
      margin-right: auto;
    }
  }
}
</style>
