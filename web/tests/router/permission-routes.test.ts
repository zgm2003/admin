import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'

import type { PermissionMenuNode } from '@/api/permission/permission'
import { YesNo } from '@/enums/yes-no'
import { registerPermissionRoutes, type PageModuleMap } from '@/router/permission-routes'
import { ProtocolError } from '@/types/http'

const TestLayout = { template: '<router-view />' }
const TestView = { template: '<div>test view</div>' }
const testViews: PageModuleMap = {
  '../views/account/users/index.vue': async () => ({ default: TestView }),
  '../views/account/profile/index.vue': async () => ({ default: TestView }),
  '../views/account/sessions/index.vue': async () => ({ default: TestView }),
  '../views/account/login-logs/index.vue': async () => ({ default: TestView }),
  '../views/permission/auth-platforms/index.vue': async () => ({ default: TestView }),
  '../views/permission/menus/index.vue': async () => ({ default: TestView }),
  '../views/permission/roles/index.vue': async () => ({ default: TestView }),
  '../views/system/operation-logs/index.vue': async () => ({ default: TestView }),
  '../views/cloud/storage-object/index.vue': async () => ({ default: TestView }),
  '../views/message/mail/index.vue': async () => ({ default: TestView }),
}

describe('access route registration', () => {
  it('maps a route URL to its independent component path and i18n metadata', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(
      router,
      [directory('account', [page('account:user:view', '/account/users', 'account/users')])],
      testViews,
    )

    expect(router.hasRoute('access:system')).toBe(false)
    expect(router.hasRoute('access:account:user:view')).toBe(true)
    const resolved = router.resolve('/account/users')
    expect(resolved.name).toBe('access:account:user:view')
    expect(resolved.meta.requiresAuth).toBe(true)
    expect(resolved.meta.i18nKey).toBe('navigation.accountUsers')
    expect(resolved.matched.map((record) => record.name)).toContain('admin-layout')

    cleanup()
    expect(router.hasRoute('access:account:user:view')).toBe(false)
    expect(router.resolve('/account/users').matched).toHaveLength(0)
  })

  it('registers menu pages from every root dynamically', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(
      router,
      [
        directory('account', [
          page('account:user:view', '/account/users', 'account/users'),
          page('auth:session:view', '/account/sessions', 'account/sessions'),
        ]),
        directory('access', [
          page('permission:menu:view', '/permission/menus', 'permission/menus'),
          page('permission:role:view', '/permission/roles', 'permission/roles'),
          page('auth:platform:view', '/permission/auth-platforms', 'permission/auth-platforms'),
        ]),
        directory('system', [
          page('system:operation-log:view', '/system/operation-logs', 'system/operation-logs'),
        ]),
      ],
      testViews,
    )

    expect(router.resolve('/permission/menus').name).toBe('access:permission:menu:view')
    expect(router.hasRoute('access:permission:menu:view')).toBe(true)
    expect(router.hasRoute('access:account:user:view')).toBe(true)
    expect(router.hasRoute('access:auth:session:view')).toBe(true)
    expect(router.hasRoute('access:permission:role:view')).toBe(true)
    expect(router.hasRoute('access:auth:platform:view')).toBe(true)
    expect(router.hasRoute('access:system:operation-log:view')).toBe(true)

    cleanup()
    expect(router.hasRoute('access:permission:menu:view')).toBe(false)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it('loads login logs from the account view module', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(
      router,
      [
        directory('account', [
          page('account:user:loginlog:list', '/account/login-logs', 'account/login-logs'),
        ]),
      ],
      testViews,
    )

    expect(router.resolve('/account/login-logs').name).toBe('access:account:user:loginlog:list')
    cleanup()
  })

  it('loads mail service from the message view module', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(
      router,
      [directory('message', [page('message:mail:view', '/message/mail', 'message/mail')])],
      testViews,
    )

    expect(router.resolve('/message/mail').name).toBe('access:message:mail:view')
    cleanup()
  })

  it('allows two URLs to reuse one component and ignores hidden state', () => {
    const router = testRouter()
    const hidden = page('system:account:view', '/system/accounts', 'account/users')
    hidden.isHidden = YesNo.Yes
    const cleanup = registerPermissionRoutes(
      router,
      [directory('system', [hidden, page('account:user:view', '/account/users', 'account/users')])],
      testViews,
    )

    expect(router.hasRoute('access:system:account:view')).toBe(true)
    expect(router.hasRoute('access:account:user:view')).toBe(true)
    cleanup()
  })

  it('rejects an unknown component path before registering anything', () => {
    const router = testRouter()
    const nodes = [page('account:user:view', '/account/users', 'system/missing')]

    expect(() => registerPermissionRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it.each([
    {
      name: 'duplicate path',
      nodes: [
        page('account:user:view', '/account/users', 'account/users'),
        page('system:other:view', '/account/users', 'permission/roles'),
      ],
    },
    {
      name: 'duplicate route name',
      nodes: [
        page('account:user:view', '/account/users', 'account/users'),
        page('account:user:view', '/system/accounts', 'account/users'),
      ],
    },
  ])('rejects $name before registering anything', ({ nodes }) => {
    const router = testRouter()
    expect(() => registerPermissionRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it('registers the hidden profile page dynamically with its access route name', () => {
    const router = testRouter()
    const profile = page('account:profile:view', '/account/profile', 'account/profile')
    profile.i18nKey = 'layout.account.profile'
    profile.isHidden = YesNo.Yes
    const cleanup = registerPermissionRoutes(router, [directory('account', [profile])], testViews)

    expect(router.hasRoute('account-profile')).toBe(false)
    expect(router.hasRoute('access:account:profile:view')).toBe(true)
    expect(router.resolve('/account/profile').name).toBe('access:account:profile:view')
    cleanup()
    expect(router.resolve('/account/profile').matched).toHaveLength(0)
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

    expect(() =>
      registerPermissionRoutes(
        router,
        [
          directory('system', [
            page('permission:role:view', '/permission/roles', 'permission/roles'),
            page('account:user:view', '/account/users', 'account/users'),
          ]),
        ],
        testViews,
      ),
    ).toThrow('add route failed')
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it('returns an idempotent cleanup for multiple pages', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(
      router,
      [
        directory('system', [
          page('permission:role:view', '/permission/roles', 'permission/roles'),
          page('account:user:view', '/account/users', 'account/users'),
        ]),
      ],
      testViews,
    )

    cleanup()
    cleanup()
    expect(accessRoutes(router)).toHaveLength(0)
  })
})

function testRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/',
        name: 'admin-layout',
        component: TestLayout,
        meta: { requiresAuth: true },
        children: [],
      },
    ],
  })
}

