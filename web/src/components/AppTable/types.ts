import type { TableColumnCtx } from 'element-plus'

export type TableRow = object
export type TableColumnKey = string

export type TableColumnElementProps<Row extends TableRow = TableRow> = Partial<
  Pick<
    TableColumnCtx<Row>,
    | 'align'
    | 'headerAlign'
    | 'sortable'
    | 'sortMethod'
    | 'sortBy'
    | 'resizable'
    | 'columnKey'
    | 'className'
    | 'labelClassName'
    | 'filters'
    | 'filterMethod'
    | 'filterPlacement'
    | 'filterMultiple'
    | 'filteredValue'
    | 'reserveSelection'
    | 'sortOrders'
    | 'tooltipFormatter'
  >
>

interface TableColumnBase<Row extends TableRow> {
  label: string
  expand?: boolean
  hidden?: boolean
  width?: string | number
  minWidth?: string | number
  fixed?: boolean | 'left' | 'right'
  overflowTooltip?: boolean
  elementProps?: TableColumnElementProps<Row>
}

type PropertyTableColumn<Row extends TableRow> = {
  [Key in keyof Row & string]: TableColumnBase<Row> & {
    prop: Key
    key?: TableColumnKey
    formatter?: (row: Row, value: Row[Key], index: number) => unknown
  }
}[keyof Row & string]

export interface DerivedTableColumn<Row extends TableRow> extends TableColumnBase<Row> {
  key: TableColumnKey
  prop?: never
  value?: (row: Row, index: number) => unknown
  formatter?: (row: Row, value: unknown, index: number) => unknown
}

export type TableColumn<Row extends TableRow = TableRow> =
  PropertyTableColumn<Row> | DerivedTableColumn<Row>

export interface TablePaginationState {
  currentPage: number
  pageSize: number
  total: number
}

interface TableColumnIdentity<Row extends TableRow> {
  readonly key?: TableColumnKey
  readonly prop?: keyof Row & string
  readonly label?: string
}

export function tableColumnKey<Row extends TableRow>(
  column: TableColumnIdentity<Row>,
): TableColumnKey {
  const key = column.key ?? column.prop
  if (typeof key !== 'string' || key.trim() === '') {
    throw new Error('AppTable column key or prop is required')
  }
  return key
}

export function tableColumnProp<Row extends TableRow>(
  column: TableColumnIdentity<Row>,
): string | undefined {
  return column.prop
}

export function tableColumnValue<Row extends TableRow>(
  row: Row,
  column: TableColumn<Row>,
  index = 0,
): unknown {
  if (column.prop !== undefined) return row[column.prop]
  return 'value' in column ? column.value?.(row, index) : undefined
}

export function formatTableColumnValue<Row extends TableRow>(
  row: Row,
  column: TableColumn<Row>,
  index: number,
): unknown {
  if (column.prop !== undefined) {
    const value = row[column.prop]
    return column.formatter === undefined ? value : column.formatter(row, value, index)
  }
  if (!('value' in column)) return undefined
  const value = tableColumnValue(row, column, index)
  return column.formatter === undefined ? value : column.formatter(row, value, index)
}
