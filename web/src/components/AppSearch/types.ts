export type SearchOptionValue = string | number
export type SearchDateRange = [] | [string, string]
export type SearchScalar = string | number | null | undefined
export type SearchFormValue = SearchScalar | SearchDateRange

export type SearchFieldType = 'input' | 'select-v2' | 'date-range'

interface SearchFieldBase {
  label: string
  placeholder?: string
  width?: string | number
  disabled?: boolean
  clearable?: boolean
  testId?: string
}

export interface InputSearchField<TKey extends string = string> extends SearchFieldBase {
  key: TKey
  type: 'input'
}

export interface SelectSearchField<
  TKey extends string = string,
  TValue extends SearchOptionValue = SearchOptionValue,
> extends SearchFieldBase {
  key: TKey
  type: 'select-v2'
  options: SearchOption<TValue>[]
}

export interface DateRangeSearchField<TKey extends string = string> extends SearchFieldBase {
  key: TKey
  type: 'date-range'
  startPlaceholder?: string
  endPlaceholder?: string
  valueFormat?: string
  rangeSeparator?: string
}

type SearchFieldForKey<T extends object, TKey extends keyof T & string> =
  Exclude<T[TKey], undefined> extends SearchDateRange
    ? DateRangeSearchField<TKey>
    : Exclude<T[TKey], undefined> extends SearchScalar
      ? | InputSearchField<TKey>
        | SelectSearchField<TKey, Extract<Exclude<T[TKey], null | undefined>, SearchOptionValue>>
      : never

type StrictSearchField<T extends object> = {
  [K in keyof T & string]: SearchFieldForKey<T, K>
}[keyof T & string]

export type SearchField<T extends object = Record<string, SearchFormValue>> = string extends keyof T
  ? InputSearchField | SelectSearchField | DateRangeSearchField
  : StrictSearchField<T>

export type SearchFormModel<T extends object = Record<string, SearchFormValue>> = T

export interface SearchOption<TValue extends SearchOptionValue = SearchOptionValue> {
  label: string
  value: TValue
}
