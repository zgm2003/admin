import type {
  AuthPlatformListItem,
  CreateAuthPlatformInput,
  UpdateAuthPlatformInput,
} from '@/api/permission/authPlatform'
import type { AuthPlatformOptions } from '@/api/permission/authPlatformOptions'
import type { AuthPlatformForm } from './components/AuthPlatformDialog/types'

export function createAuthPlatformForm(
  defaults: AuthPlatformOptions['defaults'],
): AuthPlatformForm {
  return { code: '', name: '', ...defaults, loginTypes: [...defaults.loginTypes] }
}

export function editAuthPlatformForm(platform: AuthPlatformListItem): AuthPlatformForm {
  return {
    code: platform.code,
    name: platform.name,
    loginTypes: [...platform.loginTypes],
    accessTTLSeconds: platform.accessTTLSeconds,
    refreshTTLSeconds: platform.refreshTTLSeconds,
    sessionCacheTTLSeconds: platform.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: platform.accessCacheTTLSeconds,
    bindDevice: platform.bindDevice,
    bindIP: platform.bindIP,
    maxSessions: platform.maxSessions,
    allowRegister: platform.allowRegister,
    isEnabled: platform.isEnabled,
  }
}

function inRange(value: number, minimum: number, maximum: number): boolean {
  return Number.isInteger(value) && value >= minimum && value <= maximum
}

export function isAuthPlatformFormValid(
  form: AuthPlatformForm,
  isEditing: boolean,
  options: AuthPlatformOptions | null,
): boolean {
  if (options === null) return false
  return (
    (isEditing || new RegExp(options.codePattern).test(form.code.trim())) &&
    form.name.trim() !== '' &&
    new TextEncoder().encode(form.name.trim()).length <= options.nameMaxBytes &&
    form.loginTypes.length > 0 &&
    Object.entries(options.limits).every(([key, range]) =>
      inRange(form[key as keyof typeof options.limits], range.minimum, range.maximum),
    )
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
    loginTypes: form.loginTypes,
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

export function updateAuthPlatformInput(form: AuthPlatformForm): UpdateAuthPlatformInput {
  return {
    name: form.name.trim(),
    loginTypes: form.loginTypes,
    accessTTLSeconds: form.accessTTLSeconds,
    refreshTTLSeconds: form.refreshTTLSeconds,
    sessionCacheTTLSeconds: form.sessionCacheTTLSeconds,
    accessCacheTTLSeconds: form.accessCacheTTLSeconds,
    bindDevice: form.bindDevice,
    bindIP: form.bindIP,
    maxSessions: form.maxSessions,
    allowRegister: form.allowRegister,
  }
}
