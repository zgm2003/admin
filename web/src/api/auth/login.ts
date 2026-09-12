import { refreshAccessCredential, request } from '@/utils/request'
import {
  expectArray,
  expectBoolean,
  expectExactKeys,
  expectEmptyObject,
  expectInteger,
  expectNullableString,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'

export type LoginType = 'email' | 'phone' | 'password'

export type LoginInput =
  | { loginType: 'password'; loginAccount: string; password: string; challengeId?: never; code?: never }
  | {
      loginType: 'email' | 'phone'
      loginAccount: string
      challengeId: string
      code: string
      password?: never
    }

export interface AccessCredential {
  accessToken: string
  expiresIn: number
  isNewUser: boolean
  passwordSetRequired: boolean
}

export interface LoginConfigOption {
  value: LoginType
  label: string
}

export interface LoginConfig {
  loginTypes: LoginConfigOption[]
  allowRegister: boolean
}

export interface SendCodeResult {
  challengeId: string
  expiresAt: string
  resendAfterSeconds: number
}

export interface CaptchaChallenge {
  captchaId: string
  captchaType: 'slide'
  masterImage: string
  tileImage: string
  tileX: number
  tileY: number
  tileWidth: number
  tileHeight: number
  imageWidth: number
  imageHeight: number
  expiresIn: number
}

export interface CaptchaAnswer { x: number; y: number }

export async function getCaptcha(): Promise<CaptchaChallenge> {
  return parseCaptchaChallenge(await request<unknown>({ method: 'GET', url: '/api/v1/auth/captcha' }))
}

export interface CurrentUser {
  userId: number
  username: string
  email: string
  phone: string | null
  avatar: string
  passwordSetRequired: boolean
}

export async function login(input: LoginInput): Promise<AccessCredential> {
  return parseAccessCredential(
    await request<unknown>({ method: 'POST', url: '/api/v1/auth/login', data: input }),
  )
}

export async function getLoginConfig(): Promise<LoginConfig> {
  return parseLoginConfig(
    await request<unknown>({ method: 'GET', url: '/api/v1/auth/login-config' }),
  )
}

export async function sendLoginCode(
  account: string,
  loginType: 'email' | 'phone',
  scene: 'login' = 'login',
  challengeId?: string,
  captcha?: { captchaId: string; captchaAnswer: CaptchaAnswer },
): Promise<SendCodeResult> {
  return parseSendCodeResult(
    await request<unknown>({
      method: 'POST',
      url: '/api/v1/auth/send-code',
      data: { account, loginType, scene, challengeId, captchaId: captcha?.captchaId, captchaAnswer: captcha?.captchaAnswer },
    }),
  )
}

export interface ResetPasswordInput {
  account: string
  loginType: 'email' | 'phone'
  challengeId: string
  code: string
  newPassword: string
  confirmPassword: string
}

export async function forgotPassword(account: string, loginType: 'email' | 'phone', captcha?: { captchaId: string; captchaAnswer: CaptchaAnswer }): Promise<SendCodeResult> {
  return parseSendCodeResult(
    await request<unknown>({
      method: 'POST',
      url: '/api/v1/auth/password/forgot',
      data: { account, loginType, captchaId: captcha?.captchaId, captchaAnswer: captcha?.captchaAnswer },
    }),
  )
}

export async function resetPassword(input: ResetPasswordInput): Promise<void> {
  expectEmptyObject(
    await request<unknown>({
      method: 'POST',
      url: '/api/v1/auth/password/reset',
      data: input,
    }),
    'reset password result',
  )
}

export async function refresh(): Promise<AccessCredential> {
  return refreshAccessCredential()
}

export async function logout(): Promise<void> {
  expectEmptyObject(
    await request<unknown>({ method: 'POST', url: '/api/v1/auth/logout' }),
    'logout result',
  )
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return parseCurrentUser(await request<unknown>({ method: 'GET', url: '/api/v1/auth/me' }))
}

function parseCurrentUser(value: unknown): CurrentUser {
  const record = expectExactKeys(
    value,
    ['userId', 'username', 'email', 'phone', 'avatar', 'passwordSetRequired'],
    'current user response',
  )
  return {
    userId: expectInteger(record.userId, 'current user.userId'),
    username: expectString(record.username, 'current user.username'),
    email: expectString(record.email, 'current user.email'),
    phone: expectNullableString(record.phone, 'current user.phone'),
    avatar: expectString(record.avatar, 'current user.avatar'),
    passwordSetRequired: expectBoolean(
      record.passwordSetRequired,
      'current user.passwordSetRequired',
    ),
  }
}

function parseAccessCredential(value: unknown): AccessCredential {
  const record = expectExactKeys(
    value,
    ['accessToken', 'expiresIn', 'isNewUser', 'passwordSetRequired'],
    'access credential response',
  )
  return {
    accessToken: expectString(record.accessToken, 'access credential.accessToken'),
    expiresIn: expectInteger(record.expiresIn, 'access credential.expiresIn'),
    isNewUser: expectBoolean(record.isNewUser, 'access credential.isNewUser'),
    passwordSetRequired: expectBoolean(
      record.passwordSetRequired,
      'access credential.passwordSetRequired',
    ),
  }
}

const loginTypeSet = new Set<string>(['email', 'phone', 'password'])
const effectiveLoginTypeSet = new Set<string>(['email', 'phone', 'password'])

function parseLoginConfigOption(value: unknown, index: number): LoginConfigOption {
  const record = expectExactKeys(value, ['value', 'label'], `login config.options[${index}]`)
  const loginType = expectString(record.value, `login config.options[${index}].value`)
  if (!loginTypeSet.has(loginType)) throw new ProtocolError('login config option value is invalid')
  if (!effectiveLoginTypeSet.has(loginType)) {
    throw new ProtocolError('login config option value is unavailable')
  }
  return {
    value: loginType as LoginType,
    label: expectString(record.label, `login config.options[${index}].label`),
  }
}

function parseLoginConfig(value: unknown): LoginConfig {
  const record = expectExactKeys(value, ['loginTypes', 'allowRegister'], 'login config response')
  const rawLoginTypes = expectArray(record.loginTypes, 'login config.loginTypes')
  if (rawLoginTypes.length === 0) {
    throw new ProtocolError('login config.loginTypes must not be empty')
  }
  const seen = new Set<LoginType>()
  const loginTypes = rawLoginTypes.map((item, index) => {
    const option = parseLoginConfigOption(item, index)
    if (seen.has(option.value)) {
      throw new ProtocolError('login config.loginTypes contains duplicates')
    }
    seen.add(option.value)
    return option
  })
  return {
    loginTypes,
    allowRegister: expectBoolean(record.allowRegister, 'login config.allowRegister'),
  }
}

function parseSendCodeResult(value: unknown): SendCodeResult {
  const record = expectExactKeys(
    value,
    ['challengeId', 'expiresAt', 'resendAfterSeconds'],
    'send code response',
  )
  const expiresAt = expectString(record.expiresAt, 'send code.expiresAt')
  if (expiresAt.trim() === '' || Number.isNaN(Date.parse(expiresAt))) {
    throw new ProtocolError('send code.expiresAt must be a timestamp')
  }
  const resendAfterSeconds = expectInteger(
    record.resendAfterSeconds,
    'send code.resendAfterSeconds',
  )
  if (resendAfterSeconds < 0 || resendAfterSeconds > 86400) {
    throw new ProtocolError('send code.resendAfterSeconds must be between 0 and 86400')
  }
  return {
    challengeId: expectString(record.challengeId, 'send code.challengeId'),
    expiresAt,
    resendAfterSeconds,
  }
}

function parseCaptchaChallenge(value: unknown): CaptchaChallenge {
  const record = expectExactKeys(value, ['captchaId', 'captchaType', 'masterImage', 'tileImage', 'tileX', 'tileY', 'tileWidth', 'tileHeight', 'imageWidth', 'imageHeight', 'expiresIn'], 'captcha challenge')
  if (record.captchaType !== 'slide') throw new ProtocolError('captcha challenge type is invalid')
  return {
    captchaId: expectString(record.captchaId, 'captcha challenge.captchaId'),
    captchaType: 'slide',
    masterImage: expectString(record.masterImage, 'captcha challenge.masterImage'),
    tileImage: expectString(record.tileImage, 'captcha challenge.tileImage'),
    tileX: expectInteger(record.tileX, 'captcha challenge.tileX'),
    tileY: expectInteger(record.tileY, 'captcha challenge.tileY'),
    tileWidth: expectInteger(record.tileWidth, 'captcha challenge.tileWidth'),
    tileHeight: expectInteger(record.tileHeight, 'captcha challenge.tileHeight'),
    imageWidth: expectInteger(record.imageWidth, 'captcha challenge.imageWidth'),
    imageHeight: expectInteger(record.imageHeight, 'captcha challenge.imageHeight'),
    expiresIn: expectInteger(record.expiresIn, 'captcha challenge.expiresIn'),
  }
}
