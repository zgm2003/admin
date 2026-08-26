import type { YesNo } from '../enums/yes-no'
import type { PageRequest, PageResult } from '../types/pagination'
import { request } from '../utils/request'

export interface UserListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
  roleId?: number
}

export interface UserRoleSummary { id: number; code: string; name: string; isEnabled: YesNo }
export interface UserListItem { id: number; username: string; email: string; isEnabled: YesNo; roles: UserRoleSummary[]; createdAt: string; updatedAt: string }
export type UserPage = PageResult<UserListItem>
export interface UserRolesResponse { user: { id: number; username: string; email: string; isEnabled: YesNo }; roles: UserRoleSummary[]; roleIds: number[] }
export interface UpdateUserInput { username: string }
export interface UpdateUserRolesInput { roleIds: number[] }
export interface UpdatedUsername { id: number; username: string; updatedAt: string }
export interface UserStatusResult { id: number; isEnabled: YesNo }
export interface UserRoleOptions { roles: UserRoleSummary[] }
export interface UserRoleResult { id: number; roleCount: number }

export async function getUsers(query: UserListQuery): Promise<UserPage> {
  return request<UserPage>({ method: 'GET', url: '/api/v1/users', params: query })
}

export async function getUserRoleOptions(): Promise<UserRoleOptions> {
  return request<UserRoleOptions>({ method: 'GET', url: '/api/v1/users/role-options' })
}

export async function updateUser(id: number, input: UpdateUserInput): Promise<UpdatedUsername> {
  return request<UpdatedUsername>({ method: 'PUT', url: `/api/v1/users/${id}`, data: { username: input.username } })
}

export async function updateUserStatus(id: number, isEnabled: YesNo): Promise<UserStatusResult> {
  return request<UserStatusResult>({ method: 'PATCH', url: `/api/v1/users/${id}/status`, data: { isEnabled } })
}

export async function deleteUser(id: number): Promise<Record<string, never>> {
  return request<Record<string, never>>({ method: 'DELETE', url: `/api/v1/users/${id}` })
}

export async function getUserRoles(id: number): Promise<UserRolesResponse> {
  return request<UserRolesResponse>({ method: 'GET', url: `/api/v1/users/${id}/roles` })
}

export async function updateUserRoles(id: number, input: UpdateUserRolesInput): Promise<UserRoleResult> {
  return request<UserRoleResult>({ method: 'PUT', url: `/api/v1/users/${id}/roles`, data: { roleIds: input.roleIds } })
}
