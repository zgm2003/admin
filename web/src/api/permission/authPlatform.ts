import { type YesNo } from '@/enums/yesNo'
import { request } from '@/utils/request'
import type { PageResult, PageRequest } from '@/types/pagination'

export type LoginType = string

export interface AuthPlatformListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}

export interface AuthPlatformListItem {
  presentation: {
    loginTypes: Array<{ value: LoginType; label: string }>
    maxSessionsLabel: string
    deleteReason: string
  }
  actions: { update: boolean; status: boolean; delete: boolean }
  id: number
  code: string
  name: string
  loginTypes: LoginType[]
  policyVersion: number
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: YesNo
  bindIP: YesNo
  maxSessions: number
  allowRegister: YesNo
  isEnabled: YesNo
  isBuiltin: YesNo
  createdAt: string
  updatedAt: string
}

export interface CreateAuthPlatformInput {
  code: string
  name: string
  loginTypes: LoginType[]
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: YesNo
  bindIP: YesNo
  maxSessions: number
  allowRegister: YesNo
  isEnabled: YesNo
}

export interface UpdateAuthPlatformInput {
  name: string
  loginTypes: LoginType[]
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: YesNo
  bindIP: YesNo
  maxSessions: number
  allowRegister: YesNo
}

export interface AuthPlatformStatusResult {
  id: number
  isEnabled: YesNo
}

export async function getAuthPlatforms(
  query: AuthPlatformListQuery,
): Promise<PageResult<AuthPlatformListItem>> {
  return request.get<PageResult<AuthPlatformListItem>>('/api/admin/v1/permission/authplatform', {
    params: query,
  })
}

export async function createAuthPlatform(input: CreateAuthPlatformInput): Promise<{ id: number }> {
  return request.post<{ id: number }>('/api/admin/v1/permission/authplatform', {
    code: input.code,
    name: input.name,
    loginTypes: input.loginTypes,
    accessTTLSeconds: input.accessTTLSeconds,
    refreshTTLSeconds: input.refreshTTLSeconds,
    sessionCacheTTLSeconds: input.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: input.accessCacheTTLSeconds,
    bindDevice: input.bindDevice,
    bindIP: input.bindIP,
    maxSessions: input.maxSessions,
    allowRegister: input.allowRegister,
    isEnabled: input.isEnabled,
  })
}

export async function updateAuthPlatform(
  id: number,
  input: UpdateAuthPlatformInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>>(`/api/admin/v1/permission/authplatform/${id}`, {
    name: input.name,
    loginTypes: input.loginTypes,
    accessTTLSeconds: input.accessTTLSeconds,
    refreshTTLSeconds: input.refreshTTLSeconds,
    sessionCacheTTLSeconds: input.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: input.accessCacheTTLSeconds,
    bindDevice: input.bindDevice,
    bindIP: input.bindIP,
    maxSessions: input.maxSessions,
    allowRegister: input.allowRegister,
  })
}

export async function updateAuthPlatformStatus(
  id: number,
  isEnabled: YesNo,
): Promise<AuthPlatformStatusResult> {
  return request.patch<AuthPlatformStatusResult>(
    `/api/admin/v1/permission/authplatform/${id}/status`,
    { isEnabled },
  )
}

export async function deleteAuthPlatform(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`/api/admin/v1/permission/authplatform/${id}`)
}
