import { ProtocolError, request } from '../../utils/request'

export interface AccountProfile {
  userId: number
  username: string
  email: string
  phone: string | null
  birthday: string | null
  gender: 0 | 1 | 2
}

export interface UpdateAccountProfileInput {
  username: string
  phone: string | null
  birthday: string | null
  gender: 0 | 1 | 2
}

export interface UpdateAccountProfileResult extends AccountProfile {
  updatedAt: string
}

export interface ChangePasswordInput {
  currentPassword: string
  newPassword: string
  confirmPassword: string
}

export async function getAccountProfile(): Promise<AccountProfile> {
  return parseAccountProfile(await request<unknown>({ method: 'GET', url: '/api/admin/v1/account/profile' }))
}

export async function updateAccountProfile(input: UpdateAccountProfileInput): Promise<UpdateAccountProfileResult> {
  return parseUpdatedAccountProfile(await request<unknown>({ method: 'PUT', url: '/api/admin/v1/account/profile', data: input }))
}

export async function changePassword(input: ChangePasswordInput): Promise<void> {
  await request<Record<string, never>>({ method: 'POST', url: '/api/admin/v1/account/password', data: input })
}

function parseAccountProfile(value: unknown): AccountProfile {
  if (!isRecord(value) || !hasExactKeys(value, ['userId', 'username', 'email', 'phone', 'birthday', 'gender']) ||
    !isPositiveInteger(value.userId) || typeof value.username !== 'string' || typeof value.email !== 'string' ||
    !isNullableString(value.phone) || !isNullableDate(value.birthday) || !isGender(value.gender)) {
    throw new ProtocolError('account profile response is invalid')
  }
  return { userId: value.userId, username: value.username, email: value.email, phone: value.phone, birthday: value.birthday, gender: value.gender }
}

function parseUpdatedAccountProfile(value: unknown): UpdateAccountProfileResult {
  if (!isRecord(value) || !hasExactKeys(value, ['userId', 'username', 'email', 'phone', 'birthday', 'gender', 'updatedAt']) ||
    !isPositiveInteger(value.userId) || typeof value.username !== 'string' || typeof value.email !== 'string' ||
    !isNullableString(value.phone) || !isNullableDate(value.birthday) || !isGender(value.gender) || typeof value.updatedAt !== 'string') {
    throw new ProtocolError('updated account profile response is invalid')
  }
  return { userId: value.userId, username: value.username, email: value.email, phone: value.phone, birthday: value.birthday, gender: value.gender, updatedAt: value.updatedAt }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function hasExactKeys(value: Record<string, unknown>, expected: readonly string[]): boolean {
  const actual = Object.keys(value).sort()
  const keys = [...expected].sort()
  return actual.length === keys.length && actual.every((key, index) => key === keys[index])
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

function isNullableString(value: unknown): value is string | null {
  return value === null || typeof value === 'string'
}

function isNullableDate(value: unknown): value is string | null {
  return value === null || (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value))
}

function isGender(value: unknown): value is 0 | 1 | 2 {
  return value === 0 || value === 1 || value === 2
}
