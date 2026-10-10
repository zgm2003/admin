import { refreshAccessCredential, request } from '@/utils/request'

export type LoginType = 'email' | 'phone' | 'password'

export type LoginInput =
  | {
      loginType: 'password'
      loginAccount: string
      password: string
      challengeId?: never
      code?: never
    }
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

export interface CaptchaAnswer {
  x: number
  y: number
}

export async function getCaptcha(): Promise<CaptchaChallenge> {
  return request.get<CaptchaChallenge>('/api/v1/auth/captcha')
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
  return request.post<AccessCredential>('/api/v1/auth/login', input)
}

export async function getLoginConfig(): Promise<LoginConfig> {
  return request.get<LoginConfig>('/api/v1/auth/login-config')
}

export async function sendLoginCode(
  account: string,
  loginType: 'email' | 'phone',
  scene: 'login' = 'login',
  challengeId?: string,
  captcha?: { captchaId: string; captchaAnswer: CaptchaAnswer },
): Promise<SendCodeResult> {
  return request.post<SendCodeResult>('/api/v1/auth/send-code', {
    account,
    loginType,
    scene,
    challengeId,
    captchaId: captcha?.captchaId,
    captchaAnswer: captcha?.captchaAnswer,
  })
}

export interface ResetPasswordInput {
  account: string
  loginType: 'email' | 'phone'
  challengeId: string
  code: string
  newPassword: string
  confirmPassword: string
}

export async function forgotPassword(
  account: string,
  loginType: 'email' | 'phone',
  captcha?: { captchaId: string; captchaAnswer: CaptchaAnswer },
): Promise<SendCodeResult> {
  return request.post<SendCodeResult>('/api/v1/auth/password/forgot', {
    account,
    loginType,
    captchaId: captcha?.captchaId,
    captchaAnswer: captcha?.captchaAnswer,
  })
}

export async function resetPassword(input: ResetPasswordInput): Promise<void> {
  return request.post<void>('/api/v1/auth/password/reset', input)
}

export async function refresh(): Promise<AccessCredential> {
  return refreshAccessCredential()
}

export async function logout(): Promise<void> {
  return request.post<void>('/api/v1/auth/logout', undefined)
}

export async function getCurrentUser(): Promise<CurrentUser> {
  return request.get<CurrentUser>('/api/v1/auth/me')
}
