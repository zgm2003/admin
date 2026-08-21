import type { YesNo } from '../enums/yes-no'
import { request } from '../utils/request'
import {
  parseAuthPlatformDeployment,
  parseAuthPlatformIDResult,
  parseAuthPlatformPage,
  parseAuthPlatformStatusResult,
  parseEmptyAuthPlatformResult,
  type AuthPlatformDeployment,
  type AuthPlatformListItem,
  type AuthPlatformListQuery,
  type AuthPlatformStatusResult,
  type CreateAuthPlatformInput,
  type UpdateAuthPlatformInput,
} from './auth-platform.contract'
import type { PageResult } from '../types/pagination'

export async function getAuthPlatforms(query: AuthPlatformListQuery): Promise<PageResult<AuthPlatformListItem>> {
  const data = await request<unknown>({ method: 'GET', url: '/api/v1/auth-platforms', params: query })
  return parseAuthPlatformPage(data)
}

export async function getAuthPlatformDeployment(): Promise<AuthPlatformDeployment> {
  const data = await request<unknown>({ method: 'GET', url: '/api/v1/auth-platforms/deployment' })
  return parseAuthPlatformDeployment(data)
}

export async function createAuthPlatform(input: CreateAuthPlatformInput): Promise<{ id: number }> {
  const data = await request<unknown>({
    method: 'POST',
    url: '/api/v1/auth-platforms',
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
  return parseAuthPlatformIDResult(data)
}

export async function updateAuthPlatform(id: number, input: UpdateAuthPlatformInput): Promise<Record<string, never>> {
  const data = await request<unknown>({
    method: 'PUT',
    url: `/api/v1/auth-platforms/${id}`,
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
  return parseEmptyAuthPlatformResult(data)
}

export async function updateAuthPlatformStatus(id: number, isEnabled: YesNo): Promise<AuthPlatformStatusResult> {
  const data = await request<unknown>({ method: 'PATCH', url: `/api/v1/auth-platforms/${id}/status`, data: { isEnabled } })
  return parseAuthPlatformStatusResult(data)
}

export async function deleteAuthPlatform(id: number): Promise<Record<string, never>> {
  const data = await request<unknown>({ method: 'DELETE', url: `/api/v1/auth-platforms/${id}` })
  return parseEmptyAuthPlatformResult(data)
}
