import type { RequestClient } from '@/utils/request'

export type LoginType = 'email' | 'phone' | 'password'

export type LoginInput =
  | { loginType: 'password'; loginAccount: string; password: string }
  | { loginType: 'email' | 'phone'; loginAccount: string; challengeId: string; code: string }

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

export interface SendCodeResult {
  challengeId: string
  expiresAt: string
  resendAfterSeconds: number
}

export interface CurrentUser {
  userId: number
  username: string
  email: string
  phone: string | null
  avatar: string
  passwordSetRequired: boolean
}

export interface PublicPolicy {
  code: string
  name: string
  allowRegister: 0 | 1
}

export interface CaptchaAnswer {
  captchaId: string
  captchaAnswer: { x: number; y: number }
}

export function createAuthApi(client: RequestClient) {
  return {
    getLoginConfig: () => client.get<LoginConfig>('/api/canvas/v1/auth/login-config'),
    getPolicy: () => client.get<PublicPolicy>('/api/canvas/v1/auth/policy'),
    getCaptcha: () => client.get<CaptchaChallenge>('/api/canvas/v1/auth/captcha'),
    sendLoginCode: (account: string, loginType: 'email' | 'phone', captcha?: CaptchaAnswer) =>
      client.post<SendCodeResult>('/api/canvas/v1/auth/send-code', {
        account,
        loginType,
        scene: 'login',
        captchaId: captcha?.captchaId,
        captchaAnswer: captcha?.captchaAnswer,
      }),
    login: (input: LoginInput) => client.post<AccessCredential>('/api/canvas/v1/auth/login', input),
    refresh: () => client.post<AccessCredential>('/api/canvas/v1/auth/refresh'),
    getCurrentUser: () => client.get<CurrentUser>('/api/canvas/v1/auth/me'),
    logout: () => client.post<void>('/api/canvas/v1/auth/logout'),
  }
}

export type AuthApi = ReturnType<typeof createAuthApi>
