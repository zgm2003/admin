import { request } from '../utils/request'
import type { YesNo } from '../enums/yes-no'
import {
  type CreateMenuInput,
  type ManagedMenuNode,
  type MenuIDResult,
  type MenuStatusResult,
  type UpdateMenuInput,
  parseManagedMenus,
  parseMenuIDResult,
  parseMenuStatusResult,
} from './menu.contract'

export async function getMenus(): Promise<ManagedMenuNode[]> {
  const data = await request<unknown>({ method: 'GET', url: '/api/v1/menus' })
  return parseManagedMenus(data)
}

export async function createMenu(input: CreateMenuInput): Promise<MenuIDResult> {
  const data = await request<unknown>({ method: 'POST', url: '/api/v1/menus', data: input })
  return parseMenuIDResult(data)
}

export async function updateMenu(id: number, input: UpdateMenuInput): Promise<MenuIDResult> {
  const data = await request<unknown>({ method: 'PUT', url: `/api/v1/menus/${id}`, data: input })
  return parseMenuIDResult(data)
}

export async function updateMenuStatus(id: number, isEnabled: YesNo): Promise<MenuStatusResult> {
  const data = await request<unknown>({ method: 'PATCH', url: `/api/v1/menus/${id}/status`, data: { isEnabled } })
  return parseMenuStatusResult(data)
}

export async function deleteMenu(id: number): Promise<MenuIDResult> {
  const data = await request<unknown>({ method: 'DELETE', url: `/api/v1/menus/${id}` })
  return parseMenuIDResult(data)
}
