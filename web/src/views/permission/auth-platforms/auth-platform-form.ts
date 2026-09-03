import type {
  AuthPlatformListItem,
  CreateAuthPlatformInput,
  UpdateAuthPlatformInput,
} from '@/api/auth/platform'
import { YesNo } from '@/enums/yes-no'
import type { AuthPlatformForm } from './components/AuthPlatformDialog/types'

export const authPlatformDefaultTTL = Object.freeze({
  accessTTLSeconds: 900,
  refreshTTLSeconds: 86_400,
  sessionCacheTTLSeconds: 7_200,
  accessCacheTTLSeconds: 600,
})

export function createAuthPlatformForm(): AuthPlatformForm {
  return {
    code: '',
    name: '',
    ...authPlatformDefaultTTL,
    bindDevice: YesNo.Yes,
    bindIP: YesNo.No,
    maxSessions: 1,
    allowRegister: YesNo.No,
    isEnabled: YesNo.Yes,
  }
}

export function editAuthPlatformForm(platform: AuthPlatformListItem): AuthPlatformForm {
  const isBuiltinAdmin = platform.code === 'admin' && platform.isBuiltin === YesNo.Yes
  return {
    code: platform.code,
    name: platform.name,
    accessTTLSeconds: platform.accessTTLSeconds,
    refreshTTLSeconds: platform.refreshTTLSeconds,
    sessionCacheTTLSeconds: platform.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: platform.accessCacheTTLSeconds,
    bindDevice: platform.bindDevice,
    bindIP: platform.bindIP,
    maxSessions: platform.maxSessions,
    allowRegister: isBuiltinAdmin ? YesNo.No : platform.allowRegister,
    isEnabled: platform.isEnabled,
  }
}

function inRange(value: number, minimum: number, maximum: number): boolean {
  return Number.isInteger(value) && value >= minimum && value <= maximum
}

export function isAuthPlatformFormValid(form: AuthPlatformForm, isEditing: boolean): boolean {
  const codeValid = isEditing || /^[a-z][a-z0-9_]{1,48}$/.test(form.code.trim())
  return (
    codeValid &&
    form.name.trim() !== '' &&
    form.name.trim().length <= 64 &&
    inRange(form.accessTTLSeconds, 60, 2_592_000) &&
    inRange(form.refreshTTLSeconds, 60, 31_536_000) &&
    inRange(form.sessionCacheTTLSeconds, 60, 86_400) &&
    inRange(form.accessCacheTTLSeconds, 60, 86_400) &&
    inRange(form.maxSessions, 0, 100)
  )
}

export function authPlatformSecurityChanged(
  form: AuthPlatformForm,
  platform: AuthPlatformListItem,
): boolean {
  return (
    form.bindDevice !== platform.bindDevice ||
    form.bindIP !== platform.bindIP ||
    form.accessTTLSeconds !== platform.accessTTLSeconds ||
    form.refreshTTLSeconds !== platform.refreshTTLSeconds
  )
}

export function createAuthPlatformInput(form: AuthPlatformForm): CreateAuthPlatformInput {
  return {
    code: form.code.trim(),
    name: form.name.trim(),
    accessTTLSeconds: form.accessTTLSeconds,
    refreshTTLSeconds: form.refreshTTLSeconds,
    sessionCacheTTLSeconds: form.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: form.accessCacheTTLSeconds,
    bindDevice: form.bindDevice,
    bindIP: form.bindIP,
    maxSessions: form.maxSessions,
    allowRegister: form.allowRegister,
    isEnabled: form.isEnabled,
  }
}

export function updateAuthPlatformInput(
  form: AuthPlatformForm,
  isBuiltinAdmin: boolean,
): UpdateAuthPlatformInput {
  return {
    name: form.name.trim(),
    accessTTLSeconds: form.accessTTLSeconds,
    refreshTTLSeconds: form.refreshTTLSeconds,
    sessionCacheTTLSeconds: form.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: form.accessCacheTTLSeconds,
    bindDevice: form.bindDevice,
    bindIP: form.bindIP,
    maxSessions: form.maxSessions,
    allowRegister: isBuiltinAdmin ? YesNo.No : form.allowRegister,
  }
}