function accessRoutes(router: ReturnType<typeof testRouter>) {
  return router.getRoutes().filter((route) => String(route.name).startsWith('access:'))
}

function directory(code: string, children: PermissionMenuNode[]): PermissionMenuNode {
  return {
    code,
    menuType: 'directory',
    path: null,
    componentPath: null,
    i18nKey: `navigation.${code}`,
    icon: null,
    isHidden: YesNo.No,
    children,
  }
}

function page(code: string, path: string, componentPath: string): PermissionMenuNode {
  return {
    code,
    menuType: 'page',
    path,
    componentPath,
    i18nKey: pageI18nKey(code),
    icon: null,
    isHidden: YesNo.No,
    children: [],
  }
}

function pageI18nKey(code: string): string {
  const keys: Readonly<Record<string, string>> = {
    'account:profile:view': 'layout.account.profile',
    'account:user:view': 'navigation.accountUsers',
    'system:operation-log:view': 'navigation.systemOperationLogs',
    'auth:platform:view': 'navigation.accessAuthPlatforms',
    'auth:session:view': 'navigation.accountSessions',
    'account:user:loginlog:list': 'navigation.accountLoginLogs',
    'account:user:loginlog:view': 'navigation.accountLoginLogs',
    'permission:menu:view': 'navigation.accessMenus',
    'permission:role:view': 'navigation.accessRoles',
  }
  return keys[code] ?? 'navigation.accountUsers'
}
