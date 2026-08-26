import type { YesNo } from '../enums/yes-no'
import { request } from '../utils/request'
import type { PageRequest, PageResult } from '../types/pagination'

export interface RoleListQuery extends PageRequest { keyword?: string; isEnabled?: YesNo }
export interface RoleListItem { id:number; code:string; name:string; isDefault:YesNo; isEnabled:YesNo; userCount:number; permissionCount:number; createdAt:string; updatedAt:string }
export type RolePermissionMenuType = 'directory'|'page'|'action'
export interface RolePermissionTreeNode { id:number; parentId:number|null; menuType:RolePermissionMenuType; code:string; name:string; isEnabled:YesNo; children:RolePermissionTreeNode[] }
export interface CreateRoleInput { code:string; name:string }
export interface UpdateRoleInput { name:string }
export interface RoleStatusResult { id:number; isEnabled:YesNo }
export interface RoleDefaultResult { id:number; isDefault:YesNo }
export interface RolePermissionsResponse { role:{id:number;code:string;name:string;isDefault:YesNo;isEnabled:YesNo}; menuTree:RolePermissionTreeNode[]; menuIds:number[] }
export interface UpdateRolePermissionsInput { menuIds:number[] }
export interface RolePermissionResult { id:number; permissionCount:number }

export function getRoles(query: RoleListQuery): Promise<PageResult<RoleListItem>> { return request<PageResult<RoleListItem>>({ method: 'GET', url: '/api/v1/roles', params: query }) }
export function createRole(input: CreateRoleInput): Promise<{ id: number }> { return request<{ id: number }>({ method: 'POST', url: '/api/v1/roles', data: { code: input.code, name: input.name } }) }
export function updateRole(id: number, input: UpdateRoleInput): Promise<Record<string, never>> { return request<Record<string, never>>({ method: 'PUT', url: `/api/v1/roles/${id}`, data: { name: input.name } }) }
export function updateRoleStatus(id: number, isEnabled: YesNo): Promise<RoleStatusResult> { return request<RoleStatusResult>({ method: 'PATCH', url: `/api/v1/roles/${id}/status`, data: { isEnabled } }) }
export function setDefaultRole(id: number): Promise<RoleDefaultResult> { return request<RoleDefaultResult>({ method: 'PATCH', url: `/api/v1/roles/${id}/default` }) }
export function deleteRole(id: number): Promise<Record<string, never>> { return request<Record<string, never>>({ method: 'DELETE', url: `/api/v1/roles/${id}` }) }
export function getRolePermissions(id: number): Promise<RolePermissionsResponse> { return request<RolePermissionsResponse>({ method: 'GET', url: `/api/v1/roles/${id}/permissions` }) }
export function updateRolePermissions(id: number, input: UpdateRolePermissionsInput): Promise<RolePermissionResult> { return request<RolePermissionResult>({ method: 'PUT', url: `/api/v1/roles/${id}/permissions`, data: { menuIds: input.menuIds } }) }
