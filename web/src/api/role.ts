import type { YesNo } from '../enums/yes-no'
import { request } from '../utils/request'
import {
  parseEmptyResult,
  parseRoleDefaultResult,
  parseRoleIDResult,
  parseRolePage,
  parseRolePermissionResult,
  parseRolePermissions,
  parseRoleStatusResult,
  type CreateRoleInput,
  type RoleListQuery,
  type UpdateRoleInput,
  type UpdateRolePermissionsInput,
} from './role.contract'

export async function getRoles(query: RoleListQuery) {
  const data = await request<unknown>({
    method: 'GET',
    url: '/api/v1/roles',
    params: query,
  })
  return parseRolePage(data)
}

export async function createRole(input: CreateRoleInput) {
  const data = await request<unknown>({
    method: 'POST',
    url: '/api/v1/roles',
    data: {
      code: input.code,
      name: input.name,
    },
  })
  return parseRoleIDResult(data)
}

export async function updateRole(id: number, input: UpdateRoleInput) {
  const data = await request<unknown>({
    method: 'PUT',
    url: `/api/v1/roles/${id}`,
    data: {
      name: input.name,
    },
  })
  return parseEmptyResult(data)
}

export async function updateRoleStatus(id: number, isEnabled: YesNo) {
  const data = await request<unknown>({
    method: 'PATCH',
    url: `/api/v1/roles/${id}/status`,
    data: { isEnabled },
  })
  return parseRoleStatusResult(data)
}

export async function setDefaultRole(id: number) {
  const data = await request<unknown>({
    method: 'PATCH',
    url: `/api/v1/roles/${id}/default`,
  })
  return parseRoleDefaultResult(data)
}

export async function deleteRole(id: number) {
  const data = await request<unknown>({
    method: 'DELETE',
    url: `/api/v1/roles/${id}`,
  })
  return parseEmptyResult(data)
}

export async function getRolePermissions(id: number) {
  const data = await request<unknown>({
    method: 'GET',
    url: `/api/v1/roles/${id}/permissions`,
  })
  return parseRolePermissions(data)
}

export async function updateRolePermissions(id: number, input: UpdateRolePermissionsInput) {
  const data = await request<unknown>({
    method: 'PUT',
    url: `/api/v1/roles/${id}/permissions`,
    data: {
      menuIds: input.menuIds,
    },
  })
  return parseRolePermissionResult(data)
}
