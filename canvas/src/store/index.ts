import { createRequestClient } from '@/utils/request'
import { getDeviceId } from '@/auth/deviceId'
import { authPlatform } from '@/auth/platform'
import { AuthStore } from '@/store/auth'

let store: AuthStore | null = null

export const request = createRequestClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? '',
  platform: authPlatform,
  deviceId: getDeviceId(),
  onUnauthorized: () => store?.markAnonymous(),
})

export const authStore = new AuthStore(request)
store = authStore
