import { request } from '@/utils/request'

export type PhoneChangeAction = number

export interface PhoneChangeLogItem {
  id: number
  action: PhoneChangeAction
  actionLabel: string
  oldPhone: string | null
  newPhone: string
  platform: string
  createdAt: string
}

export interface PhoneChangeLogPage {
  list: PhoneChangeLogItem[]
  total: number
  page: number
  pageSize: number
}

export type IdentityTarget = 'current' | 'next'

export interface PhoneSendCodeInput {
  target: IdentityTarget
  phone?: string
  challengeId?: string
}

export interface PhoneSendCodeResult {
  challengeId: string
  expiresAt: string
  resendAfterSeconds: number
}

export interface BindPhoneInput {
  currentChallengeId?: string
  currentCode?: string
  nextPhone: string
  nextChallengeId: string
  nextCode: string
}

export interface PhoneResult {
  phone: string
}

export async function sendPhoneCode(input: PhoneSendCodeInput): Promise<PhoneSendCodeResult> {
  return request.post<PhoneSendCodeResult>('/api/admin/v1/user/phone/send-code', input)
}

export async function bindPhone(input: BindPhoneInput): Promise<PhoneResult> {
  return request.put<PhoneResult>('/api/admin/v1/user/phone', input)
}

export async function getPhoneChangeLogs(
  userId: number,
  query: { page: number; pageSize: number },
): Promise<PhoneChangeLogPage> {
  return request.get<PhoneChangeLogPage>(`/api/admin/v1/user/account/${userId}/phone-change-log`, {
    params: query,
  })
}
