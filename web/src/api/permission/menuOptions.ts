import { request } from '@/utils/request'
import type { ManagedMenuType } from './menu'

export interface MenuConstraints {
  codePattern: string
  i18nKeyPattern: string
  pathPattern: string
  componentPathPattern: string
  nameMaxLength: number
  codeMaxLength: number
  i18nKeyMaxLength: number
  pathMaxLength: number
  reservedPagePaths: string[]
}
export interface MenuOptions {
  menuTypes: Array<{ value: ManagedMenuType; label: string }>
  constraints: MenuConstraints
}
export function getMenuOptions(): Promise<MenuOptions> {
  return request.get<MenuOptions>('/api/admin/v1/permission/menu/options')
}
