import type { Router } from 'vue-router'

import { getAuthPolicy, getCurrentUser, refresh } from './api/auth'
import { YesNo } from './enums/yes-no'
import { appI18n } from './i18n'
import { registerAccessRoutes } from './router/access-routes'
import { pinia } from './store'
import { useAccessStore } from './store/access'
import { useAuthStore } from './store/auth'
import { ApiError, ProtocolError } from './types/http'

export function installPermissionGuard(router: Router): void {
  let removeAccessRoutes: (() => void) | null = null

  function clearAccess(): void {
    if (removeAccessRoutes !== null) {
      removeAccessRoutes()
      removeAccessRoutes = null
    }
    useAccessStore(pinia).reset()
  }

  router.beforeEach(async (to) => {
    if (to.matched.some((route) => typeof route.meta.requiresAuth !== 'boolean')) {
      throw new Error(`Route ${to.fullPath} must declare requiresAuth`)
    }
    const auth = useAuthStore(pinia)
    const access = useAccessStore(pinia)
    const isPublic = to.matched.length > 0 && to.matched.every((route) => route.meta.requiresAuth === false)
    if (isPublic) {
      if (auth.status === 'anonymous') {
        clearAccess()
      }
      if (auth.status === 'authenticated' && to.name === 'login') {
        return { name: 'dashboard' }
      }
      if (to.name === 'register') {
        try {
          const policy = await getAuthPolicy()
          if (policy.allowRegister === YesNo.No) {
            return { name: 'login' }
          }
        } catch (error: unknown) {
          auth.setError(errorMessage(error))
          clearAccess()
          return { name: 'login' }
        }
      }
      return true
    }

    if (auth.status === 'anonymous') {
      clearAccess()
      return { name: 'login', query: { redirect: to.fullPath } }
    }

    if (auth.status !== 'authenticated') {
      try {
        await refresh()
        const currentUser = await getCurrentUser()
        auth.setAuthenticated(currentUser)
      } catch (error: unknown) {
        if (isUnauthorized(error)) {
          auth.setAnonymous()
        } else {
          auth.setError(errorMessage(error))
        }
        clearAccess()
        return { name: 'login', query: { redirect: to.fullPath } }
      }
    }

    let accessFailed = access.status === 'error'
    if (!accessFailed && access.status !== 'ready') {
      try {
        await access.load()
      } catch {
        accessFailed = true
      }
    }

    if (!accessFailed && access.status === 'ready' && removeAccessRoutes === null) {
      try {
        removeAccessRoutes = registerAccessRoutes(router, access.menuTree)
      } catch (error: unknown) {
        access.fail(error)
        accessFailed = true
      }
    }

    if (accessFailed || access.status === 'error') {
      return to.name === 'dashboard' ? true : { name: 'dashboard' }
    }

		const protectedRecord = [...to.matched]
			.reverse()
			.find((record) => record.meta.requiredPermission !== undefined)
		const requiredPermission = protectedRecord?.meta.requiredPermission
		if (requiredPermission !== undefined && !access.hasPermission(requiredPermission)) {
			return { name: 'dashboard' }
		}

    if (to.matched.length === 0) {
      const resolved = router.resolve(to.fullPath)
      return resolved.matched.length === 0
        ? { name: 'dashboard' }
        : { path: to.fullPath, replace: true }
    }

    return true
  })
}

function isUnauthorized(error: unknown): boolean {
  return error instanceof ApiError && (error.httpStatus === 401 || error.code === 10002)
}

function errorMessage(error: unknown): string {
  if (error instanceof ProtocolError) {
    return appI18n.global.t('request.protocolError')
  }
  return error instanceof Error && error.message !== ''
    ? error.message
    : appI18n.global.t('auth.login.bootstrapFailed')
}
