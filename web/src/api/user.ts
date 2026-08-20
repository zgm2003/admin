import type { YesNo } from '../enums/yes-no'
import { request } from '../utils/request'
import {
  parseEmptyUserResult, parseUpdatedUsername, parseUserPage, parseUserRoleOptions,
  parseUserRoleResult, parseUserRoles, parseUserStatusResult,
  type UpdateUserInput, type UpdateUserRolesInput, type UserListQuery,
} from './user.contract'

export async function getUsers(query: UserListQuery) {
  const data = await request<unknown>({ method:'GET', url:'/api/v1/users', params:query })
  return parseUserPage(data)
}
export async function getUserRoleOptions() {
  const data = await request<unknown>({ method:'GET', url:'/api/v1/users/role-options' })
  return parseUserRoleOptions(data)
}
export async function updateUser(id: number, input: UpdateUserInput) {
  const data = await request<unknown>({ method:'PUT', url:`/api/v1/users/${id}`, data:{ username:input.username } })
  return parseUpdatedUsername(data)
}
export async function updateUserStatus(id: number, isEnabled: YesNo) {
  const data = await request<unknown>({ method:'PATCH', url:`/api/v1/users/${id}/status`, data:{ isEnabled } })
  return parseUserStatusResult(data)
}
export async function deleteUser(id: number) {
  const data = await request<unknown>({ method:'DELETE', url:`/api/v1/users/${id}` })
  return parseEmptyUserResult(data)
}
export async function getUserRoles(id: number) {
  const data = await request<unknown>({ method:'GET', url:`/api/v1/users/${id}/roles` })
  return parseUserRoles(data)
}
export async function updateUserRoles(id: number, input: UpdateUserRolesInput) {
  const data = await request<unknown>({ method:'PUT', url:`/api/v1/users/${id}/roles`, data:{ roleIds:input.roleIds } })
  return parseUserRoleResult(data)
}
