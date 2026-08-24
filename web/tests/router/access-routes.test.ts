import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'

import type { RouteViewMap } from '@src/access/route-views'
import type { AccessMenuNode } from '@src/api/access.contract'
import { ProtocolError } from '@src/types/http'
import { registerAccessRoutes } from '@src/router/access-routes'

const TestLayout = { template: '<router-view />' }
const TestView = { template: '<div>test view</div>' }
const testViews: RouteViewMap = {
  systemUsers: async () => TestView,
  systemTeams: async () => TestView,
}

describe('access route registration', () => {
  it('registers only pages below the named layout and preserves title metadata', () => {
    const router = testRouter()
		const cleanup = registerAccessRoutes(router, [directory('system', [
			page('system:menu:list', '/system/menus', 'systemUsers'),
    ])], testViews)

    expect(router.hasRoute('access:system')).toBe(false)
		expect(router.hasRoute('access:system:menu:list')).toBe(true)
		const resolved = router.resolve('/system/menus')
		expect(resolved.name).toBe('access:system:menu:list')
    expect(resolved.meta.requiresAuth).toBe(true)
		expect(resolved.meta.titleKey).toBe('navigation.systemMenus')
    expect(resolved.matched.map((record) => record.name)).toContain('admin-layout')

    cleanup()
		expect(router.hasRoute('access:system:menu:list')).toBe(false)
		expect(router.resolve('/system/menus').matched).toHaveLength(0)
  })

  it('rejects an unknown view key before registering anything', () => {
    const router = testRouter()
		const nodes = [page('system:menu:list', '/system/menus', 'missingView')]

    expect(() => registerAccessRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(router.getRoutes().filter((route) => String(route.name).startsWith('access:'))).toHaveLength(0)
  })

  it.each([
    {
      name: 'duplicate path',
      nodes: [
				page('system:menu:list', '/system/menus', 'systemUsers'),
				page('system:other:list', '/system/menus', 'systemTeams'),
      ],
    },
    {
      name: 'duplicate route name',
      nodes: [
				page('system:menu:list', '/system/menus', 'systemUsers'),
				page('system:menu:list', '/system/other-menus', 'systemUsers'),
      ],
    },
  ])('rejects $name before registering anything', ({ nodes }) => {
    const router = testRouter()
    expect(() => registerAccessRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(router.getRoutes().filter((route) => String(route.name).startsWith('access:'))).toHaveLength(0)
  })

  it('returns an idempotent cleanup for multiple flat pages', () => {
    const router = testRouter()
    const cleanup = registerAccessRoutes(router, [directory('system', [
			page('system:menu:list', '/system/menus', 'systemUsers'),
			page('system:other:list', '/system/other-menus', 'systemTeams'),
    ])], testViews)

		expect(router.hasRoute('access:system:menu:list')).toBe(true)
		expect(router.hasRoute('access:system:other:list')).toBe(true)
    cleanup()
    cleanup()
		expect(router.hasRoute('access:system:menu:list')).toBe(false)
		expect(router.hasRoute('access:system:other:list')).toBe(false)
  })
})

function testRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{
      path: '/',
      name: 'admin-layout',
      component: TestLayout,
      meta: { requiresAuth: true },
      children: [],
    }],
  })
}

function directory(code: string, children: AccessMenuNode[]): AccessMenuNode {
  return {
    code,
    menuType: 'directory',
    path: null,
    viewKey: null,
		titleKey: 'navigation.system',
    icon: null,
    children,
  }
}

function page(code: string, path: string, viewKey: string): AccessMenuNode {
  return {
    code,
    menuType: 'page',
    path,
    viewKey,
		titleKey: 'navigation.systemMenus',
    icon: null,
    children: [],
  }
}
