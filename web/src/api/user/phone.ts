import { expectExactKeys, expectInteger, expectString } from '@/api/protocol'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

export type PhoneChangeAction = 1 | 2
export interface PhoneChangeLogItem {
  id: number
  action: PhoneChangeAction
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
  return parseSendCodeResult(
    await request({
      method: 'POST',
      url: '/api/admin/v1/user/phone/send-code',
      data: input,
    }),
  )
}

export async function bindPhone(input: BindPhoneInput): Promise<PhoneResult> {
  return parsePhoneResult(
    await request({ method: 'PUT', url: '/api/admin/v1/user/phone', data: input }),
  )
}

export async function getPhoneChangeLogs(
  userId: number,
  query: { page: number; pageSize: number },
): Promise<PhoneChangeLogPage> {
  if (!Number.isInteger(userId) || userId < 1) throw new ProtocolError('user id is invalid')
  return parsePhoneChangeLogPage(
    await request({
      method: 'GET',
      url: `/api/admin/v1/user/account/${userId}/phone-change-log`,
      params: query,
    }),
  )
}

function parseSendCodeResult(value: unknown): PhoneSendCodeResult {
  const data = expectExactKeys(
    value,
    ['challengeId', 'expiresAt', 'resendAfterSeconds'],
    'phone send code',
  )
  const challengeId = expectString(data.challengeId, 'phone send code.challengeId')
  const expiresAt = expectString(data.expiresAt, 'phone send code.expiresAt')
  const resendAfterSeconds = expectInteger(
    data.resendAfterSeconds,
    'phone send code.resendAfterSeconds',
  )
  if (!isChallengeID(challengeId)) {
    throw new ProtocolError('phone send code.challengeId is invalid')
  }
  if (!isTimestamp(expiresAt)) throw new ProtocolError('phone send code.expiresAt is invalid')
  if (resendAfterSeconds < 0 || resendAfterSeconds > 86400) {
    throw new ProtocolError('phone send code.resendAfterSeconds is invalid')
  }
  return { challengeId, expiresAt, resendAfterSeconds }
}

function parsePhoneResult(value: unknown): PhoneResult {
  const data = expectExactKeys(value, ['phone'], 'phone result')
  const phone = expectString(data.phone, 'phone result.phone')
  if (!/^\+861[3-9][0-9]{9}$/.test(phone)) {
    throw new ProtocolError('phone result.phone is invalid')
  }
  return { phone }
}

function parsePhoneChangeLogPage(value: unknown): PhoneChangeLogPage {
  const data = expectExactKeys(value, ['list', 'total', 'page', 'pageSize'], 'phone change logs')
  if (!Array.isArray(data.list)) {
    throw new ProtocolError('phone change logs response is invalid')
  }
  const total = expectInteger(data.total, 'phone change logs.total')
  const page = expectInteger(data.page, 'phone change logs.page')
  const pageSize = expectInteger(data.pageSize, 'phone change logs.pageSize')
  if (total < 0 || page < 1 || pageSize < 1 || pageSize > 100)
    throw new ProtocolError('phone change logs response is invalid')
  return {
    list: data.list.map(parsePhoneChangeLogItem),
    total,
    page,
    pageSize,
  }
}

function parsePhoneChangeLogItem(value: unknown): PhoneChangeLogItem {
  const data = expectExactKeys(
    value,
    ['id', 'action', 'oldPhone', 'newPhone', 'platform', 'createdAt'],
    'phone change log item',
  )
  if (
    typeof data.id !== 'number' ||
    !Number.isInteger(data.id) ||
    (data.action !== 1 && data.action !== 2) ||
    !isNullablePhone(data.oldPhone) ||
    typeof data.newPhone !== 'string' ||
    !isPhone(data.newPhone) ||
    typeof data.platform !== 'string' ||
    data.platform.trim() === '' ||
    typeof data.createdAt !== 'string' ||
    !isTimestamp(data.createdAt)
  ) {
    throw new ProtocolError('phone change log item is invalid')
  }
  if (
    (data.action === 1 && data.oldPhone === null) ||
    (data.action === 2 && data.oldPhone !== null)
  ) {
    throw new ProtocolError('phone change log oldPhone is invalid')
  }
  const id = expectInteger(data.id, 'phone change log item.id')
  const newPhone = expectString(data.newPhone, 'phone change log item.newPhone')
  const platform = expectString(data.platform, 'phone change log item.platform')
  const createdAt = expectString(data.createdAt, 'phone change log item.createdAt')
  if (id < 1 || !isPhone(newPhone) || platform.trim() === '' || !isTimestamp(createdAt))
    throw new ProtocolError('phone change log item is invalid')
  return {
    id,
    action: data.action,
    oldPhone: data.oldPhone,
    newPhone,
    platform,
    createdAt,
  }
}

function isNullablePhone(value: unknown): value is string | null {
  return value === null || (typeof value === 'string' && isPhone(value))
}
function isPhone(value: string): boolean {
  return /^\+861[3-9][0-9]{9}$/.test(value)
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
