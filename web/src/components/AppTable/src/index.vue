<script setup lang="ts" generic="Row extends TableRow">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElButton, ElTable, ElTableColumn, ElPagination } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'

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

type TableResultState = 'idle' | 'loading' | 'refreshing' | 'success' | 'empty' | 'error'

const props = withDefaults(defineProps<{
  columns: TableColumn<Row>[]
  data: Row[]
  loading?: boolean
  rowKey?: string
  selectable?: boolean
  selectionSelectable?: (row: Row, index: number) => boolean
  pagination?: TablePaginationState | null
  resultState?: TableResultState
  statusMessage?: string
  ariaLabel?: string
  fixedFooter?: boolean
  refreshLabel?: string
}>(), {
  loading: false,
  rowKey: 'id',
  selectable: false,
  selectionSelectable: undefined,
  pagination: null,
  resultState: 'idle',
  statusMessage: '',
  ariaLabel: 'Data table',
  fixedFooter: false,
  refreshLabel: '刷新',
})

const emit = defineEmits<{
  refresh: []
  'row-click': [row: Row]
  'selection-change': [selection: Row[]]
  'update:pagination': [pagination: TablePaginationState]
  'column-change': [keys: TableColumnKey[]]
}>()

const paginationState = ref<TablePaginationState | null>(props.pagination ? { ...props.pagination } : null)
const isMobile = ref(false)

function updateMobile(): void {
  isMobile.value = window.innerWidth <= 768
}
onMounted(() => {
  updateMobile()
  window.addEventListener('resize', updateMobile)
})
onBeforeUnmount(() => window.removeEventListener('resize', updateMobile))

watch(() => props.pagination, (value) => {
  paginationState.value = value === null ? null : { ...value }
}, { deep: true, immediate: true })

const visibleColumns = computed(() => props.columns.filter((column) => !column.hidden))
const dataColumns = computed(() => visibleColumns.value.filter((column) => !column.expand))
const expandColumns = computed(() => visibleColumns.value.filter((column) => column.expand))
const busy = computed(() => props.loading || props.resultState === 'loading' || props.resultState === 'refreshing')
const failed = computed(() => props.resultState === 'error')
const tableClasses = computed(() => ({ 'app-table__table--fixed': props.fixedFooter }))
const paginationLayout = computed(() => isMobile.value ? 'total, prev, pager, next' : 'total, sizes, prev, pager, next')
const pageSizes = computed(() => isMobile.value ? [20, 50] : [20, 50, 100])
const tableHeaderCellStyle = { background: 'var(--el-fill-color-light)' }

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
    :aria-label="ariaLabel"
    :aria-busy="busy"
  >
    <div class="app-table__toolbar">
      <div class="app-table__toolbar-left"><slot name="toolbar-left" /></div>
      <div class="app-table__toolbar-right">
        <slot name="toolbar-right" />
        <el-button
          data-testid="app-table-refresh"
          type="primary"
          :icon="Refresh"
          :loading="busy"
          @click="emit('refresh')"
        >
          {{ refreshLabel }}
        </el-button>
      </div>
    </div>
    <el-table
      ref="tableRef"
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
          <slot
            :name="`cell-${columnKey(column)}`"
            :row="row"
            :col="column"
            :value="cellValue(row, column, $index)"
            :index="$index"
          >
            {{ cellText(row, column, $index) }}
          </slot>
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
        <div v-if="failed" class="app-table__error" role="alert">{{ statusMessage || 'Request failed' }}</div>
        <slot v-else name="empty"><el-empty :description="statusMessage || 'No data'" /></slot>
      </template>
    </el-table>
    <div v-if="paginationState !== null" class="app-table__pagination app-table__pagination--distributed">
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

<style scoped>
.app-table { display: flex; min-width: 0; flex-direction: column; }
.app-table--fixed-footer { height: 100%; min-height: 0; overflow: hidden; }
.app-table__toolbar { display: flex; justify-content: space-between; gap: 12px; margin-bottom: 8px; }
.app-table__toolbar-left, .app-table__toolbar-right { display: flex; min-width: 0; align-items: center; gap: 8px; }
.app-table__table--fixed { flex: 1 1 auto; min-height: 0; }
.app-table__pagination { display: flex; flex-shrink: 0; justify-content: flex-end; margin-top: 8px; }
.app-table__error { padding: 12px; color: var(--el-color-danger); }
@media (max-width: 768px) {
  .app-table__toolbar { align-items: stretch; flex-direction: column; }
  .app-table__toolbar-left, .app-table__toolbar-right { flex-wrap: wrap; }
  .app-table__pagination { justify-content: space-between; overflow-x: auto; }
  .app-table__pagination--distributed :deep(.el-pagination) { width: 100%; justify-content: flex-start; }
  .app-table__pagination--distributed :deep(.el-pagination__total) { margin-right: auto; }
}
</style>
