import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getPermission } from '@src/api/permission/permission'
import type { PermissionSnapshot } from '@src/api/permission/permission'
import { getCurrentUser, refresh } from '@src/api/auth/login'
import { installPermissionGuard } from '@src/permission'
import { pinia } from '@src/store'
import { usePermissionStore } from '@/store/permission.ts'
import { useAuthStore } from '@src/store/auth'
import { ApiError } from '@src/types/http'
import { createAppRouter } from '@src/router/index'
import { YesNo } from '@src/enums/yes-no'

vi.mock('@src/api/auth/login', () => ({ refresh: vi.fn(), getCurrentUser: vi.fn() }))
vi.mock('@src/api/permission/permission', () => ({ getPermission: vi.fn() }))
const refreshMock = vi.mocked(refresh)
const getCurrentUserMock = vi.mocked(getCurrentUser)
const getPermissionMock = vi.mocked(getPermission)

describe('router', () => {
  beforeEach(() => {
    useAuthStore(pinia).$reset()
    usePermissionStore(pinia).reset()
    refreshMock.mockReset()
    getCurrentUserMock.mockReset()
    getPermissionMock.mockReset()
    getPermissionMock.mockResolvedValue(emptyPermissionSnapshot())
  })

	it('declares public auth routes and static protected Dashboard and menu management routes', () => {
    const router = createAppRouter(createMemoryHistory())
    expect(router.resolve('/login').meta.requiresAuth).toBe(false)
	  expect(router.hasRoute('register')).toBe(false)
	  expect(router.resolve('/register').matched).toHaveLength(0)
		expect(router.resolve('/dashboard').meta.requiresAuth).toBe(true)
		expect(router.resolve('/access/menus').meta.requiresAuth).toBe(true)
		expect(router.resolve('/access/menus').meta.requiredPermission).toBe('permission:menu:list')
		expect(router.resolve('/access/menus').name).toBe('access-menus')
    expect(router.hasRoute('account-profile')).toBe(false)
    expect(router.resolve('/account/profile').matched).toHaveLength(0)
    expect(router.hasRoute('admin-layout')).toBe(true)
  })

	it('declares translated i18n keys for fixed pages', () => {
    const router = createAppRouter(createMemoryHistory())
    const dashboard = router.resolve('/dashboard')
    expect(dashboard.meta.requiresAuth).toBe(true)
		expect(dashboard.meta.i18nKey).toBe('navigation.dashboard')
    expect(dashboard.meta.affix).toBe(true)
		expect(router.resolve('/login').meta.i18nKey).toBeUndefined()
		expect(router.resolve('/access/menus').meta.i18nKey).toBe('navigation.accessMenus')
	})

	it('guards the static menu page with its exact permission after loading access', async () => {
		setAuthenticated()
		const router = createAppRouter(createMemoryHistory())
		installPermissionGuard(router)

		await router.push('/access/menus')
		expect(router.currentRoute.value.path).toBe('/dashboard')

		usePermissionStore(pinia).reset()
		getPermissionMock.mockResolvedValue({ ...emptyPermissionSnapshot(), permissionCodes: ['permission:menu:list'] })
		await router.push('/access/menus')
		expect(router.currentRoute.value.path).toBe('/access/menus')
	})

  it('restores a cold dynamic URL through auth, access, route registration, and the original URL', async () => {
    const order: string[] = []
    refreshMock.mockImplementation(async () => {
      order.push('refresh')
      return { accessToken: 'jwt', expiresIn: 900 }
    })
    getCurrentUserMock.mockImplementation(async () => {
      order.push('me')
      return { userId: 1, username: 'admin', email: 'admin@example.com', phone: null, avatar: '' }
    })
    getPermissionMock.mockImplementation(async () => {
      order.push('access')
      return businessPermissionSnapshot()
    })
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/account/users')

    expect(order).toEqual(['refresh', 'me', 'access'])
    expect(router.hasRoute('access:account:user:list')).toBe(true)
    expect(router.currentRoute.value.fullPath).toBe('/account/users')
    expect(useAuthStore(pinia).status).toBe('authenticated')
    expect(usePermissionStore(pinia).status).toBe('ready')
  })

  it('loads access once for an already authenticated Dashboard', async () => {
    setAuthenticated()
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/dashboard')
    await router.push('/login')

    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(getPermissionMock).toHaveBeenCalledOnce()
    expect(refreshMock).not.toHaveBeenCalled()
  })

  it('does not reload access while clicking between installed pages', async () => {
    setAuthenticated()
    getPermissionMock.mockResolvedValue(businessPermissionSnapshot())
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/account/users')
    await router.push('/access/roles')
    await router.push('/account/users')

    expect(router.currentRoute.value.path).toBe('/account/users')
    expect(getPermissionMock).toHaveBeenCalledOnce()
  })

  it('shares one access request across concurrent protected navigations', async () => {
    setAuthenticated()
    const request = deferred<PermissionSnapshot>()
    getPermissionMock.mockReturnValue(request.promise)
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    const dashboardNavigation = router.push('/dashboard')
    const pageNavigation = router.push('/account/users')
    await vi.waitFor(() => expect(getPermissionMock).toHaveBeenCalledOnce())
    request.resolve(businessPermissionSnapshot())
    await Promise.all([dashboardNavigation, pageNavigation])

    expect(getPermissionMock).toHaveBeenCalledOnce()
    expect(router.hasRoute('access:account:user:list')).toBe(true)
  })

  it('keeps Dashboard mounted when access loading fails', async () => {
    setAuthenticated()
    getPermissionMock.mockRejectedValue(new Error('access unavailable'))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/dashboard')

    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(usePermissionStore(pinia).status).toBe('error')
    expect(usePermissionStore(pinia).errorMessage).not.toBe('')
    expect(getPermissionMock).toHaveBeenCalledOnce()
  })

  it('redirects a cold dynamic URL to Dashboard when access loading fails', async () => {
    setAuthenticated()
    getPermissionMock.mockRejectedValue(new Error('access unavailable'))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/account/users')

    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(usePermissionStore(pinia).status).toBe('error')
    expect(getPermissionMock).toHaveBeenCalledOnce()
  })

  it('removes dynamic routes and access state after authentication becomes anonymous', async () => {
    setAuthenticated()
    getPermissionMock.mockResolvedValue(businessPermissionSnapshot())
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)
    await router.push('/account/users')
		expect(router.hasRoute('access:account:user:list')).toBe(true)

    useAuthStore(pinia).setAnonymous()
    await router.push('/login')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.hasRoute('access:account:user:list')).toBe(false)
    expect(usePermissionStore(pinia).status).toBe('idle')
    expect(usePermissionStore(pinia).menuTree).toEqual([])
  })

  it('does not bootstrap access for public Login', async () => {
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)
    await router.push('/login')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(refreshMock).not.toHaveBeenCalled()
    expect(getPermissionMock).not.toHaveBeenCalled()
  })

  it('redirects a refresh 401 to Login as anonymous', async () => {
    refreshMock.mockRejectedValue(new ApiError(10002, '未登录或登录已失效', 401))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/dashboard')

    expect(router.currentRoute.value.fullPath).toBe('/login?redirect=/dashboard')
    expect(useAuthStore(pinia).status).toBe('anonymous')
    expect(getCurrentUserMock).not.toHaveBeenCalled()
    expect(getPermissionMock).not.toHaveBeenCalled()
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
    expect(getPermissionMock).not.toHaveBeenCalled()
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
  store.setAuthenticated({ userId: 1, username: 'admin', email: 'admin@example.com', phone: null, avatar: '' })
}

function emptyPermissionSnapshot(): PermissionSnapshot {
  return { roleCodes: [], menuTree: [], permissionCodes: [] }
}

function businessPermissionSnapshot(): PermissionSnapshot {
  return {
    roleCodes: ['registered_user'],
		menuTree: [
			accessDirectory('account', 'navigation.account', [
				accessPage('account:user:list', '/account/users', 'account/users', 'navigation.accountUsers'),
			]),
			accessDirectory('access', 'navigation.access', [
				accessPage('permission:role:list', '/access/roles', 'access/roles', 'navigation.accessRoles'),
			]),
			accessDirectory('system', 'navigation.system', [
				accessPage('system:operation-log:list', '/system/operation-logs', 'system/operation-logs', 'navigation.systemOperationLogs'),
			]),
		],
		permissionCodes: ['account:user:list', 'system:operation-log:list', 'permission:role:list'],
  }
}

function accessDirectory(
	code: string,
	i18nKey: string,
	children: PermissionSnapshot['menuTree'],
): PermissionSnapshot['menuTree'][number] {
	return {
		code,
		menuType: 'directory',
		path: null,
		componentPath: null,
		i18nKey,
		icon: null,
		isHidden: YesNo.No,
		children,
	}
}

function accessPage(
	code: string,
	path: string,
	componentPath: string,
	i18nKey: string,
): PermissionSnapshot['menuTree'][number] {
	return {
		code,
		menuType: 'page',
		path,
		componentPath,
		i18nKey,
		icon: null,
		isHidden: YesNo.No,
		children: [],
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
