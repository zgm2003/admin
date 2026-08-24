import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getAccess } from '@src/api/access'
import type { AccessSnapshot } from '@src/api/access.contract'
import { getAuthPolicy, getCurrentUser, refresh } from '@src/api/auth'
import { installPermissionGuard } from '@src/permission'
import { pinia } from '@src/store'
import { useAccessStore } from '@src/store/access'
import { useAuthStore } from '@src/store/auth'
import { ApiError } from '@src/types/http'
import { createAppRouter } from '@src/router/index'

vi.mock('@src/api/auth', () => ({ getAuthPolicy: vi.fn(), refresh: vi.fn(), getCurrentUser: vi.fn() }))
vi.mock('@src/api/access', () => ({ getAccess: vi.fn() }))
vi.mock('@src/access/route-views', () => ({
  routeViews: {
    systemUsers: async () => ({ template: '<div>users</div>' }),
    systemTeams: async () => ({ template: '<div>teams</div>' }),
  },
}))

const refreshMock = vi.mocked(refresh)
const getCurrentUserMock = vi.mocked(getCurrentUser)
const getAuthPolicyMock = vi.mocked(getAuthPolicy)
const getAccessMock = vi.mocked(getAccess)

describe('router', () => {
  beforeEach(() => {
    useAuthStore(pinia).$reset()
    useAccessStore(pinia).reset()
    refreshMock.mockReset()
    getCurrentUserMock.mockReset()
    getAuthPolicyMock.mockReset()
    getAuthPolicyMock.mockResolvedValue({ code: 'admin', name: 'Admin', allowRegister: 1 })
    getAccessMock.mockReset()
    getAccessMock.mockResolvedValue(emptyAccessSnapshot())
  })

  it('declares Login and Register as public, names the layout, and protects Dashboard', () => {
    const router = createAppRouter(createMemoryHistory())
    expect(router.resolve('/login').meta.requiresAuth).toBe(false)
    expect(router.resolve('/register').meta.requiresAuth).toBe(false)
    expect(router.resolve('/dashboard').meta.requiresAuth).toBe(true)
    expect(router.hasRoute('admin-layout')).toBe(true)
  })

  it('declares a translated fixed title for Dashboard', () => {
    const router = createAppRouter(createMemoryHistory())
    const dashboard = router.resolve('/dashboard')
    expect(dashboard.meta.requiresAuth).toBe(true)
    expect(dashboard.meta.titleKey).toBe('navigation.dashboard')
    expect(dashboard.meta.affix).toBe(true)
    expect(router.resolve('/login').meta.titleKey).toBeUndefined()
  })

  it('restores a cold dynamic URL through auth, access, route registration, and the original URL', async () => {
    const order: string[] = []
    refreshMock.mockImplementation(async () => {
      order.push('refresh')
      return { accessToken: 'jwt', expiresIn: 900 }
    })
    getCurrentUserMock.mockImplementation(async () => {
      order.push('me')
      return { userId: 1, username: 'admin', email: 'admin@example.com' }
    })
    getAccessMock.mockImplementation(async () => {
      order.push('access')
      return businessAccessSnapshot()
    })
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/system/users')

    expect(order).toEqual(['refresh', 'me', 'access'])
    expect(router.hasRoute('access:system:user:view')).toBe(true)
    expect(router.currentRoute.value.fullPath).toBe('/system/users')
    expect(useAuthStore(pinia).status).toBe('authenticated')
    expect(useAccessStore(pinia).status).toBe('ready')
  })

  it('loads access once for an already authenticated Dashboard', async () => {
    setAuthenticated()
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/dashboard')
    await router.push('/login')

    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(getAccessMock).toHaveBeenCalledOnce()
    expect(refreshMock).not.toHaveBeenCalled()
  })

  it('does not reload access while clicking between installed pages', async () => {
    setAuthenticated()
    getAccessMock.mockResolvedValue(businessAccessSnapshot())
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/system/users')
    await router.push('/system/teams')
    await router.push('/system/users')

    expect(router.currentRoute.value.path).toBe('/system/users')
    expect(getAccessMock).toHaveBeenCalledOnce()
  })

  it('shares one access request across concurrent protected navigations', async () => {
    setAuthenticated()
    const request = deferred<AccessSnapshot>()
    getAccessMock.mockReturnValue(request.promise)
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    const dashboardNavigation = router.push('/dashboard')
    const pageNavigation = router.push('/system/users')
    await vi.waitFor(() => expect(getAccessMock).toHaveBeenCalledOnce())
    request.resolve(businessAccessSnapshot())
    await Promise.all([dashboardNavigation, pageNavigation])

    expect(getAccessMock).toHaveBeenCalledOnce()
    expect(router.hasRoute('access:system:user:view')).toBe(true)
  })

  it('keeps Dashboard mounted when access loading fails', async () => {
    setAuthenticated()
    getAccessMock.mockRejectedValue(new Error('access unavailable'))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/dashboard')

    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(useAccessStore(pinia).status).toBe('error')
    expect(useAccessStore(pinia).errorMessage).not.toBe('')
    expect(getAccessMock).toHaveBeenCalledOnce()
  })

  it('redirects a cold dynamic URL to Dashboard when access loading fails', async () => {
    setAuthenticated()
    getAccessMock.mockRejectedValue(new Error('access unavailable'))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/system/users')

    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(useAccessStore(pinia).status).toBe('error')
    expect(getAccessMock).toHaveBeenCalledOnce()
  })

  it('removes dynamic routes and access state after authentication becomes anonymous', async () => {
    setAuthenticated()
    getAccessMock.mockResolvedValue(businessAccessSnapshot())
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)
    await router.push('/system/users')
    expect(router.hasRoute('access:system:user:view')).toBe(true)

    useAuthStore(pinia).setAnonymous()
    await router.push('/login')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.hasRoute('access:system:user:view')).toBe(false)
    expect(useAccessStore(pinia).status).toBe('idle')
    expect(useAccessStore(pinia).menuTree).toEqual([])
  })

  it('does not bootstrap access for public Login', async () => {
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)
    await router.push('/login')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(refreshMock).not.toHaveBeenCalled()
    expect(getAccessMock).not.toHaveBeenCalled()
  })

  it('redirects public Register to Login when registration is disabled', async () => {
    getAuthPolicyMock.mockResolvedValue({ code: 'admin', name: 'Admin', allowRegister: 0 })
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/register')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(getAuthPolicyMock).toHaveBeenCalledOnce()
  })

  it('allows public Register when registration is enabled', async () => {
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/register')

    expect(router.currentRoute.value.path).toBe('/register')
    expect(getAuthPolicyMock).toHaveBeenCalledOnce()
  })

  it('keeps a registration policy failure as an explicit auth error', async () => {
    getAuthPolicyMock.mockRejectedValue(new ApiError(10006, '服务暂未就绪', 503))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/register')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(useAuthStore(pinia).status).toBe('error')
    expect(useAuthStore(pinia).errorMessage).toBe('服务暂未就绪')
  })

  it('redirects a refresh 401 to Login as anonymous', async () => {
    refreshMock.mockRejectedValue(new ApiError(10002, '未登录或登录已失效', 401))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/dashboard')

    expect(router.currentRoute.value.fullPath).toBe('/login?redirect=/dashboard')
    expect(useAuthStore(pinia).status).toBe('anonymous')
    expect(getCurrentUserMock).not.toHaveBeenCalled()
    expect(getAccessMock).not.toHaveBeenCalled()
  })

  it('keeps authentication dependency failures distinct from anonymous', async () => {
    refreshMock.mockRejectedValue(new ApiError(10006, '服务暂未就绪', 503))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)
    await router.push('/dashboard')

    const store = useAuthStore(pinia)
    expect(store.status).toBe('error')
    expect(store.errorMessage).toBe('服务暂未就绪')
    expect(router.currentRoute.value.path).toBe('/login')
    expect(getAccessMock).not.toHaveBeenCalled()
  })

  it('rejects a matched route record without an explicit requiresAuth declaration', async () => {
    const childWithoutMeta = {
      path: 'missing-meta',
      component: { template: '<div />' },
    } as RouteRecordRaw
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/',
          component: { template: '<router-view />' },
          meta: { requiresAuth: false },
          children: [childWithoutMeta],
        },
      ],
    })
    installPermissionGuard(router)
    router.onError(() => undefined)

    await expect(router.push('/missing-meta')).rejects.toThrow('Route /missing-meta must declare requiresAuth')
  })
})

