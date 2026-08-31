import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'

import type { PermissionMenuNode } from '@src/api/permission/permission'
import { YesNo } from '@src/enums/yes-no'
import { registerPermissionRoutes, type PageModuleMap } from '@src/router/permission-routes'
import { ProtocolError } from '@src/types/http'

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
}

describe('access route registration', () => {
  it('maps a route URL to its independent component path and i18n metadata', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(router, [directory('account', [
      page('account:user:list', '/account/users', 'account/users'),
    ])], testViews)

    expect(router.hasRoute('access:system')).toBe(false)
    expect(router.hasRoute('access:account:user:list')).toBe(true)
    const resolved = router.resolve('/account/users')
    expect(resolved.name).toBe('access:account:user:list')
    expect(resolved.meta.requiresAuth).toBe(true)
    expect(resolved.meta.i18nKey).toBe('navigation.accountUsers')
    expect(resolved.matched.map((record) => record.name)).toContain('admin-layout')

    cleanup()
    expect(router.hasRoute('access:account:user:list')).toBe(false)
    expect(router.resolve('/account/users').matched).toHaveLength(0)
  })

	it('reuses the exact static menu binding and registers pages from every root', () => {
		const router = testRouter(true)
		const cleanup = registerPermissionRoutes(router, [
			directory('account', [
				page('account:user:list', '/account/users', 'account/users'),
				page('auth:session:list', '/account/sessions', 'account/sessions'),
			]),
			directory('access', [
				page('permission:menu:list', '/access/menus', 'access/menus'),
				page('permission:role:list', '/access/roles', 'access/roles'),
				page('auth:platform:list', '/access/auth-platforms', 'access/auth-platforms'),
			]),
			directory('system', [
				page('system:operation-log:list', '/system/operation-logs', 'system/operation-logs'),
			]),
		], testViews)

		expect(router.resolve('/access/menus').name).toBe('access-menus')
		expect(router.hasRoute('access:permission:menu:list')).toBe(false)
		expect(router.hasRoute('access:account:user:list')).toBe(true)
		expect(router.hasRoute('access:auth:session:list')).toBe(true)
		expect(router.hasRoute('access:permission:role:list')).toBe(true)
		expect(router.hasRoute('access:auth:platform:list')).toBe(true)
		expect(router.hasRoute('access:system:operation-log:list')).toBe(true)

		cleanup()
		expect(router.hasRoute('access-menus')).toBe(true)
		expect(accessRoutes(router)).toHaveLength(0)
	})

	it('loads login logs from the account view module', () => {
		const router = testRouter()
		const cleanup = registerPermissionRoutes(router, [directory('account', [
			page('account:user:loginlog:list', '/account/login-logs', 'user/login-logs'),
		])], testViews)

		expect(router.resolve('/account/login-logs').name).toBe('access:account:user:loginlog:list')
		cleanup()
	})

	it('allows two URLs to reuse one component and ignores hidden state', () => {
    const router = testRouter()
    const hidden = page('system:account:list', '/system/accounts', 'account/users')
    hidden.isHidden = YesNo.Yes
    const cleanup = registerPermissionRoutes(router, [directory('system', [
      hidden,
      page('account:user:list', '/account/users', 'account/users'),
    ])], testViews)

    expect(router.hasRoute('access:system:account:list')).toBe(true)
    expect(router.hasRoute('access:account:user:list')).toBe(true)
    cleanup()
  })

  it('rejects an unknown component path before registering anything', () => {
    const router = testRouter()
    const nodes = [page('account:user:list', '/account/users', 'system/missing')]

    expect(() => registerPermissionRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it.each([
    {
      name: 'duplicate path',
      nodes: [
        page('account:user:list', '/account/users', 'account/users'),
        page('system:other:list', '/account/users', 'access/roles'),
      ],
    },
    {
      name: 'duplicate route name',
      nodes: [
        page('account:user:list', '/account/users', 'account/users'),
        page('account:user:list', '/system/accounts', 'account/users'),
      ],
    },
		{
			name: 'static code mismatch',
			nodes: [page('permission:other:list', '/access/menus', 'access/menus')],
		},
		{
			name: 'static path mismatch',
			nodes: [page('permission:menu:list', '/access/menu-settings', 'access/menus')],
		},
		{
			name: 'static component mismatch',
			nodes: [page('permission:menu:list', '/access/menus', 'account/users')],
		},
  ])('rejects $name before registering anything', ({ nodes }) => {
    const router = testRouter(true)
    expect(() => registerPermissionRoutes(router, nodes, testViews)).toThrow(ProtocolError)
    expect(accessRoutes(router)).toHaveLength(0)
  })

	it('rejects an incorrectly named static menu route', () => {
		const router = testRouter(true, 'system-menus')
		expect(() => registerPermissionRoutes(router, [
			directory('access', [page('permission:menu:list', '/access/menus', 'access/menus')]),
		], testViews)).toThrow(ProtocolError)
		expect(accessRoutes(router)).toHaveLength(0)
	})

	it('registers the hidden profile page dynamically with its access route name', () => {
		const router = testRouter()
		const profile = page('account:profile:list', '/account/profile', 'account/profile')
		profile.i18nKey = 'layout.account.profile'
		profile.isHidden = YesNo.Yes
		const cleanup = registerPermissionRoutes(router, [directory('account', [profile])], testViews)

		expect(router.hasRoute('account-profile')).toBe(false)
		expect(router.hasRoute('access:account:profile:list')).toBe(true)
		expect(router.resolve('/account/profile').name).toBe('access:account:profile:list')
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

    expect(() => registerPermissionRoutes(router, [directory('system', [
      page('permission:role:list', '/access/roles', 'access/roles'),
      page('account:user:list', '/account/users', 'account/users'),
    ])], testViews)).toThrow('add route failed')
    expect(accessRoutes(router)).toHaveLength(0)
  })

  it('returns an idempotent cleanup for multiple pages', () => {
    const router = testRouter()
    const cleanup = registerPermissionRoutes(router, [directory('system', [
      page('permission:role:list', '/access/roles', 'access/roles'),
      page('account:user:list', '/account/users', 'account/users'),
    ])], testViews)

    cleanup()
    cleanup()
    expect(accessRoutes(router)).toHaveLength(0)
  })
})

function testRouter(includeStaticMenu = false, staticMenuRouteName = 'access-menus') {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{
      path: '/',
      name: 'admin-layout',
      component: TestLayout,
      meta: { requiresAuth: true },
      children: includeStaticMenu ? [{
        path: 'access/menus',
				name: staticMenuRouteName,
        component: TestView,
				meta: {
					requiresAuth: true,
					i18nKey: 'navigation.accessMenus',
					requiredPermission: 'permission:menu:list',
				},
      }] : [],
    }],
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
		'account:profile:list': 'layout.account.profile',
		'account:user:list': 'navigation.accountUsers',
		'system:operation-log:list': 'navigation.systemOperationLogs',
		'auth:platform:list': 'navigation.accessAuthPlatforms',
		'auth:session:list': 'navigation.accountSessions',
		'account:user:loginlog:list': 'navigation.accountLoginLogs',
		'permission:menu:list': 'navigation.accessMenus',
		'permission:role:list': 'navigation.accessRoles',
	}
	return keys[code] ?? 'navigation.accountUsers'
}
