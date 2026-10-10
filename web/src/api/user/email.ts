import { request } from '@/utils/request'

export type EmailChangeAction = number

export interface EmailChangeLogItem {
  id: number
  action: EmailChangeAction
  actionLabel: string
  oldEmail: string | null
  newEmail: string
  platform: string
  createdAt: string
}

export interface EmailChangeLogPage {
  list: EmailChangeLogItem[]
  total: number
  page: number
  pageSize: number
}

export type IdentityTarget = 'current' | 'next'

export interface EmailSendCodeInput {
  target: IdentityTarget
  email?: string
  challengeId?: string
}

export interface EmailSendCodeResult {
  challengeId: string
  expiresAt: string
  resendAfterSeconds: number
}

export interface BindEmailInput {
  currentChallengeId?: string
  currentCode?: string
  nextEmail: string
  nextChallengeId: string
  nextCode: string
}

export interface EmailResult {
  email: string
}

export async function sendEmailCode(input: EmailSendCodeInput): Promise<EmailSendCodeResult> {
  return request.post<EmailSendCodeResult>('/api/admin/v1/user/email/send-code', input)
}

export async function bindEmail(input: BindEmailInput): Promise<EmailResult> {
  return request.put<EmailResult>('/api/admin/v1/user/email', input)
}

export async function getEmailChangeLogs(
  userId: number,
  query: { page: number; pageSize: number },
): Promise<EmailChangeLogPage> {
  return request.get<EmailChangeLogPage>(`/api/admin/v1/user/account/${userId}/email-change-log`, {
    params: query,
  })
}
