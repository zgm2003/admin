export type SearchOptionValue = string | number
export type SearchDateRange = [] | [string, string]
export type SearchFormValue = string | number | boolean | SearchDateRange | null | undefined
export type SearchFormModel = Record<string, SearchFormValue>

export interface SearchOption {
  label: string
  value: SearchOptionValue
}

export type SearchFieldType = 'input' | 'select-v2'

interface SearchFieldBase {
  key: string
  label: string
  placeholder?: string
  width?: string | number
  disabled?: boolean
  clearable?: boolean
  testId?: string
}

export interface InputSearchField extends SearchFieldBase {
  type: 'input'
}

export interface SelectSearchField extends SearchFieldBase {
  type: 'select-v2'
  options: SearchOption[]
}

export interface DateRangeSearchField extends SearchFieldBase {
  type: 'date-range'
  valueFormat?: string
  rangeSeparator?: string
}

export type SearchField = InputSearchField | SelectSearchField | DateRangeSearchField
