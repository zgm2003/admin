export type SearchOptionValue = string | number
export type SearchDateRange = [] | [string, string]
export type SearchScalar = string | number | null | undefined
export type SearchFormValue = SearchScalar | SearchDateRange

export type SearchFieldType = 'input' | 'select-v2' | 'date-range'

type KeysMatching<T extends object, V> = string extends keyof T
  ? string
  : {
      [K in keyof T]-?: Exclude<T[K], undefined> extends V ? K : never
    }[keyof T] &
      string

interface SearchFieldBase {
  label: string
  placeholder?: string
  width?: string | number
  disabled?: boolean
  clearable?: boolean
  testId?: string
}

export interface InputSearchField<
  T extends object = Record<string, SearchFormValue>,
> extends SearchFieldBase {
  key: KeysMatching<T, SearchScalar>
  type: 'input'
}

export interface SelectSearchField<
  T extends object = Record<string, SearchFormValue>,
> extends SearchFieldBase {
  key: KeysMatching<T, SearchScalar>
  type: 'select-v2'
  options: SearchOption[]
}

export interface DateRangeSearchField<
  T extends object = Record<string, SearchFormValue>,
> extends SearchFieldBase {
  key: KeysMatching<T, SearchDateRange>
  type: 'date-range'
  valueFormat?: string
  rangeSeparator?: string
}

export type SearchField<T extends object = Record<string, SearchFormValue>> =
  InputSearchField<T> | SelectSearchField<T> | DateRangeSearchField<T>

export type SearchFormModel<T extends object = Record<string, SearchFormValue>> = T

export interface SearchOption {
  label: string
  value: SearchOptionValue
}
