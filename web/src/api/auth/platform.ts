import { isYesNo, type YesNo } from '@/enums/yes-no'
import { request } from '@/utils/request'
import type { PageResult, PageRequest } from '@/types/pagination'
import {
  expectEmptyObject,
  expectId,
  expectInteger,
  expectPage,
  expectRecord,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'

export interface AuthPlatformListQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}
export interface AuthPlatformListItem {
  id: number
  code: string
  name: string
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
  return expectPage(
    await request<unknown>({
      method: 'GET',
      url: '/api/admin/v1/auth-platforms',
      params: query,
    }),
    parseAuthPlatform,
    'auth platforms',
  )
}

export async function createAuthPlatform(input: CreateAuthPlatformInput): Promise<{ id: number }> {
  return expectId(
    await request<unknown>({
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
    }),
    'auth platform create result',
  )
}

export async function updateAuthPlatform(
  id: number,
  input: UpdateAuthPlatformInput,
): Promise<Record<string, never>> {
  expectEmptyObject(
    await request<unknown>({
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
    }),
    'auth platform update result',
  )
  return {}
}

export async function updateAuthPlatformStatus(
  id: number,
  isEnabled: YesNo,
): Promise<AuthPlatformStatusResult> {
  const result = expectRecord(
    await request<unknown>({
      method: 'PATCH',
      url: `/api/admin/v1/auth-platforms/${id}/status`,
      data: { isEnabled },
    }),
    'auth platform status result',
  )
  if (!isYesNo(result.isEnabled)) throw new ProtocolError('auth platform status is invalid')
  return { id: expectInteger(result.id, 'auth platform status id'), isEnabled: result.isEnabled }
}

export async function deleteAuthPlatform(id: number): Promise<Record<string, never>> {
  expectEmptyObject(
    await request<unknown>({
      method: 'DELETE',
      url: `/api/admin/v1/auth-platforms/${id}`,
    }),
    'auth platform delete result',
  )
  return {}
}

function parseAuthPlatform(value: unknown, index: number): AuthPlatformListItem {
  const item = expectRecord(value, `auth platforms.list[${index}]`)
  const bindDevice = item.bindDevice
  const bindIP = item.bindIP
  const allowRegister = item.allowRegister
  const isEnabled = item.isEnabled
  const isBuiltin = item.isBuiltin
  if (
    !isYesNo(bindDevice) ||
    !isYesNo(bindIP) ||
    !isYesNo(allowRegister) ||
    !isYesNo(isEnabled) ||
    !isYesNo(isBuiltin)
  ) {
    throw new ProtocolError('auth platform yes/no field is invalid')
  }
  return {
    id: expectInteger(item.id, 'auth platform.id'),
    code: expectString(item.code, 'auth platform.code'),
    name: expectString(item.name, 'auth platform.name'),
    policyVersion: expectInteger(item.policyVersion, 'auth platform.policyVersion'),
    accessTTLSeconds: expectInteger(item.accessTTLSeconds, 'auth platform.accessTTLSeconds'),
    refreshTTLSeconds: expectInteger(item.refreshTTLSeconds, 'auth platform.refreshTTLSeconds'),
    sessionCacheTTLSeconds: expectInteger(
      item.sessionCacheTTLSeconds,
      'auth platform.sessionCacheTTLSeconds',
    ),
    accessCacheTTLSeconds: expectInteger(
      item.accessCacheTTLSeconds,
      'auth platform.accessCacheTTLSeconds',
    ),
    bindDevice,
    bindIP,
    maxSessions: expectInteger(item.maxSessions, 'auth platform.maxSessions'),
    allowRegister,
    isEnabled,
    isBuiltin,
    createdAt: expectString(item.createdAt, 'auth platform.createdAt'),
    updatedAt: expectString(item.updatedAt, 'auth platform.updatedAt'),
  }
}
