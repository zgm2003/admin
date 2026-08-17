import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { describe, expect, it } from 'vitest'

import { createAppRouter } from './index'
import { installPermissionGuard } from '../permission'

describe('router', () => {
  it('redirects the root to the explicitly public dashboard', async () => {
    const router = createAppRouter(createMemoryHistory())
    installPermissionGuard(router)

    await router.push('/')
    await router.isReady()

    expect(router.currentRoute.value.fullPath).toBe('/dashboard')
    expect(router.currentRoute.value.meta.requiresAuth).toBe(false)
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

    await expect(router.push('/missing-meta')).rejects.toThrow(
      'Route /missing-meta must declare requiresAuth',
    )
  })
})
