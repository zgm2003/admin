import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser, refresh } from '../api/auth'
import { installPermissionGuard } from '../permission'
import { pinia } from '../store'
import { useAuthStore } from '../store/auth'
import { ApiError } from '../types/http'
import { createAppRouter } from './index'

vi.mock('../api/auth', () => ({ refresh: vi.fn(), getCurrentUser: vi.fn() }))

const refreshMock = vi.mocked(refresh)
const getCurrentUserMock = vi.mocked(getCurrentUser)

describe('router', () => {
  beforeEach(() => {
    useAuthStore(pinia).$reset()
    refreshMock.mockReset()
    getCurrentUserMock.mockReset()
  })

  it('declares Login as public, removes Register, and protects Dashboard', () => {
    const router = createAppRouter(createMemoryHistory())
    expect(router.resolve('/login').meta.requiresAuth).toBe(false)
    expect(router.resolve('/register').matched).toHaveLength(0)
    expect(router.resolve('/dashboard').meta.requiresAuth).toBe(true)
  })

  it('declares a translated fixed title for Dashboard', () => {
    const router = createAppRouter(createMemoryHistory())
    const dashboard = router.resolve('/dashboard')
    expect(dashboard.meta.requiresAuth).toBe(true)
    expect(dashboard.meta.titleKey).toBe('navigation.dashboard')
    expect(dashboard.meta.affix).toBe(true)
    expect(router.resolve('/login').meta.titleKey).toBeUndefined()
  })

  it('restores a cold protected route through refresh then current user', async () => {
    const order: string[] = []
    refreshMock.mockImplementation(async () => {
      order.push('refresh')
      return { accessToken: 'jwt', expiresIn: 900 }
    })
    getCurrentUserMock.mockImplementation(async () => {
      order.push('me')
      return { userId: 1, username: 'admin', email: 'admin@example.com' }
    })
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/dashboard')
    await router.isReady()
    expect(router.currentRoute.value.fullPath).toBe('/dashboard')
    expect(order).toEqual(['refresh', 'me'])
    expect(useAuthStore(pinia).status).toBe('authenticated')
  })

  it('redirects a refresh 401 to Login as anonymous', async () => {
    refreshMock.mockRejectedValue(new ApiError(10002, '未登录或登录已失效', 401))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/dashboard')
    await router.isReady()
    expect(router.currentRoute.value.fullPath).toBe('/login?redirect=/dashboard')
    expect(useAuthStore(pinia).status).toBe('anonymous')
    expect(getCurrentUserMock).not.toHaveBeenCalled()
  })

  it('does not bootstrap authenticated or public navigation', async () => {
    const store = useAuthStore(pinia)
    store.setCredential({ accessToken: 'jwt', expiresIn: 900 })
    store.setAuthenticated({ userId: 1, username: 'admin', email: 'admin@example.com' })
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)
    await router.push('/dashboard')
    expect(refreshMock).not.toHaveBeenCalled()

    store.setAnonymous()
    await router.push('/login')
    expect(router.currentRoute.value.path).toBe('/login')
    expect(refreshMock).not.toHaveBeenCalled()
  })

  it('keeps dependency failures distinct from anonymous', async () => {
    refreshMock.mockRejectedValue(new ApiError(10006, '服务暂未就绪', 503))
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)
    await router.push('/dashboard')
    await router.isReady()

    const store = useAuthStore(pinia)
    expect(store.status).toBe('error')
    expect(store.errorMessage).toBe('服务暂未就绪')
    expect(router.currentRoute.value.path).toBe('/login')
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
