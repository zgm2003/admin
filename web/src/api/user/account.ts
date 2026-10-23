import { YesNo } from '@/enums/yesNo'
import type { PageRequest, PageResult } from '@/types/pagination'
import { request } from '@/utils/request'

export interface UserListQuery extends PageRequest {
  keyword?: string
  status?: YesNo
  roleId?: number
}

export interface UserRoleSummary {
  id: number
  code: string
  name: string
  status: YesNo
}

export interface UserActions {
  update: boolean
  status: boolean
  delete: boolean
  authorize: boolean
}
export interface UserActionLabels {
  update: string
  status: string
  delete: string
  authorize: string
}
export interface UserAssignmentRole extends UserRoleSummary {
  selectable: boolean
  locked: boolean
}
export interface UserListItem {
  actions: UserActions
  actionLabels: UserActionLabels
  id: number
  username: string
  email: string
  phone: string | null
  status: YesNo
  roles: UserRoleSummary[]
  createdAt: string
  updatedAt: string
}

export type UserPage = PageResult<UserListItem>

export interface UserRolesResponse {
  user: { id: number; username: string; email: string; phone: string | null; status: YesNo }
  roles: UserAssignmentRole[]
  roleIds: number[]
}

export interface UpdateUserInput {
  username: string
}

export interface UpdateUserRolesInput {
  roleIds: number[]
}

export interface UpdatedProfile {
  id: number
  username: string
  phone: string | null
  updatedAt: string
}

export interface UserStatusResult {
  id: number
  status: YesNo
}

export interface UserRoleOptions {
  roles: UserRoleSummary[]
}

export interface UserRoleResult {
  id: number
  roleCount: number
}

export async function getUsers(query: UserListQuery): Promise<UserPage> {
  return request.get<UserPage>('/api/admin/v1/user/account', { params: query })
}

export async function getUserRoleOptions(): Promise<UserRoleOptions> {
  return request.get<UserRoleOptions>('/api/admin/v1/user/account/role-options')
}

export async function updateUser(id: number, input: UpdateUserInput): Promise<UpdatedProfile> {
  return request.put<UpdatedProfile>(`/api/admin/v1/user/account/${id}`, input)
}

export async function updateUserStatus(id: number, status: YesNo): Promise<UserStatusResult> {
  return request.patch<UserStatusResult>(`/api/admin/v1/user/account/${id}/status`, { status })
}

export async function deleteUser(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`/api/admin/v1/user/account/${id}`)
}

export async function getUserRoles(id: number): Promise<UserRolesResponse> {
  return request.get<UserRolesResponse>(`/api/admin/v1/user/account/${id}/role`)
}

export async function updateUserRoles(
  id: number,
  input: UpdateUserRolesInput,
): Promise<UserRoleResult> {
  return request.put<UserRoleResult>(`/api/admin/v1/user/account/${id}/role`, {
    roleIds: input.roleIds,
  })
}

export interface UserFormOptions {
  usernameMinLength: number
  usernameMaxLength: number
  usernamePattern: string
}
export async function getUserFormOptions(): Promise<UserFormOptions> {
  return request.get<UserFormOptions>('/api/admin/v1/user/account/options')
}
