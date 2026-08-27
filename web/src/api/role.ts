import { isYesNo, type YesNo } from '../enums/yes-no'
import { request } from '../utils/request'
import type { PageRequest, PageResult } from '../types/pagination'
import { ProtocolError } from '../types/http'

export interface RoleListQuery extends PageRequest { keyword?: string; isEnabled?: YesNo }
export interface RoleListItem { id:number; code:string; name:string; isDefault:YesNo; isEnabled:YesNo; userCount:number; permissionCount:number; createdAt:string; updatedAt:string }
export type RolePermissionMenuType = 'directory'|'page'|'action'
export interface RolePermissionTreeNode { id:number; parentId:number|null; menuType:RolePermissionMenuType; code:string; name:string; isEnabled:YesNo; children:RolePermissionTreeNode[] }
export interface RolePermissionPlatform { id:number; code:string; name:string; isEnabled:YesNo; menuTree:RolePermissionTreeNode[] }
export interface CreateRoleInput { code:string; name:string }
export interface UpdateRoleInput { name:string }
export interface RoleStatusResult { id:number; isEnabled:YesNo }
export interface RoleDefaultResult { id:number; isDefault:YesNo }
export interface RolePermissionsResponse { role:{id:number;code:string;name:string;isDefault:YesNo;isEnabled:YesNo}; platforms:RolePermissionPlatform[]; menuIds:number[] }
export interface UpdateRolePermissionsInput { menuIds:number[] }
export interface RolePermissionResult { id:number; permissionCount:number }

export function getRoles(query: RoleListQuery): Promise<PageResult<RoleListItem>> { return request<PageResult<RoleListItem>>({ method: 'GET', url: '/api/admin/v1/roles', params: query }) }
export function createRole(input: CreateRoleInput): Promise<{ id: number }> { return request<{ id: number }>({ method: 'POST', url: '/api/admin/v1/roles', data: { code: input.code, name: input.name } }) }
export function updateRole(id: number, input: UpdateRoleInput): Promise<Record<string, never>> { return request<Record<string, never>>({ method: 'PUT', url: `/api/admin/v1/roles/${id}`, data: { name: input.name } }) }
export function updateRoleStatus(id: number, isEnabled: YesNo): Promise<RoleStatusResult> { return request<RoleStatusResult>({ method: 'PATCH', url: `/api/admin/v1/roles/${id}/status`, data: { isEnabled } }) }
export function setDefaultRole(id: number): Promise<RoleDefaultResult> { return request<RoleDefaultResult>({ method: 'PATCH', url: `/api/admin/v1/roles/${id}/default` }) }
export function deleteRole(id: number): Promise<Record<string, never>> { return request<Record<string, never>>({ method: 'DELETE', url: `/api/admin/v1/roles/${id}` }) }
export async function getRolePermissions(id: number): Promise<RolePermissionsResponse> {
  const raw = await request<unknown>({ method: 'GET', url: `/api/admin/v1/roles/${id}/permissions` })
  return parseRolePermissions(raw)
}
export function updateRolePermissions(id: number, input: UpdateRolePermissionsInput): Promise<RolePermissionResult> { return request<RolePermissionResult>({ method: 'PUT', url: `/api/admin/v1/roles/${id}/permissions`, data: { menuIds: input.menuIds } }) }

const roleSummaryKeys = ['id', 'code', 'name', 'isDefault', 'isEnabled'] as const
const permissionPlatformKeys = ['id', 'code', 'name', 'isEnabled', 'menuTree'] as const
const permissionNodeKeys = ['id', 'parentId', 'menuType', 'code', 'name', 'isEnabled', 'children'] as const

function parseRolePermissions(value: unknown): RolePermissionsResponse {
  if (!hasExactKeys(value, ['role', 'platforms', 'menuIds']) || !hasExactKeys(value.role, roleSummaryKeys) ||
    !isPositiveInteger(value.role.id) || !isNonEmptyString(value.role.code) || !isNonEmptyString(value.role.name) ||
    !isYesNo(value.role.isDefault) || !isYesNo(value.role.isEnabled) || !Array.isArray(value.platforms) ||
    value.platforms.length === 0 || !Array.isArray(value.menuIds)) {
    throw invalidRolePermissions()
  }
  const platformIDs = new Set<number>()
  const platformCodes = new Set<string>()
  const menuIDs = new Set<number>()
  const grantableMenuIDs = new Set<number>()
  const platforms = value.platforms.map((platform) => {
    if (!hasExactKeys(platform, permissionPlatformKeys) || !isPositiveInteger(platform.id) ||
      !isNonEmptyString(platform.code) || !isNonEmptyString(platform.name) || !isYesNo(platform.isEnabled) ||
      !Array.isArray(platform.menuTree) || platformIDs.has(platform.id) || platformCodes.has(platform.code)) {
      throw invalidRolePermissions()
    }
    platformIDs.add(platform.id)
    platformCodes.add(platform.code)
    return {
      id: platform.id,
      code: platform.code,
      name: platform.name,
      isEnabled: platform.isEnabled,
      menuTree: platform.menuTree.map((node) => parsePermissionNode(node, null, menuIDs, grantableMenuIDs)),
    }
  })
  const directMenuIDs: number[] = []
  let previousMenuID = 0
  for (const menuID of value.menuIds) {
    if (!isPositiveInteger(menuID) || menuID <= previousMenuID || !grantableMenuIDs.has(menuID)) {
      throw invalidRolePermissions()
    }
    directMenuIDs.push(menuID)
    previousMenuID = menuID
  }
  return {
    role: {
      id: value.role.id,
      code: value.role.code,
      name: value.role.name,
      isDefault: value.role.isDefault,
      isEnabled: value.role.isEnabled,
    },
    platforms,
    menuIds: directMenuIDs,
  }
}

function parsePermissionNode(
  value: unknown,
  expectedParentID: number | null,
  menuIDs: Set<number>,
  grantableMenuIDs: Set<number>,
): RolePermissionTreeNode {
  if (!hasExactKeys(value, permissionNodeKeys) || !isPositiveInteger(value.id) ||
    !isNullablePositiveInteger(value.parentId) || value.parentId !== expectedParentID ||
    !isPermissionMenuType(value.menuType) || !isNonEmptyString(value.code) || !isNonEmptyString(value.name) ||
    !isYesNo(value.isEnabled) || !Array.isArray(value.children) || menuIDs.has(value.id)) {
    throw invalidRolePermissions()
  }
  const id = value.id
  const parentId = value.parentId
  menuIDs.add(id)
  if (value.menuType === 'page' || value.menuType === 'action') grantableMenuIDs.add(id)
  return {
    id,
    parentId,
    menuType: value.menuType,
    code: value.code,
    name: value.name,
    isEnabled: value.isEnabled,
    children: value.children.map((child) => parsePermissionNode(child, id, menuIDs, grantableMenuIDs)),
  }
}

function hasExactKeys<const Keys extends readonly string[]>(value: unknown, keys: Keys): value is Record<Keys[number], unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const actual = Object.keys(value)
  return actual.length === keys.length && keys.every((key) => Object.prototype.hasOwnProperty.call(value, key))
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

function isNullablePositiveInteger(value: unknown): value is number | null {
  return value === null || isPositiveInteger(value)
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value !== ''
}

function isPermissionMenuType(value: unknown): value is RolePermissionMenuType {
  return value === 'directory' || value === 'page' || value === 'action'
}

function invalidRolePermissions(): ProtocolError {
  return new ProtocolError('role permissions response is invalid')
}
