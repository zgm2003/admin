import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it } from 'vitest'

import type { RouteViewMap } from '../access/route-views'
import type { AccessMenuNode } from '../api/access.contract'
import { ProtocolError } from '../types/http'
import { registerAccessRoutes } from './access-routes'

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
      page('system:user:view', '/system/users', 'systemUsers'),
    ])], testViews)

    expect(router.hasRoute('access:system')).toBe(false)
    expect(router.hasRoute('access:system:user:view')).toBe(true)
    const resolved = router.resolve('/system/users')
    expect(resolved.name).toBe('access:system:user:view')
    expect(resolved.meta.requiresAuth).toBe(true)
    expect(resolved.meta.titleKey).toBe('navigation.dashboard')
    expect(resolved.matched.map((record) => record.name)).toContain('admin-layout')

    cleanup()
    expect(router.hasRoute('access:system:user:view')).toBe(false)
    expect(router.resolve('/system/users').matched).toHaveLength(0)
  })

  it('rejects an unknown view key before registering anything', () => {
    const router = testRouter()
    const nodes = [page('system:user:view', '/system/users', 'missingView')]

    expect(() => registerAccessRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(router.getRoutes().filter((route) => String(route.name).startsWith('access:'))).toHaveLength(0)
  })

  it.each([
    {
      name: 'duplicate path',
      nodes: [
        page('system:user:view', '/system/users', 'systemUsers'),
        page('system:team:view', '/system/users', 'systemTeams'),
      ],
    },
    {
      name: 'duplicate route name',
      nodes: [
        page('system:user:view', '/system/users', 'systemUsers'),
        page('system:user:view', '/system/other-users', 'systemUsers'),
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
      page('system:user:view', '/system/users', 'systemUsers'),
      page('system:team:view', '/system/teams', 'systemTeams'),
    ])], testViews)

    expect(router.hasRoute('access:system:user:view')).toBe(true)
    expect(router.hasRoute('access:system:team:view')).toBe(true)
    cleanup()
    cleanup()
    expect(router.hasRoute('access:system:user:view')).toBe(false)
    expect(router.hasRoute('access:system:team:view')).toBe(false)
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
    titleKey: 'navigation.main',
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
    titleKey: 'navigation.dashboard',
    icon: null,
    children: [],
  }
}
