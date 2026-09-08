import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'

import type { PermissionMenuNode } from '@/api/permission/permission'
import { YesNo } from '@/enums/yes-no'
import { registerPermissionRoutes, type PageModuleMap } from '@/router/permission-routes'
import { ProtocolError } from '@/types/http'

const TestLayout = { template: '<router-view />' }
const TestView = { template: '<div>test view</div>' }
const testViews: PageModuleMap = {
  '../views/user/account/index.vue': async () => ({ default: TestView }),
  '../views/user/profile/index.vue': async () => ({ default: TestView }),
  '../views/user/session/index.vue': async () => ({ default: TestView }),
  '../views/user/loginlog/index.vue': async () => ({ default: TestView }),
  '../views/permission/authplatform/index.vue': async () => ({ default: TestView }),
  '../views/permission/menu/index.vue': async () => ({ default: TestView }),
  '../views/permission/role/index.vue': async () => ({ default: TestView }),
  '../views/system/operationlog/index.vue': async () => ({ default: TestView }),
  '../views/storage/object/index.vue': async () => ({ default: TestView }),
  '../views/message/mail/index.vue': async () => ({ default: TestView }),
}

describe('access route registration', () => {
  it('maps a route URL to its independent component path and i18n metadata', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(
      router,
      [directory('account', [page('user:account:view', '/user/account', 'user/account')])],
      testViews,
    )

    expect(router.hasRoute('access:system')).toBe(false)
    expect(router.hasRoute('access:user:account:view')).toBe(true)
    const resolved = router.resolve('/user/account')
    expect(resolved.name).toBe('access:user:account:view')
    expect(resolved.meta.requiresAuth).toBe(true)
    expect(resolved.meta.i18nKey).toBe('navigation.userAccount')
    expect(resolved.matched.map((record) => record.name)).toContain('admin-layout')

    cleanup()
    expect(router.hasRoute('access:user:account:view')).toBe(false)
    expect(router.resolve('/user/account').matched).toHaveLength(0)
  })

  it('registers menu pages from every root dynamically', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(
      router,
      [
        directory('account', [
          page('user:account:view', '/user/account', 'user/account'),
          page('user:session:view', '/user/session', 'user/session'),
        ]),
        directory('access', [
          page('permission:menu:view', '/permission/menu', 'permission/menu'),
          page('permission:role:view', '/permission/role', 'permission/role'),
          page(
            'permission:authplatform:view',
            '/permission/authplatform',
            'permission/authplatform',
          ),
        ]),
        directory('system', [
          page('system:operationlog:view', '/system/operationlog', 'system/operationlog'),
        ]),
      ],
      testViews,
    )

    expect(router.resolve('/permission/menu').name).toBe('access:permission:menu:view')
    expect(router.hasRoute('access:permission:menu:view')).toBe(true)
    expect(router.hasRoute('access:user:account:view')).toBe(true)
    expect(router.hasRoute('access:user:session:view')).toBe(true)
    expect(router.hasRoute('access:permission:role:view')).toBe(true)
    expect(router.hasRoute('access:permission:authplatform:view')).toBe(true)
    expect(router.hasRoute('access:system:operationlog:view')).toBe(true)

    cleanup()
    expect(router.hasRoute('access:permission:menu:view')).toBe(false)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it('loads login logs from the account view module', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(
      router,
      [directory('account', [page('user:loginlog:list', '/user/loginlog', 'user/loginlog')])],
      testViews,
    )

    expect(router.resolve('/user/loginlog').name).toBe('access:user:loginlog:list')
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
    const hidden = page('system:account:view', '/system/accounts', 'user/account')
    hidden.isHidden = YesNo.Yes
    const cleanup = registerPermissionRoutes(
      router,
      [directory('system', [hidden, page('user:account:view', '/user/account', 'user/account')])],
      testViews,
    )

    expect(router.hasRoute('access:system:account:view')).toBe(true)
    expect(router.hasRoute('access:user:account:view')).toBe(true)
    cleanup()
  })

  it('rejects an unknown component path before registering anything', () => {
    const router = testRouter()
    const nodes = [page('user:account:view', '/user/account', 'system/missing')]

    expect(() => registerPermissionRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it.each([
    {
      name: 'duplicate path',
      nodes: [
        page('user:account:view', '/user/account', 'user/account'),
        page('system:other:view', '/user/account', 'permission/role'),
      ],
    },
    {
      name: 'duplicate route name',
      nodes: [
        page('user:account:view', '/user/account', 'user/account'),
        page('user:account:view', '/system/accounts', 'user/account'),
      ],
    },
  ])('rejects $name before registering anything', ({ nodes }) => {
    const router = testRouter()
    expect(() => registerPermissionRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it('registers the hidden profile page dynamically with its access route name', () => {
    const router = testRouter()
    const profile = page('user:profile:view', '/user/profile', 'user/profile')
    profile.i18nKey = 'layout.user.profile'
    profile.isHidden = YesNo.Yes
    const cleanup = registerPermissionRoutes(router, [directory('account', [profile])], testViews)

    expect(router.hasRoute('account-profile')).toBe(false)
    expect(router.hasRoute('access:user:profile:view')).toBe(true)
    expect(router.resolve('/user/profile').name).toBe('access:user:profile:view')
    cleanup()
    expect(router.resolve('/user/profile').matched).toHaveLength(0)
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
            page('permission:role:view', '/permission/role', 'permission/role'),
            page('user:account:view', '/user/account', 'user/account'),
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
          page('permission:role:view', '/permission/role', 'permission/role'),
          page('user:account:view', '/user/account', 'user/account'),
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
    'user:profile:view': 'layout.user.profile',
    'user:account:view': 'navigation.userAccount',
    'system:operationlog:view': 'navigation.systemOperationlog',
    'permission:authplatform:view': 'navigation.permissionAuthplatform',
    'user:session:view': 'navigation.userSession',
    'user:loginlog:list': 'navigation.userLoginlog',
    'user:loginlog:view': 'navigation.userLoginlog',
    'permission:menu:view': 'navigation.permissionMenu',
    'permission:role:view': 'navigation.permissionRole',
  }
  return keys[code] ?? 'navigation.userAccount'
}
