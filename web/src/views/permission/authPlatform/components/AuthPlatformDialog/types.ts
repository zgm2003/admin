export interface AuthPlatformForm {
  code: string
  name: string
  loginTypes: import('@/api/permission/authPlatform').LoginType[]
  accessTTLSeconds: number
  refreshTTLSeconds: number
  sessionCacheTTLSeconds: number
  accessCacheTTLSeconds: number
  bindDevice: import('@/enums/yesNo').YesNo
  bindIP: import('@/enums/yesNo').YesNo
  maxSessions: number
  allowRegister: import('@/enums/yesNo').YesNo
  isEnabled: import('@/enums/yesNo').YesNo
}
