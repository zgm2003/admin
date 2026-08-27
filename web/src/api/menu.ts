import { request } from '../utils/request'
import type { YesNo } from '../enums/yes-no'
import { isMenuIconName, type MenuIconName } from '../icons/menu-icons'

export const menuI18nKeyPattern = /^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$/
export const menuCodePattern = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?::[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
export const menuPathPattern = /^\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
export const componentPathPattern = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/

const staticPaths: ReadonlySet<string> = new Set(['/login', '/register', '/dashboard'])

export function isMenuI18nKey(value: string): boolean { return value.length <= 128 && menuI18nKeyPattern.test(value) }
export function isMenuPath(value: string): boolean { return value.length <= 255 && !staticPaths.has(value) && menuPathPattern.test(value) }
export function isComponentPath(value: string): boolean { return value.length <= 255 && componentPathPattern.test(value) }
export function isMenuIcon(value: string): value is MenuIconName { return isMenuIconName(value) }

export type ManagedMenuType = 'directory' | 'page' | 'action'
export interface ManagedMenuNode { id:number; parentId:number|null; menuType:ManagedMenuType; name:string; code:string; i18nKey:string|null; path:string|null; componentPath:string|null; icon:MenuIconName|null; sortOrder:number; isEnabled:YesNo; isHidden:YesNo; isProtected:YesNo; createdAt:string; updatedAt:string; children:ManagedMenuNode[] }
export interface CreateMenuInput { parentId:number|null; menuType:ManagedMenuType; name:string; code:string; i18nKey:string|null; path:string|null; componentPath:string|null; icon:MenuIconName|null; sortOrder:number; isEnabled:YesNo; isHidden:YesNo }
export interface UpdateMenuInput { parentId:number|null; menuType:ManagedMenuType; name:string; i18nKey:string|null; path:string|null; componentPath:string|null; icon:MenuIconName|null; sortOrder:number; isHidden:YesNo }
export interface MenuIDResult { id:number }
export interface MenuStatusResult { id:number; isEnabled:YesNo }

export function getMenus(): Promise<ManagedMenuNode[]> { return request<ManagedMenuNode[]>({ method: 'GET', url: '/api/admin/v1/menus' }) }
export function createMenu(input: CreateMenuInput): Promise<MenuIDResult> { return request<MenuIDResult>({ method: 'POST', url: '/api/admin/v1/menus', data: input }) }
export function updateMenu(id: number, input: UpdateMenuInput): Promise<MenuIDResult> { return request<MenuIDResult>({ method: 'PUT', url: `/api/admin/v1/menus/${id}`, data: input }) }
export function updateMenuStatus(id: number, isEnabled: YesNo): Promise<MenuStatusResult> { return request<MenuStatusResult>({ method: 'PATCH', url: `/api/admin/v1/menus/${id}/status`, data: { isEnabled } }) }
export function deleteMenu(id: number): Promise<MenuIDResult> { return request<MenuIDResult>({ method: 'DELETE', url: `/api/admin/v1/menus/${id}` }) }
