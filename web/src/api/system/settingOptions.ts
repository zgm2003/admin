import { request } from '@/utils/request'

export type SettingEditor = 'text' | 'number' | 'boolean' | 'json' | 'media'
export interface SettingPresentation {
  editor: SettingEditor
  valueTypeLocked: boolean
  minimum: number | null
  maximum: number | null
  maxLength: number | null
  allowEmpty: boolean
  warnOnDecrease: boolean
  refreshBrand: boolean
  mediaAccept: string
  mediaVariant: 'avatar' | 'default' | 'file'
  mediaRuleCode: string
  actions: { status: boolean; delete: boolean }
}
export interface SettingTypeOption {
  allowEmpty: boolean
  value: number
  label: string
  editor: SettingEditor
}
export interface SettingOptions {
  valueTypes: SettingTypeOption[]
  defaultValueType: number
  defaultPresentation: SettingPresentation
}

export function getSettingOptions(): Promise<SettingOptions> {
  return request.get<SettingOptions>('/api/admin/v1/system/setting/options')
}
