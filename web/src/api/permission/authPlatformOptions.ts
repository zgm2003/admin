import { request } from '@/utils/request'
import type { CreateAuthPlatformInput, LoginType } from './authPlatform'

export type PlatformNumberField =
  | 'accessTTLSeconds'
  | 'refreshTTLSeconds'
  | 'sessionCacheTTLSeconds'
  | 'accessCacheTTLSeconds'
  | 'maxSessions'
export interface AuthPlatformOptions {
  loginTypes: Array<{ value: LoginType; label: string }>
  limits: Record<PlatformNumberField, { minimum: number; maximum: number }>
  defaults: Omit<CreateAuthPlatformInput, 'code' | 'name'>
  codePattern: string
  nameMaxBytes: number
}

export function getAuthPlatformOptions(): Promise<AuthPlatformOptions> {
  return request.get<AuthPlatformOptions>('/api/admin/v1/permission/authplatform/options')
}
