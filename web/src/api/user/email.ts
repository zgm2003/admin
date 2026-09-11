import { expectExactKeys, expectInteger, expectString } from '@/api/protocol'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

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
  return parseSendCodeResult(
    await request<unknown>({
      method: 'POST',
      url: '/api/admin/v1/user/email/send-code',
      data: input,
    }),
  )
}

export async function bindEmail(input: BindEmailInput): Promise<EmailResult> {
  return parseEmailResult(
    await request<unknown>({ method: 'PUT', url: '/api/admin/v1/user/email', data: input }),
  )
}

function parseSendCodeResult(value: unknown): EmailSendCodeResult {
  const data = expectExactKeys(
    value,
    ['challengeId', 'expiresAt', 'resendAfterSeconds'],
    'email send code',
  )
  const challengeId = expectString(data.challengeId, 'email send code.challengeId')
  const expiresAt = expectString(data.expiresAt, 'email send code.expiresAt')
  const resendAfterSeconds = expectInteger(
    data.resendAfterSeconds,
    'email send code.resendAfterSeconds',
  )
  if (!isChallengeID(challengeId)) {
    throw new ProtocolError('email send code.challengeId is invalid')
  }
  if (!isTimestamp(expiresAt)) throw new ProtocolError('email send code.expiresAt is invalid')
  if (resendAfterSeconds < 0 || resendAfterSeconds > 86400) {
    throw new ProtocolError('email send code.resendAfterSeconds is invalid')
  }
  return { challengeId, expiresAt, resendAfterSeconds }
}

function parseEmailResult(value: unknown): EmailResult {
  const data = expectExactKeys(value, ['email'], 'email result')
  const email = expectString(data.email, 'email result.email')
  if (!isCanonicalEmail(email)) throw new ProtocolError('email result.email is invalid')
  return { email }
}

function isCanonicalEmail(value: string): boolean {
  if (value === '' || value.length > 254 || value !== value.trim() || value !== value.toLowerCase()) {
    return false
  }
  if ([...value].some((character) => {
    const code = character.charCodeAt(0)
    return code <= 0x1f || code === 0x7f
  })) return false
  const parts = value.split('@')
  return parts.length === 2 && parts[0] !== '' && parts[1] !== ''
}

function isChallengeID(value: string): boolean {
  return value.length > 0 && value.length <= 128 && ![...value].some((character) => {
    const code = character.charCodeAt(0)
    return code <= 0x20 || code === 0x7f
  })
}

function isTimestamp(value: string): boolean {
  return (
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) &&
    !Number.isNaN(Date.parse(value))
  )
}
