import type { Router } from 'vue-router'

import { getCurrentUser, refresh } from './api/auth'
import { pinia } from './store'
import { useAuthStore } from './store/auth'
import { ApiError } from './types/http'

export function installPermissionGuard(router: Router): void {
  router.beforeEach(async (to) => {
    if (to.matched.some((route) => typeof route.meta.requiresAuth !== 'boolean')) {
      throw new Error(`Route ${to.fullPath} must declare requiresAuth`)
    }
    const auth = useAuthStore(pinia)
    if (!to.meta.requiresAuth) {
      if (auth.status === 'authenticated' && (to.name === 'login' || to.name === 'register')) {
        return { name: 'dashboard' }
      }
      return true
    }
    if (auth.status === 'authenticated') {
      return true
    }
    try {
      await refresh()
      const currentUser = await getCurrentUser()
      auth.setAuthenticated(currentUser)
      return true
    } catch (error: unknown) {
      if (isUnauthorized(error)) {
        auth.setAnonymous()
      } else {
        auth.setError(errorMessage(error))
      }
      return { name: 'login', query: { redirect: to.fullPath } }
    }
  })
}

function isUnauthorized(error: unknown): boolean {
  return error instanceof ApiError && (error.httpStatus === 401 || error.code === 10002)
}

function errorMessage(error: unknown): string {
  return error instanceof Error && error.message !== '' ? error.message : '认证服务响应异常'
}