function setAuthenticated(): void {
  const store = useAuthStore(pinia)
  store.setCredential({ accessToken: 'jwt', expiresIn: 900 })
  store.setAuthenticated({ userId: 1, username: 'admin', email: 'admin@example.com' })
}

function emptyAccessSnapshot(): AccessSnapshot {
  return { roleCodes: [], menuTree: [], permissionCodes: [] }
}

function businessAccessSnapshot(): AccessSnapshot {
  return {
    roleCodes: ['registered_user'],
    menuTree: [{
      code: 'system',
      menuType: 'directory',
      path: null,
      viewKey: null,
		titleKey: 'navigation.system',
      icon: null,
      children: [
        {
          code: 'system:team:view',
          menuType: 'page',
          path: '/system/teams',
          viewKey: 'systemTeams',
			titleKey: 'navigation.systemMenus',
          icon: null,
          children: [],
        },
        {
          code: 'system:user:view',
          menuType: 'page',
          path: '/system/users',
          viewKey: 'systemUsers',
			titleKey: 'navigation.systemMenus',
          icon: null,
          children: [],
        },
      ],
    }],
    permissionCodes: ['system:team:view', 'system:user:view'],
  }
}

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolvePromise: ((value: T) => void) | undefined
  const promise = new Promise<T>((resolve) => {
    resolvePromise = resolve
  })
  return {
    promise,
    resolve: (value: T) => {
      if (resolvePromise === undefined) throw new Error('deferred promise was not initialized')
      resolvePromise(value)
    },
  }
}
