import { isYesNo, type YesNo } from '../enums/yes-no'
import { ProtocolError } from '../types/http'
import type { PageRequest, PageResult } from '../types/pagination'

export interface UserListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
  roleId?: number
}

export interface UserRoleSummary { id: number; code: string; name: string; isEnabled: YesNo }
export interface UserListItem { id: number; username: string; email: string; isEnabled: YesNo; roles: UserRoleSummary[]; createdAt: string; updatedAt: string }
export interface UserRolesResponse { user: { id: number; username: string; email: string; isEnabled: YesNo }; roles: UserRoleSummary[]; roleIds: number[] }
export interface UpdateUserInput { username: string }
export interface UpdateUserRolesInput { roleIds: number[] }
export interface UpdatedUsername { id: number; username: string; updatedAt: string }
export interface UserStatusResult { id: number; isEnabled: YesNo }
export interface UserRoleOptions { roles: UserRoleSummary[] }
export interface UserRoleResult { id: number; roleCount: number }

const timestampPattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/

export function parseUserPage(value: unknown): PageResult<UserListItem> {
  const record = closed(value, ['list', 'page', 'pageSize', 'total'], 'user page')
  if (!Array.isArray(record.list)) throw new ProtocolError('user page list must be an array')
  const ids = new Set<number>()
  const list = record.list.map((value, index) => {
    const row = closed(value, ['createdAt', 'email', 'id', 'isEnabled', 'roles', 'updatedAt', 'username'], `user row ${index}`)
    const id = positive(row.id, 'user id')
    if (ids.has(id)) throw new ProtocolError('user page contains duplicate ids')
    ids.add(id)
    if (!isYesNo(row.isEnabled)) throw new ProtocolError('user status must be 0 or 1')
    return {
      id,
      username: text(row.username, 'username'),
      email: text(row.email, 'email'),
      isEnabled: row.isEnabled,
      roles: parseRoleArray(row.roles, `user ${id} roles`, true),
      createdAt: timestamp(row.createdAt, 'createdAt'),
      updatedAt: timestamp(row.updatedAt, 'updatedAt'),
    }
  })
  return { list, total: nonNegative(record.total, 'total'), page: positive(record.page, 'page'), pageSize: positive(record.pageSize, 'pageSize') }
}

export function parseUserRoleOptions(value: unknown): UserRoleOptions {
  const record = closed(value, ['roles'], 'user role options')
  return { roles: parseRoleArray(record.roles, 'role options', false) }
}

export function parseUpdatedUsername(value: unknown): UpdatedUsername {
  const record = closed(value, ['id', 'updatedAt', 'username'], 'updated username')
  return { id: positive(record.id, 'id'), username: text(record.username, 'username'), updatedAt: timestamp(record.updatedAt, 'updatedAt') }
}

export function parseUserStatusResult(value: unknown): UserStatusResult {
  const record = closed(value, ['id', 'isEnabled'], 'user status result')
  if (!isYesNo(record.isEnabled)) throw new ProtocolError('invalid user status')
  return { id: positive(record.id, 'id'), isEnabled: record.isEnabled }
}

export function parseEmptyUserResult(value: unknown): Record<string, never> {
  closed(value, [], 'empty user result')
  return {}
}

export function parseUserRoles(value: unknown): UserRolesResponse {
  const record = closed(value, ['roleIds', 'roles', 'user'], 'user roles')
  const rawUser = closed(record.user, ['email', 'id', 'isEnabled', 'username'], 'role user')
  if (!isYesNo(rawUser.isEnabled)) throw new ProtocolError('invalid role user status')
  const roles = parseRoleArray(record.roles, 'user role options', true)
  if (!Array.isArray(record.roleIds)) throw new ProtocolError('roleIds must be an array')
	const rawRoleIDs = record.roleIds
  const optionIDs = new Set(roles.map((role) => role.id))
	const roleIds = rawRoleIDs.map((value, index) => {
    const id = positive(value, 'roleIds')
		if ((index > 0 && positive(rawRoleIDs[index - 1], 'roleIds') >= id) || !optionIDs.has(id)) {
      throw new ProtocolError('roleIds must be sorted unique option ids')
    }
    return id
  })
  if (roleIds.length === 0) throw new ProtocolError('user must have at least one role')
  return {
    user: { id: positive(rawUser.id, 'user id'), username: text(rawUser.username, 'username'), email: text(rawUser.email, 'email'), isEnabled: rawUser.isEnabled },
    roles,
    roleIds,
  }
}

export function parseUserRoleResult(value: unknown): UserRoleResult {
  const record = closed(value, ['id', 'roleCount'], 'user role result')
  return { id: positive(record.id, 'id'), roleCount: positive(record.roleCount, 'roleCount') }
}

function parseRoleArray(value: unknown, label: string, requireNonEmpty: boolean): UserRoleSummary[] {
  if (!Array.isArray(value)) throw new ProtocolError(`${label} must be an array`)
  if (requireNonEmpty && value.length === 0) throw new ProtocolError(`${label} must not be empty`)
  const ids = new Set<number>()
  const codes = new Set<string>()
  const result = value.map((item, index) => {
    const record = closed(item, ['code', 'id', 'isEnabled', 'name'], `${label} item ${index}`)
    const id = positive(record.id, 'role id')
    const code = text(record.code, 'role code')
    if (ids.has(id) || codes.has(code)) throw new ProtocolError(`${label} contains duplicate role`)
    ids.add(id); codes.add(code)
    if (!isYesNo(record.isEnabled)) throw new ProtocolError('role status must be 0 or 1')
    return { id, code, name: text(record.name, 'role name'), isEnabled: record.isEnabled }
  })
  for (let index = 1; index < result.length; index += 1) {
    const previous = result[index - 1]
    const current = result[index]
    if (previous.code > current.code || (previous.code === current.code && previous.id >= current.id)) {
      throw new ProtocolError(`${label} must be sorted by code and id`)
    }
  }
  return result
}

function closed(value: unknown, keys: readonly string[], label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ProtocolError(`${label} must be an object`)
  const record = value as Record<string, unknown>
  const actual = Object.keys(record).sort()
  const expected = [...keys].sort()
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) throw new ProtocolError(`${label} has missing or extra fields`)
  return record
}

function positive(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 1) throw new ProtocolError(`${label} must be a positive integer`)
  return value
}

function nonNegative(value: unknown, label: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) throw new ProtocolError(`${label} must be a non-negative integer`)
  return value
}

function text(value: unknown, label: string): string {
  if (typeof value !== 'string' || value === '' || value.trim() !== value) throw new ProtocolError(`${label} must be a non-empty trimmed string`)
  return value
}

function timestamp(value: unknown, label: string): string {
  const result = text(value, label)
  if (!timestampPattern.test(result) || !Number.isFinite(Date.parse(result))) throw new ProtocolError(`${label} must be RFC3339`)
  return result
}
