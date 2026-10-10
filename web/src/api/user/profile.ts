import { request } from '@/utils/request'

export interface AccountProfile {
  userId: number
  username: string
  email: string
  phone: string | null
  avatar: string
  birthday: string | null
  gender: number
}

export interface UpdateAccountProfileInput {
  username: string
  avatar: string
  birthday: string | null
  gender: number
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
  return request.get<AccountProfile>('/api/admin/v1/user/profile')
}

export async function updateAccountProfile(
  input: UpdateAccountProfileInput,
): Promise<UpdateAccountProfileResult> {
  return request.put<UpdateAccountProfileResult>('/api/admin/v1/user/profile', input)
}

export async function changePassword(input: ChangePasswordInput): Promise<void> {
  return request.post<void>('/api/admin/v1/user/password', input)
}

export interface SetPasswordInput {
  newPassword: string
  confirmPassword: string
}

export async function setPassword(input: SetPasswordInput): Promise<void> {
  return request.post<void>('/api/admin/v1/user/password/set', input)
}

export async function sendPasswordCode(
  loginType: PasswordCodeLoginType,
): Promise<PasswordCodeResult> {
  return request.post<PasswordCodeResult>('/api/admin/v1/user/password/send-code', { loginType })
}

export async function changePasswordByCode(input: ChangePasswordByCodeInput): Promise<void> {
  return request.put<void>('/api/admin/v1/user/password/by-code', input)
}
