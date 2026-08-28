import type { YesNo } from '../enums/yes-no'
import { request } from '../utils/request'
import type { PageResult } from '../types/pagination'
import type { PageRequest } from '../types/pagination'

export interface AuthPlatformListQuery extends PageRequest { keyword?: string; isEnabled?: YesNo }
export interface AuthPlatformListItem { id:number; code:string; name:string; policyVersion:number; accessTTLSeconds:number; refreshTTLSeconds:number; sessionCacheTTLSeconds:number; accessCacheTTLSeconds:number; bindDevice:YesNo; bindIP:YesNo; maxSessions:number; allowRegister:YesNo; isEnabled:YesNo; isBuiltin:YesNo; createdAt:string; updatedAt:string }
export interface CreateAuthPlatformInput { code:string; name:string; accessTTLSeconds:number; refreshTTLSeconds:number; sessionCacheTTLSeconds:number; accessCacheTTLSeconds:number; bindDevice:YesNo; bindIP:YesNo; maxSessions:number; allowRegister:YesNo; isEnabled:YesNo }
export interface UpdateAuthPlatformInput { name:string; accessTTLSeconds:number; refreshTTLSeconds:number; sessionCacheTTLSeconds:number; accessCacheTTLSeconds:number; bindDevice:YesNo; bindIP:YesNo; maxSessions:number; allowRegister:YesNo }
export interface AuthPlatformStatusResult { id:number; isEnabled:YesNo }

export async function getAuthPlatforms(query: AuthPlatformListQuery): Promise<PageResult<AuthPlatformListItem>> {
  return request<PageResult<AuthPlatformListItem>>({ method: 'GET', url: '/api/admin/v1/auth-platforms', params: query })
}

export async function createAuthPlatform(input: CreateAuthPlatformInput): Promise<{ id: number }> {
  return request<{ id: number }>({
    method: 'POST',
    url: '/api/admin/v1/auth-platforms',
    data: {
      code: input.code,
      name: input.name,
      accessTTLSeconds: input.accessTTLSeconds,
      refreshTTLSeconds: input.refreshTTLSeconds,
      sessionCacheTTLSeconds: input.sessionCacheTTLSeconds,
      accessCacheTTLSeconds: input.accessCacheTTLSeconds,
      bindDevice: input.bindDevice,
      bindIP: input.bindIP,
      maxSessions: input.maxSessions,
      allowRegister: input.allowRegister,
      isEnabled: input.isEnabled,
    },
  })
}

export async function updateAuthPlatform(id: number, input: UpdateAuthPlatformInput): Promise<Record<string, never>> {
  return request<Record<string, never>>({
    method: 'PUT',
    url: `/api/admin/v1/auth-platforms/${id}`,
    data: {
      name: input.name,
      accessTTLSeconds: input.accessTTLSeconds,
      refreshTTLSeconds: input.refreshTTLSeconds,
      sessionCacheTTLSeconds: input.sessionCacheTTLSeconds,
      accessCacheTTLSeconds: input.accessCacheTTLSeconds,
      bindDevice: input.bindDevice,
      bindIP: input.bindIP,
      maxSessions: input.maxSessions,
      allowRegister: input.allowRegister,
    },
  })
}

export async function updateAuthPlatformStatus(id: number, isEnabled: YesNo): Promise<AuthPlatformStatusResult> {
  return request<AuthPlatformStatusResult>({ method: 'PATCH', url: `/api/admin/v1/auth-platforms/${id}/status`, data: { isEnabled } })
}

export async function deleteAuthPlatform(id: number): Promise<Record<string, never>> {
  return request<Record<string, never>>({ method: 'DELETE', url: `/api/admin/v1/auth-platforms/${id}` })
}
