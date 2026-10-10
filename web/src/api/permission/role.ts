import { type YesNo } from '@/enums/yesNo'
import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface RoleListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}

export interface RoleActions {
  update: boolean
  status: boolean
  setDefault: boolean
  delete: boolean
  authorize: boolean
}
export interface RoleActionLabels {
  update: string
  status: string
  setDefault: string
  delete: string
  authorize: string
}
export interface RoleListItem {
  actions: RoleActions
  actionLabels: RoleActionLabels
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
  return request.get<PageResult<RoleListItem>>('/api/admin/v1/permission/role', { params: query })
}

export async function createRole(input: CreateRoleInput): Promise<{ id: number }> {
  return request.post<{ id: number }>('/api/admin/v1/permission/role', {
    code: input.code,
    name: input.name,
  })
}

export async function updateRole(
  id: number,
  input: UpdateRoleInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>>(`/api/admin/v1/permission/role/${id}`, {
    name: input.name,
  })
}

export async function updateRoleStatus(id: number, isEnabled: YesNo): Promise<RoleStatusResult> {
  return request.patch<RoleStatusResult>(`/api/admin/v1/permission/role/${id}/status`, {
    isEnabled,
  })
}

export async function setDefaultRole(id: number): Promise<RoleDefaultResult> {
  return request.patch<RoleDefaultResult>(`/api/admin/v1/permission/role/${id}/default`, undefined)
}

export async function deleteRole(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`/api/admin/v1/permission/role/${id}`)
}

export async function getRolePermissions(id: number): Promise<RolePermissionsResponse> {
  return request.get<RolePermissionsResponse>(`/api/admin/v1/permission/role/${id}/permission`)
}

export async function updateRolePermissions(
  id: number,
  input: UpdateRolePermissionsInput,
): Promise<RolePermissionResult> {
  return request.put<RolePermissionResult>(`/api/admin/v1/permission/role/${id}/permission`, {
    menuIds: input.menuIds,
  })
}

export interface RoleFormOptions {
  codePattern: string
  nameMinLength: number
  nameMaxLength: number
}
export async function getRoleFormOptions(): Promise<RoleFormOptions> {
  return request.get<RoleFormOptions>('/api/admin/v1/permission/role/options')
}
