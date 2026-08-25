import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'

import type { AccessMenuNode } from '@src/api/access.contract'
import { YesNo } from '@src/enums/yes-no'
import { registerAccessRoutes, type PageModuleMap } from '@src/router/access-routes'
import { ProtocolError } from '@src/types/http'

const TestLayout = { template: '<router-view />' }
const TestView = { template: '<div>test view</div>' }
const testViews: PageModuleMap = {
  '../views/system/users/index.vue': async () => ({ default: TestView }),
  '../views/system/roles/index.vue': async () => ({ default: TestView }),
}

describe('access route registration', () => {
  it('maps a route URL to its independent component path and i18n metadata', () => {
    const router = testRouter()
    const cleanup = registerAccessRoutes(router, [directory('system', [
      page('system:user:list', '/system/users', 'system/users'),
    ])], testViews)

    expect(router.hasRoute('access:system')).toBe(false)
    expect(router.hasRoute('access:system:user:list')).toBe(true)
    const resolved = router.resolve('/system/users')
    expect(resolved.name).toBe('access:system:user:list')
    expect(resolved.meta.requiresAuth).toBe(true)
    expect(resolved.meta.i18nKey).toBe('navigation.systemUsers')
    expect(resolved.matched.map((record) => record.name)).toContain('admin-layout')

    cleanup()
    expect(router.hasRoute('access:system:user:list')).toBe(false)
    expect(router.resolve('/system/users').matched).toHaveLength(0)
  })

  it('allows two URLs to reuse one component and ignores hidden state', () => {
    const router = testRouter()
    const hidden = page('system:account:list', '/system/accounts', 'system/users')
    hidden.isHidden = YesNo.Yes
    const cleanup = registerAccessRoutes(router, [directory('system', [
      hidden,
      page('system:user:list', '/system/users', 'system/users'),
    ])], testViews)

    expect(router.hasRoute('access:system:account:list')).toBe(true)
    expect(router.hasRoute('access:system:user:list')).toBe(true)
    cleanup()
  })

  it('rejects an unknown component path before registering anything', () => {
    const router = testRouter()
    const nodes = [page('system:user:list', '/system/users', 'system/missing')]

    expect(() => registerAccessRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it.each([
    {
      name: 'duplicate path',
      nodes: [
        page('system:user:list', '/system/users', 'system/users'),
        page('system:other:list', '/system/users', 'system/roles'),
      ],
    },
    {
      name: 'duplicate route name',
      nodes: [
        page('system:user:list', '/system/users', 'system/users'),
        page('system:user:list', '/system/accounts', 'system/users'),
      ],
    },
    {
      name: 'static path conflict',
      nodes: [page('system:menu:list', '/system/menus', 'system/users')],
    },
  ])('rejects $name before registering anything', ({ nodes }) => {
    const router = testRouter(true)
    expect(() => registerAccessRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it('removes routes in reverse when addRoute fails partway through', () => {
    const router = testRouter()
    const originalAddRoute = router.addRoute.bind(router)
    let calls = 0
    vi.spyOn(router, 'addRoute').mockImplementation((...args) => {
      calls += 1
      if (calls === 2) throw new Error('add route failed')
      return Reflect.apply(originalAddRoute, router, args)
    })

    expect(() => registerAccessRoutes(router, [directory('system', [
      page('system:role:list', '/system/roles', 'system/roles'),
      page('system:user:list', '/system/users', 'system/users'),
    ])], testViews)).toThrow('add route failed')
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it('returns an idempotent cleanup for multiple pages', () => {
    const router = testRouter()
    const cleanup = registerAccessRoutes(router, [directory('system', [
      page('system:role:list', '/system/roles', 'system/roles'),
      page('system:user:list', '/system/users', 'system/users'),
    ])], testViews)

    cleanup()
    cleanup()
    expect(accessRoutes(router)).toHaveLength(0)
  })
})

function testRouter(includeStaticMenu = false) {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{
      path: '/',
      name: 'admin-layout',
      component: TestLayout,
      meta: { requiresAuth: true },
      children: includeStaticMenu ? [{
        path: 'system/menus',
        name: 'system-menus',
        component: TestView,
        meta: { requiresAuth: true, requiredPermission: 'system:menu:list' },
      }] : [],
    }],
  })
}

function accessRoutes(router: ReturnType<typeof testRouter>) {
  return router.getRoutes().filter((route) => String(route.name).startsWith('access:'))
}

function directory(code: string, children: AccessMenuNode[]): AccessMenuNode {
  return {
    code,
    menuType: 'directory',
    path: null,
    componentPath: null,
    i18nKey: 'navigation.system',
    icon: null,
    isHidden: YesNo.No,
    children,
  }
}

function page(code: string, path: string, componentPath: string): AccessMenuNode {
  return {
    code,
    menuType: 'page',
    path,
    componentPath,
    i18nKey: code === 'system:user:list' ? 'navigation.systemUsers' : 'navigation.systemRoles',
    icon: null,
    isHidden: YesNo.No,
    children: [],
  }
}
