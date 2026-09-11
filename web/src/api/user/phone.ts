import { expectExactKeys, expectInteger, expectString } from '@/api/protocol'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

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
    await request<unknown>({
      method: 'POST',
      url: '/api/admin/v1/user/phone/send-code',
      data: input,
    }),
  )
}

export async function bindPhone(input: BindPhoneInput): Promise<PhoneResult> {
  return parsePhoneResult(
    await request<unknown>({ method: 'PUT', url: '/api/admin/v1/user/phone', data: input }),
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
