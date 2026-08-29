import { YesNo } from '../../enums/yes-no'
import type { PageRequest, PageResult } from '../../types/pagination'
import { ProtocolError, request } from '../../utils/request'

export interface UserListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
  roleId?: number
}

export interface UserRoleSummary { id: number; code: string; name: string; isEnabled: YesNo }
export interface UserListItem { id: number; username: string; email: string; phone: string | null; isEnabled: YesNo; roles: UserRoleSummary[]; createdAt: string; updatedAt: string }
export type UserPage = PageResult<UserListItem>
export interface UserRolesResponse { user: { id: number; username: string; email: string; phone: string | null; isEnabled: YesNo }; roles: UserRoleSummary[]; roleIds: number[] }
export interface UpdateUserInput { username: string; phone: string | null }
export interface UpdateUserRolesInput { roleIds: number[] }
export interface UpdatedProfile { id: number; username: string; phone: string | null; updatedAt: string }
export interface UserStatusResult { id: number; isEnabled: YesNo }
export interface UserRoleOptions { roles: UserRoleSummary[] }
export interface UserRoleResult { id: number; roleCount: number }

export async function getUsers(query: UserListQuery): Promise<UserPage> {
  return parseUserPage(await request<unknown>({ method: 'GET', url: '/api/admin/v1/users', params: query }))
}

export async function getUserRoleOptions(): Promise<UserRoleOptions> {
  return request<UserRoleOptions>({ method: 'GET', url: '/api/admin/v1/users/role-options' })
}

export async function updateUser(id: number, input: UpdateUserInput): Promise<UpdatedProfile> {
  return parseUpdatedProfile(await request<unknown>({ method: 'PUT', url: `/api/admin/v1/users/${id}`, data: input }))
}

export async function updateUserStatus(id: number, isEnabled: YesNo): Promise<UserStatusResult> {
  return request<UserStatusResult>({ method: 'PATCH', url: `/api/admin/v1/users/${id}/status`, data: { isEnabled } })
}

export async function deleteUser(id: number): Promise<Record<string, never>> {
  return request<Record<string, never>>({ method: 'DELETE', url: `/api/admin/v1/users/${id}` })
}

export async function getUserRoles(id: number): Promise<UserRolesResponse> {
  return parseUserRoles(await request<unknown>({ method: 'GET', url: `/api/admin/v1/users/${id}/roles` }))
}

export async function updateUserRoles(id: number, input: UpdateUserRolesInput): Promise<UserRoleResult> {
  return request<UserRoleResult>({ method: 'PUT', url: `/api/admin/v1/users/${id}/roles`, data: { roleIds: input.roleIds } })
}

function parseUserPage(value: unknown): UserPage {
  if (!isExactRecord(value, ['list', 'total', 'page', 'pageSize']) ||
    !Array.isArray(value.list) || !isNonNegativeInteger(value.total) ||
    !isPositiveInteger(value.page) || !isPositiveInteger(value.pageSize)) {
    throw new ProtocolError('user list response is invalid')
  }
  return { list: value.list.map(parseUserListItem), total: value.total, page: value.page, pageSize: value.pageSize }
}

function parseUserListItem(value: unknown): UserListItem {
  if (!isExactRecord(value, ['id', 'username', 'email', 'phone', 'isEnabled', 'roles', 'createdAt', 'updatedAt']) ||
    !isPositiveInteger(value.id) || typeof value.username !== 'string' ||
    typeof value.email !== 'string' || !isNullableString(value.phone) ||
    !isYesNo(value.isEnabled) || !Array.isArray(value.roles) ||
    typeof value.createdAt !== 'string' || typeof value.updatedAt !== 'string') {
    throw new ProtocolError('user list item response is invalid')
  }
  return {
    id: value.id,
    username: value.username,
    email: value.email,
    phone: value.phone,
    isEnabled: value.isEnabled,
    roles: value.roles.map(parseUserRoleSummary),
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
  }
}

function parseUpdatedProfile(value: unknown): UpdatedProfile {
  if (!isExactRecord(value, ['id', 'username', 'phone', 'updatedAt']) ||
    !isPositiveInteger(value.id) || typeof value.username !== 'string' ||
    !isNullableString(value.phone) || typeof value.updatedAt !== 'string') {
    throw new ProtocolError('updated user profile response is invalid')
  }
  return { id: value.id, username: value.username, phone: value.phone, updatedAt: value.updatedAt }
}

function parseUserRoles(value: unknown): UserRolesResponse {
  if (!isExactRecord(value, ['user', 'roles', 'roleIds']) ||
    !Array.isArray(value.roles) || !Array.isArray(value.roleIds) ||
    !value.roleIds.every(isPositiveInteger)) {
    throw new ProtocolError('user roles response is invalid')
  }
  return {
    user: parseUserSummary(value.user),
    roles: value.roles.map(parseUserRoleSummary),
    roleIds: value.roleIds,
  }
}

function parseUserSummary(value: unknown): UserRolesResponse['user'] {
  if (!isExactRecord(value, ['id', 'username', 'email', 'phone', 'isEnabled']) ||
    !isPositiveInteger(value.id) || typeof value.username !== 'string' ||
    typeof value.email !== 'string' || !isNullableString(value.phone) || !isYesNo(value.isEnabled)) {
    throw new ProtocolError('user role summary response is invalid')
  }
  return { id: value.id, username: value.username, email: value.email, phone: value.phone, isEnabled: value.isEnabled }
}

function parseUserRoleSummary(value: unknown): UserRoleSummary {
  if (!isExactRecord(value, ['id', 'code', 'name', 'isEnabled']) ||
    !isPositiveInteger(value.id) || typeof value.code !== 'string' ||
    typeof value.name !== 'string' || !isYesNo(value.isEnabled)) {
    throw new ProtocolError('user role response is invalid')
  }
  return { id: value.id, code: value.code, name: value.name, isEnabled: value.isEnabled }
}

function isExactRecord(value: unknown, keys: readonly string[]): value is Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false
  const actual = Object.keys(value).sort()
  const expected = [...keys].sort()
  return actual.length === expected.length && actual.every((key, index) => key === expected[index])
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

function isNonNegativeInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0
}

function isNullableString(value: unknown): value is string | null {
  return value === null || typeof value === 'string'
}

function isYesNo(value: unknown): value is YesNo {
  return value === YesNo.No || value === YesNo.Yes
}
