import { isYesNo, type YesNo } from '@/enums/yes-no'
import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'
import { ProtocolError } from '@/types/http'
import { expectId, expectInteger, expectPage, expectRecord, expectString } from '@/api/protocol'

export interface RoleListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}
export interface RoleListItem {
  id: number
  code: string
  name: string
  isDefault: YesNo
  isEnabled: YesNo
  userCount: number
  permissionCount: number
  createdAt: string
  updatedAt: string
}
export type RolePermissionMenuType = 'directory' | 'page' | 'action'
export interface RolePermissionTreeNode {
  id: number
  parentId: number | null
  menuType: RolePermissionMenuType
  code: string
  name: string
  isEnabled: YesNo
  children: RolePermissionTreeNode[]
}
export interface RolePermissionPlatform {
  id: number
  code: string
  name: string
  isEnabled: YesNo
  menuTree: RolePermissionTreeNode[]
}
export interface CreateRoleInput {
  code: string
  name: string
}
export interface UpdateRoleInput {
  name: string
}
export interface RoleStatusResult {
  id: number
  isEnabled: YesNo
}
export interface RoleDefaultResult {
  id: number
  isDefault: YesNo
}
export interface RolePermissionsResponse {
  role: { id: number; code: string; name: string; isDefault: YesNo; isEnabled: YesNo }
  platforms: RolePermissionPlatform[]
  menuIds: number[]
}
export interface UpdateRolePermissionsInput {
  menuIds: number[]
}
export interface RolePermissionResult {
  id: number
  permissionCount: number
}

export async function getRoles(query: RoleListQuery): Promise<PageResult<RoleListItem>> {
  return expectPage(
    await request<unknown>({
      method: 'GET',
      url: '/api/admin/v1/roles',
      params: query,
    }),
    parseRole,
    'roles',
  )
}
export async function createRole(input: CreateRoleInput): Promise<{ id: number }> {
  return expectId(
    await request<unknown>({
      method: 'POST',
      url: '/api/admin/v1/roles',
      data: { code: input.code, name: input.name },
    }),
    'role create result',
  )
}
export async function updateRole(
  id: number,
  input: UpdateRoleInput,
): Promise<Record<string, never>> {
  await request<unknown>({
    method: 'PUT',
    url: `/api/admin/v1/roles/${id}`,
    data: { name: input.name },
  })
  return {}
}
export async function updateRoleStatus(id: number, isEnabled: YesNo): Promise<RoleStatusResult> {
  return parseRoleStatus(
    await request<unknown>({
      method: 'PATCH',
      url: `/api/admin/v1/roles/${id}/status`,
      data: { isEnabled },
    }),
  )
}
export async function setDefaultRole(id: number): Promise<RoleDefaultResult> {
  return parseRoleDefault(
    await request<unknown>({ method: 'PATCH', url: `/api/admin/v1/roles/${id}/default` }),
  )
}
export async function deleteRole(id: number): Promise<Record<string, never>> {
  await request<unknown>({ method: 'DELETE', url: `/api/admin/v1/roles/${id}` })
  return {}
}
export async function getRolePermissions(id: number): Promise<RolePermissionsResponse> {
  const raw = await request<unknown>({
    method: 'GET',
    url: `/api/admin/v1/roles/${id}/permissions`,
  })
  return parseRolePermissions(raw)
}
export async function updateRolePermissions(
  id: number,
  input: UpdateRolePermissionsInput,
): Promise<RolePermissionResult> {
  return parseRolePermissionResult(
    await request<unknown>({
      method: 'PUT',
      url: `/api/admin/v1/roles/${id}/permissions`,
      data: { menuIds: input.menuIds },
    }),
  )
}

function parseRole(value: unknown, index: number): RoleListItem {
  const item = expectRecord(value, `roles.list[${index}]`)
  const isDefault = item.isDefault
  const isEnabled = item.isEnabled
  if (!isYesNo(isDefault) || !isYesNo(isEnabled))
    throw new ProtocolError('role yes/no field is invalid')
  return {
    id: expectInteger(item.id, 'role.id'),
    code: expectString(item.code, 'role.code'),
    name: expectString(item.name, 'role.name'),
    isDefault,
    isEnabled,
    userCount: expectInteger(item.userCount, 'role.userCount'),
    permissionCount: expectInteger(item.permissionCount, 'role.permissionCount'),
    createdAt: expectString(item.createdAt, 'role.createdAt'),
    updatedAt: expectString(item.updatedAt, 'role.updatedAt'),
  }
}

