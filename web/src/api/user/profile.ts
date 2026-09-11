import { request } from '@/utils/request'
import { ProtocolError } from '@/types/http'
import { expectEmptyObject, expectExactKeys, expectInteger, expectString } from '@/api/protocol'

export interface AccountProfile {
  userId: number
  username: string
  email: string
  phone: string | null
  avatar: string
  birthday: string | null
  gender: 0 | 1 | 2
}

export interface UpdateAccountProfileInput {
  username: string
  avatar: string
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

export type PasswordCodeLoginType = 'email' | 'phone'

export interface PasswordCodeResult {
  challengeId: string
  expiresAt: string
  resendAfterSeconds: number
}

export interface ChangePasswordByCodeInput {
  loginType: PasswordCodeLoginType
  challengeId: string
  code: string
  newPassword: string
  confirmPassword: string
}

export async function getAccountProfile(): Promise<AccountProfile> {
  return parseAccountProfile(
    await request<unknown>({ method: 'GET', url: '/api/admin/v1/user/profile' }),
  )
}

export async function updateAccountProfile(
  input: UpdateAccountProfileInput,
): Promise<UpdateAccountProfileResult> {
  return parseUpdatedAccountProfile(
    await request<unknown>({ method: 'PUT', url: '/api/admin/v1/user/profile', data: input }),
  )
}

export async function changePassword(input: ChangePasswordInput): Promise<void> {
  expectEmptyObject(
    await request<unknown>({
      method: 'POST',
      url: '/api/admin/v1/user/password',
      data: input,
    }),
    'change password result',
  )
}

export interface SetPasswordInput {
  newPassword: string
  confirmPassword: string
}

export async function setPassword(input: SetPasswordInput): Promise<void> {
  expectEmptyObject(
    await request<unknown>({
      method: 'POST',
      url: '/api/admin/v1/user/password/set',
      data: input,
    }),
    'set password result',
  )
}

export async function sendPasswordCode(
  loginType: PasswordCodeLoginType,
): Promise<PasswordCodeResult> {
  return parsePasswordCodeResult(
    await request<unknown>({
      method: 'POST',
      url: '/api/admin/v1/user/password/send-code',
      data: { loginType },
    }),
  )
}

export async function changePasswordByCode(input: ChangePasswordByCodeInput): Promise<void> {
  expectEmptyObject(
    await request<unknown>({
      method: 'PUT',
      url: '/api/admin/v1/user/password/by-code',
      data: input,
    }),
    'password code result',
  )
}

function parsePasswordCodeResult(value: unknown): PasswordCodeResult {
  const data = expectExactKeys(
    value,
    ['challengeId', 'expiresAt', 'resendAfterSeconds'],
    'password send code',
  )
  const challengeId = expectString(data.challengeId, 'password send code.challengeId')
  const expiresAt = expectString(data.expiresAt, 'password send code.expiresAt')
  const resendAfterSeconds = expectInteger(
    data.resendAfterSeconds,
    'password send code.resendAfterSeconds',
  )
  if (
    challengeId.length === 0 ||
    challengeId.length > 128 ||
    [...challengeId].some((character) => {
      const code = character.charCodeAt(0)
      return code <= 0x20 || code === 0x7f
    })
  ) {
    throw new ProtocolError('password send code.challengeId is invalid')
  }
  if (
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(
      expiresAt,
    ) ||
    Number.isNaN(Date.parse(expiresAt))
  ) {
    throw new ProtocolError('password send code.expiresAt is invalid')
  }
  if (resendAfterSeconds < 0 || resendAfterSeconds > 86400) {
    throw new ProtocolError('password send code.resendAfterSeconds is invalid')
  }
  return { challengeId, expiresAt, resendAfterSeconds }
}

function parseAccountProfile(value: unknown): AccountProfile {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, [
      'userId',
      'username',
      'email',
      'phone',
      'avatar',
      'birthday',
      'gender',
    ]) ||
    !isPositiveInteger(value.userId) ||
    typeof value.username !== 'string' ||
    typeof value.email !== 'string' ||
    !isNullableString(value.phone) ||
    typeof value.avatar !== 'string' ||
    !isNullableDate(value.birthday) ||
    !isGender(value.gender)
  ) {
    throw new ProtocolError('account profile response is invalid')
  }
  return {
    userId: value.userId,
    username: value.username,
    email: value.email,
    phone: value.phone,
    avatar: value.avatar,
    birthday: value.birthday,
    gender: value.gender,
  }
}

function parseUpdatedAccountProfile(value: unknown): UpdateAccountProfileResult {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, [
      'userId',
      'username',
      'email',
      'phone',
      'avatar',
      'birthday',
      'gender',
      'updatedAt',
    ]) ||
    !isPositiveInteger(value.userId) ||
    typeof value.username !== 'string' ||
    typeof value.email !== 'string' ||
    !isNullableString(value.phone) ||
    typeof value.avatar !== 'string' ||
    !isNullableDate(value.birthday) ||
    !isGender(value.gender) ||
    typeof value.updatedAt !== 'string'
  ) {
    throw new ProtocolError('updated account profile response is invalid')
  }
  return {
    userId: value.userId,
    username: value.username,
    email: value.email,
    phone: value.phone,
    avatar: value.avatar,
    birthday: value.birthday,
    gender: value.gender,
    updatedAt: value.updatedAt,
  }
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
