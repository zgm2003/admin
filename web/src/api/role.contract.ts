import { isMenuI18nKey } from './menu-fields'
import { isYesNo, type YesNo } from '../enums/yes-no'
import { ProtocolError } from '../types/http'
import type { PageRequest, PageResult } from '../types/pagination'

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
  i18nKey: string
  isEnabled: YesNo
  children: RolePermissionTreeNode[]
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
  role: {
    id: number
    code: string
    name: string
    isDefault: YesNo
    isEnabled: YesNo
  }
  menuTree: RolePermissionTreeNode[]
  menuIds: number[]
}

export interface UpdateRolePermissionsInput {
  menuIds: number[]
}

export interface RolePermissionResult {
  id: number
  permissionCount: number
}

const timestampPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/

export function parseRolePage(value: unknown): PageResult<RoleListItem> {
  const record = closed(value, ['list', 'page', 'pageSize', 'total'], 'role page')
  if (!Array.isArray(record.list)) {
    throw new ProtocolError('role page list must be an array')
  }

  const ids = new Set<number>()
  const codes = new Set<string>()
  const list = record.list.map((item, index) => {
    const row = closed(
      item,
      [
        'code',
        'createdAt',
        'id',
        'isDefault',
        'isEnabled',
        'name',
        'permissionCount',
        'updatedAt',
        'userCount',
      ],
      `role list item ${index}`,
    )
    const id = positive(row.id, 'role id')
    const code = text(row.code, 'role code')
    const name = text(row.name, 'role name')

    if (ids.has(id) || codes.has(code)) {
      throw new ProtocolError('role list contains duplicate id or code')
    }
    ids.add(id)
    codes.add(code)

    if (!isYesNo(row.isDefault) || !isYesNo(row.isEnabled)) {
      throw new ProtocolError('role state must be 0 or 1')
    }

    return {
      id,
      code,
      name,
      isDefault: row.isDefault,
      isEnabled: row.isEnabled,
      userCount: nonNegative(row.userCount, 'userCount'),
      permissionCount: nonNegative(row.permissionCount, 'permissionCount'),
      createdAt: timestamp(row.createdAt, 'createdAt'),
      updatedAt: timestamp(row.updatedAt, 'updatedAt'),
    }
  })

  return {
    list,
    total: nonNegative(record.total, 'total'),
    page: positive(record.page, 'page'),
    pageSize: positive(record.pageSize, 'pageSize'),
  }
}

export function parseRoleIDResult(value: unknown): { id: number } {
  const record = closed(value, ['id'], 'role id result')
  return { id: positive(record.id, 'id') }
}

export function parseEmptyResult(value: unknown): Record<string, never> {
  closed(value, [], 'empty result')
  return {}
}

export function parseRoleStatusResult(value: unknown): RoleStatusResult {
  const record = closed(value, ['id', 'isEnabled'], 'role status')
  if (!isYesNo(record.isEnabled)) {
    throw new ProtocolError('invalid role status')
  }

  return {
    id: positive(record.id, 'id'),
    isEnabled: record.isEnabled,
  }
}

export function parseRoleDefaultResult(value: unknown): RoleDefaultResult {
  const record = closed(value, ['id', 'isDefault'], 'role default')
  if (!isYesNo(record.isDefault)) {
    throw new ProtocolError('invalid role default')
  }

  return {
    id: positive(record.id, 'id'),
    isDefault: record.isDefault,
  }
}

export function parseRolePermissionResult(value: unknown): RolePermissionResult {
  const record = closed(value, ['id', 'permissionCount'], 'permission result')
  return {
    id: positive(record.id, 'id'),
    permissionCount: nonNegative(record.permissionCount, 'permissionCount'),
  }
}