function parseRoleStatus(value: unknown): RoleStatusResult {
  const item = expectRecord(value, 'role status result')
  const isEnabled = item.isEnabled
  if (!isYesNo(isEnabled)) throw new ProtocolError('role status is invalid')
  return { id: expectInteger(item.id, 'role status id'), isEnabled }
}
function parseRoleDefault(value: unknown): RoleDefaultResult {
  const item = expectRecord(value, 'role default result')
  const isDefault = item.isDefault
  if (!isYesNo(isDefault)) throw new ProtocolError('role default is invalid')
  return { id: expectInteger(item.id, 'role default id'), isDefault }
}
function parseRolePermissionResult(value: unknown): RolePermissionResult {
  const item = expectRecord(value, 'role permission result')
  return {
    id: expectInteger(item.id, 'role permission id'),
    permissionCount: expectInteger(item.permissionCount, 'role permission count'),
  }
}

const roleSummaryKeys = ['id', 'code', 'name', 'isDefault', 'isEnabled'] as const
const permissionPlatformKeys = ['id', 'code', 'name', 'isEnabled', 'menuTree'] as const
const permissionNodeKeys = [
  'id',
  'parentId',
  'menuType',
  'code',
  'name',
  'isEnabled',
  'children',
] as const

function parseRolePermissions(value: unknown): RolePermissionsResponse {
  if (
    !hasExactKeys(value, ['role', 'platforms', 'menuIds']) ||
    !hasExactKeys(value.role, roleSummaryKeys) ||
    !isPositiveInteger(value.role.id) ||
    !isNonEmptyString(value.role.code) ||
    !isNonEmptyString(value.role.name) ||
    !isYesNo(value.role.isDefault) ||
    !isYesNo(value.role.isEnabled) ||
    !Array.isArray(value.platforms) ||
    value.platforms.length === 0 ||
    !Array.isArray(value.menuIds)
  ) {
    throw invalidRolePermissions()
  }
  const platformIDs = new Set<number>()
  const platformCodes = new Set<string>()
  const menuIDs = new Set<number>()
  const grantableMenuIDs = new Set<number>()
  const platforms = value.platforms.map((platform) => {
    if (
      !hasExactKeys(platform, permissionPlatformKeys) ||
      !isPositiveInteger(platform.id) ||
      !isNonEmptyString(platform.code) ||
      !isNonEmptyString(platform.name) ||
      !isYesNo(platform.isEnabled) ||
      !Array.isArray(platform.menuTree) ||
      platformIDs.has(platform.id) ||
      platformCodes.has(platform.code)
    ) {
      throw invalidRolePermissions()
    }
    platformIDs.add(platform.id)
    platformCodes.add(platform.code)
    return {
      id: platform.id,
      code: platform.code,
      name: platform.name,
      isEnabled: platform.isEnabled,
      menuTree: platform.menuTree.map((node) =>
        parsePermissionNode(node, null, menuIDs, grantableMenuIDs),
      ),
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
  if (
    !hasExactKeys(value, permissionNodeKeys) ||
    !isPositiveInteger(value.id) ||
    !isNullablePositiveInteger(value.parentId) ||
    value.parentId !== expectedParentID ||
    !isPermissionMenuType(value.menuType) ||
    !isNonEmptyString(value.code) ||
    !isNonEmptyString(value.name) ||
    !isYesNo(value.isEnabled) ||
    !Array.isArray(value.children) ||
    menuIDs.has(value.id)
  ) {
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
    children: value.children.map((child) =>
      parsePermissionNode(child, id, menuIDs, grantableMenuIDs),
    ),
  }
}

function hasExactKeys<const Keys extends readonly string[]>(
  value: unknown,
  keys: Keys,
): value is Record<Keys[number], unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const actual = Object.keys(value)
  return (
    actual.length === keys.length &&
    keys.every((key) => Object.prototype.hasOwnProperty.call(value, key))
  )
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
