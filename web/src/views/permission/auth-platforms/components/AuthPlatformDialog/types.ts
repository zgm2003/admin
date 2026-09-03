export interface AuthPlatformForm {
  code: string
  name: string
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: import('@/enums/yes-no').YesNo
  bindIP: import('@/enums/yes-no').YesNo
  maxSessions: number
  allowRegister: import('@/enums/yes-no').YesNo
  isEnabled: import('@/enums/yes-no').YesNo
}
