import { describe, expect, it } from 'vitest'

import type { AccessMenuNode } from '@src/api/rbac/access'
import { YesNo } from '@src/enums/yes-no'
import { resolveBreadcrumbs } from '@src/layout/breadcrumbs'

describe('resolveBreadcrumbs', () => {
	it('returns the fixed Dashboard breadcrumb', () => {
    expect(resolveBreadcrumbs('/dashboard', [])).toEqual([
		{ path: '/dashboard', i18nKey: 'navigation.dashboard' },
    ])
  })

  it('returns directory to leaf order without inventing a directory path', () => {
    expect(resolveBreadcrumbs('/account/users', [accountDirectory()])).toEqual([
		{ path: null, i18nKey: 'navigation.account' },
		{ path: '/account/users', i18nKey: 'navigation.accountUsers' },
    ])
  })

  it('resolves nested directories without changing the input tree', () => {
    const tree = [nestedDirectory()]
    const before = JSON.stringify(tree)

    expect(resolveBreadcrumbs('/system/security/sessions', tree)).toEqual([
		{ path: null, i18nKey: 'navigation.system' },
		{ path: null, i18nKey: 'navigation.accessAuthPlatforms' },
		{ path: '/system/security/sessions', i18nKey: 'navigation.accountSessions' },
    ])
    expect(JSON.stringify(tree)).toBe(before)
	})

	it('resolves every business root and does not invent a static menu breadcrumb', () => {
		const tree = [accountDirectory(), accessDirectory(), systemDirectory()]
		expect(resolveBreadcrumbs('/access/menus', [])).toBeNull()
		expect(resolveBreadcrumbs('/access/menus', tree)).toEqual([
			{ path: null, i18nKey: 'navigation.access' },
			{ path: '/access/menus', i18nKey: 'navigation.accessMenus' },
		])
		expect(resolveBreadcrumbs('/system/operation-logs', tree)).toEqual([
			{ path: null, i18nKey: 'navigation.system' },
			{ path: '/system/operation-logs', i18nKey: 'navigation.systemOperationLogs' },
		])
	})

	it('keeps hidden pages in the breadcrumb source tree', () => {
		const root = accountDirectory()
		root.children[0].isHidden = YesNo.Yes
		expect(resolveBreadcrumbs('/account/users', [root])).toEqual([
			{ path: null, i18nKey: 'navigation.account' },
			{ path: '/account/users', i18nKey: 'navigation.accountUsers' },
		])
	})

	it('resolves the hidden profile breadcrumb from the access tree', () => {
		const root = accountDirectory()
		root.children = [pageNode('account:profile:list', '/account/profile', 'account/profile', 'layout.account.profile')]
		root.children[0].isHidden = YesNo.Yes
		expect(resolveBreadcrumbs('/account/profile', [root])).toEqual([
			{ path: null, i18nKey: 'navigation.account' },
			{ path: '/account/profile', i18nKey: 'layout.account.profile' },
		])
	})

  it('returns null for an authenticated path missing from the access tree', () => {
    expect(resolveBreadcrumbs('/system/missing', [systemDirectory()])).toBeNull()
  })
})

function accountDirectory(): AccessMenuNode {
  return {
		code: 'account',
    menuType: 'directory',
    path: null,
		componentPath: null,
		i18nKey: 'navigation.account',
      icon: 'lucide:folder',
		isHidden: YesNo.No,
    children: [{
      code: 'account:user:list',
      menuType: 'page',
      path: '/account/users',
		componentPath: 'account/users',
		i18nKey: 'navigation.accountUsers',
      icon: 'User',
		isHidden: YesNo.No,
      children: [],
    }],
  }
}

function accessDirectory(): AccessMenuNode {
	return directoryNode('access', 'navigation.access', pageNode(
		'rbac:menu:list',
		'/access/menus',
		'access/menus',
		'navigation.accessMenus',
	))
}

function systemDirectory(): AccessMenuNode {
	return directoryNode('system', 'navigation.system', pageNode(
		'system:operation-log:list',
		'/system/operation-logs',
		'system/operation-logs',
		'navigation.systemOperationLogs',
	))
}

function directoryNode(code: string, i18nKey: string, child: AccessMenuNode): AccessMenuNode {
	return {
		code,
		menuType: 'directory',
		path: null,
		componentPath: null,
		i18nKey,
		icon: 'lucide:folder',
		isHidden: YesNo.No,
		children: [child],
	}
}

function pageNode(code: string, path: string, componentPath: string, i18nKey: string): AccessMenuNode {
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

function nestedDirectory(): AccessMenuNode {
  return {
    code: 'system',
    menuType: 'directory',
    path: null,
		componentPath: null,
		i18nKey: 'navigation.system',
    icon: 'lucide:folder',
		isHidden: YesNo.No,
    children: [{
      code: 'system:security',
      menuType: 'directory',
      path: null,
		componentPath: null,
		i18nKey: 'navigation.accessAuthPlatforms',
      icon: 'lucide:key-round',
		isHidden: YesNo.No,
      children: [{
        code: 'system:security:sessions',
        menuType: 'page',
        path: '/system/security/sessions',
			componentPath: 'account/sessions',
			i18nKey: 'navigation.accountSessions',
        icon: 'lucide:list-tree',
			isHidden: YesNo.No,
        children: [],
      }],
    }],
  }
}
