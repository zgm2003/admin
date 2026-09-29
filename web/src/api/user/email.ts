import { expectExactKeys, expectInteger, expectString } from '@/api/protocol'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

export type EmailChangeAction = 1 | 2

export interface EmailChangeLogItem {
  id: number
  action: EmailChangeAction
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
  return parseSendCodeResult(
    await request({
      method: 'POST',
      url: '/api/admin/v1/user/email/send-code',
      data: input,
    }),
  )
}

export async function bindEmail(input: BindEmailInput): Promise<EmailResult> {
  return parseEmailResult(
    await request({ method: 'PUT', url: '/api/admin/v1/user/email', data: input }),
  )
}

export async function getEmailChangeLogs(
  userId: number,
  query: { page: number; pageSize: number },
): Promise<EmailChangeLogPage> {
  if (!Number.isInteger(userId) || userId < 1) throw new ProtocolError('user id is invalid')
  return parseEmailChangeLogPage(
    await request({
      method: 'GET',
      url: `/api/admin/v1/user/account/${userId}/email-change-log`,
      params: query,
    }),
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

function parseEmailChangeLogPage(value: unknown): EmailChangeLogPage {
  const data = expectExactKeys(value, ['list', 'total', 'page', 'pageSize'], 'email change logs')
  if (!Array.isArray(data.list)) throw new ProtocolError('email change logs response is invalid')
  const total = expectInteger(data.total, 'email change logs.total')
  const page = expectInteger(data.page, 'email change logs.page')
  const pageSize = expectInteger(data.pageSize, 'email change logs.pageSize')
  if (total < 0 || page < 1 || pageSize < 1 || pageSize > 100)
    throw new ProtocolError('email change logs response is invalid')
  return {
    list: data.list.map(parseEmailChangeLogItem),
    total,
    page,
    pageSize,
  }
}

function parseEmailChangeLogItem(value: unknown): EmailChangeLogItem {
  const data = expectExactKeys(
    value,
    ['id', 'action', 'oldEmail', 'newEmail', 'platform', 'createdAt'],
    'email change log item',
  )
  if (
    typeof data.id !== 'number' ||
    !Number.isInteger(data.id) ||
    data.id < 1 ||
    (data.action !== 1 && data.action !== 2) ||
    !isNullableCanonicalEmail(data.oldEmail) ||
    typeof data.newEmail !== 'string' ||
    !isCanonicalEmail(data.newEmail) ||
    typeof data.platform !== 'string' ||
    data.platform.trim() === '' ||
    typeof data.createdAt !== 'string' ||
    !isTimestamp(data.createdAt)
  ) {
    throw new ProtocolError('email change log item is invalid')
  }
  if (data.action === 1 && data.oldEmail === null) {
    throw new ProtocolError('email change log change action requires oldEmail')
  }
  if (data.action === 2 && data.oldEmail !== null) {
    throw new ProtocolError('email change log bind action forbids oldEmail')
  }
  const id = expectInteger(data.id, 'email change log item.id')
  const platform = expectString(data.platform, 'email change log item.platform')
  const createdAt = expectString(data.createdAt, 'email change log item.createdAt')
  const newEmail = expectString(data.newEmail, 'email change log item.newEmail')
  if (id < 1 || platform.trim() === '' || !isCanonicalEmail(newEmail) || !isTimestamp(createdAt))
    throw new ProtocolError('email change log item is invalid')
  return {
    id,
    action: data.action,
    oldEmail: data.oldEmail,
    newEmail,
    platform,
    createdAt,
  }
}

function isNullableCanonicalEmail(value: unknown): value is string | null {
  return value === null || (typeof value === 'string' && isCanonicalEmail(value))
}

function isCanonicalEmail(value: string): boolean {
  if (
    value === '' ||
    value.length > 254 ||
    value !== value.trim() ||
    value !== value.toLowerCase()
  ) {
    return false
  }
  if (
    [...value].some((character) => {
      const code = character.charCodeAt(0)
      return code <= 0x1f || code === 0x7f
    })
  )
    return false
  const parts = value.split('@')
  return parts.length === 2 && parts[0] !== '' && parts[1] !== ''
}

function isChallengeID(value: string): boolean {
  return (
    value.length > 0 &&
    value.length <= 128 &&
    ![...value].some((character) => {
      const code = character.charCodeAt(0)
      return code <= 0x20 || code === 0x7f
    })
  )
}

function isTimestamp(value: string): boolean {
  return (
    /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) &&
    !Number.isNaN(Date.parse(value))
  )
}