export function parseRolePermissions(value: unknown): RolePermissionsResponse {
  const record = closed(value, ['menuIds', 'menuTree', 'role'], 'role permissions')
  const role = closed(
    record.role,
    ['code', 'id', 'isDefault', 'isEnabled', 'name'],
    'permission role',
  )

  if (!isYesNo(role.isDefault) || !isYesNo(role.isEnabled)) {
    throw new ProtocolError('invalid permission role state')
  }
  if (!Array.isArray(record.menuTree) || !Array.isArray(record.menuIds)) {
    throw new ProtocolError('permission arrays are required')
  }

  const ids = new Set<number>()
  const codes = new Set<string>()

  function parseNode(
    value: unknown,
    parentID: number | null,
    parentType: RolePermissionMenuType | null,
  ): RolePermissionTreeNode {
    const node = closed(
      value,
      ['children', 'code', 'i18nKey', 'id', 'isEnabled', 'menuType', 'parentId'],
      'permission node',
    )
    const id = positive(node.id, 'menu id')
    if (ids.has(id)) {
      throw new ProtocolError('duplicate permission menu id')
    }
    ids.add(id)

    const code = text(node.code, 'menu code')
    if (codes.has(code)) {
      throw new ProtocolError('duplicate permission menu code')
    }
    codes.add(code)

    if (node.parentId !== parentID) {
      throw new ProtocolError('permission parent mismatch')
    }
    if (node.menuType !== 'directory' && node.menuType !== 'page' && node.menuType !== 'action') {
      throw new ProtocolError('invalid permission menu type')
    }

    const menuType = node.menuType
    const hasIllegalShape =
      (parentType === null && menuType !== 'directory') ||
      (parentType === 'directory' && menuType === 'action') ||
      (parentType === 'page' && menuType !== 'action') ||
      parentType === 'action'
    if (hasIllegalShape) {
      throw new ProtocolError('illegal permission tree shape')
    }

    if (
      typeof node.i18nKey !== 'string' ||
      !isMenuI18nKey(node.i18nKey) ||
      !isYesNo(node.isEnabled) ||
      !Array.isArray(node.children)
    ) {
      throw new ProtocolError('invalid permission node protocol')
    }

    const children = node.children.map((child) => parseNode(child, id, menuType))
    if (menuType === 'action' && children.length !== 0) {
      throw new ProtocolError('action must be leaf')
    }

    return {
      id,
      parentId: parentID,
      menuType,
      code,
      i18nKey: node.i18nKey,
      isEnabled: node.isEnabled,
      children,
    }
  }

  const menuTree = record.menuTree.map((node) => parseNode(node, null, null))
  const menuIds = record.menuIds.map((id) => positive(id, 'menuIds'))
  const nodesByID = new Map(flattenNodes(menuTree).map((node) => [node.id, node]))

  for (let index = 0; index < menuIds.length; index += 1) {
    const menuID = menuIds[index]
    const previousMenuID = menuIds[index - 1]
    const node = nodesByID.get(menuID)
    const isUnstable = index > 0 && previousMenuID >= menuID

    if (isUnstable || node === undefined || node.menuType === 'directory') {
      throw new ProtocolError('menuIds must be unique sorted grantable existing ids')
    }
  }

  return {
    role: {
      id: positive(role.id, 'role id'),
      code: text(role.code, 'role code'),
      name: text(role.name, 'role name'),
      isDefault: role.isDefault,
      isEnabled: role.isEnabled,
    },
    menuTree,
    menuIds,
  }
}

function closed(
  value: unknown,
  keys: readonly string[],
  label: string,
): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ProtocolError(`${label} must be an object`)
  }

  const record = value as Record<string, unknown>
  const actual = Object.keys(record).sort()
  const expected = [...keys].sort()
  const hasDifferentKeys =
    actual.length !== expected.length || actual.some((key, index) => key !== expected[index])
  if (hasDifferentKeys) {
    throw new ProtocolError(`${label} has missing or extra fields`)
  }

  return record
}

function positive(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) {
    throw new ProtocolError(`${label} must be positive integer`)
  }
  return value
}

function nonNegative(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new ProtocolError(`${label} must be non-negative integer`)
  }
  return value
}

function text(value: unknown, label: string): string {
  if (typeof value !== 'string' || value === '' || value.trim() !== value) {
    throw new ProtocolError(`${label} must be non-empty trimmed string`)
  }
  return value
}

function timestamp(value: unknown, label: string): string {
  const result = text(value, label)
  if (!timestampPattern.test(result) || !Number.isFinite(Date.parse(result))) {
    throw new ProtocolError(`${label} must be RFC3339`)
  }
  return result
}

function flattenNodes(nodes: readonly RolePermissionTreeNode[]): RolePermissionTreeNode[] {
  const result: RolePermissionTreeNode[] = []
  const stack = [...nodes].reverse()

  while (stack.length > 0) {
    const node = stack.pop()
    if (node === undefined) {
      continue
    }
    result.push(node)
    stack.push(...[...node.children].reverse())
  }

  return result
}
