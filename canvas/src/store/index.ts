import { createRequestClient } from '@/utils/request'
import { getDeviceId } from '@/auth/deviceId'
import { authPlatform } from '@/auth/platform'
import { AuthStore } from '@/store/auth'
import { PermissionStore } from '@/store/permission'

let store: AuthStore | null = null

export const request = createRequestClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? '',
  platform: authPlatform,
  deviceId: getDeviceId(),
  onUnauthorized: () => store?.markAnonymous(),
})

export const authStore = new AuthStore(request)
store = authStore
export const permissionStore = new PermissionStore(request)
let activeUserID: number | null = null
authStore.subscribe(() => {
  const userID = authStore.state.status === 'authenticated' ? authStore.state.user?.userId ?? null : null
  if (userID !== activeUserID) { activeUserID = userID; permissionStore.reset() }
})
