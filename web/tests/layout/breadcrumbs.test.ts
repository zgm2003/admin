import { describe, expect, it } from 'vitest'

import type { AccessMenuNode } from '@src/api/access.contract'
import { YesNo } from '@src/enums/yes-no'
import { resolveBreadcrumbs } from '@src/layout/breadcrumbs'

describe('resolveBreadcrumbs', () => {
	it('returns the fixed Dashboard breadcrumb', () => {
    expect(resolveBreadcrumbs('/dashboard', [])).toEqual([
		{ path: '/dashboard', i18nKey: 'navigation.dashboard' },
    ])
  })

  it('returns directory to leaf order without inventing a directory path', () => {
    expect(resolveBreadcrumbs('/system/users', [systemDirectory()])).toEqual([
		{ path: null, i18nKey: 'navigation.system' },
		{ path: '/system/users', i18nKey: 'navigation.systemUsers' },
    ])
  })

  it('resolves nested directories without changing the input tree', () => {
    const tree = [nestedDirectory()]
    const before = JSON.stringify(tree)

    expect(resolveBreadcrumbs('/system/security/sessions', tree)).toEqual([
		{ path: null, i18nKey: 'navigation.system' },
		{ path: null, i18nKey: 'navigation.systemAuthPlatforms' },
		{ path: '/system/security/sessions', i18nKey: 'navigation.systemSessions' },
    ])
    expect(JSON.stringify(tree)).toBe(before)
	})

	it('returns the fixed menu management breadcrumb without an access node', () => {
		expect(resolveBreadcrumbs('/system/menus', [])).toEqual([
			{ path: '/system/menus', i18nKey: 'navigation.systemMenus' },
		])
	})

	it('keeps hidden pages in the breadcrumb source tree', () => {
		const root = systemDirectory()
		root.children[0].isHidden = YesNo.Yes
		expect(resolveBreadcrumbs('/system/users', [root])).toEqual([
			{ path: null, i18nKey: 'navigation.system' },
			{ path: '/system/users', i18nKey: 'navigation.systemUsers' },
		])
	})

  it('returns null for an authenticated path missing from the access tree', () => {
    expect(resolveBreadcrumbs('/system/missing', [systemDirectory()])).toBeNull()
  })
})

function systemDirectory(): AccessMenuNode {
  return {
    code: 'system',
    menuType: 'directory',
    path: null,
		componentPath: null,
		i18nKey: 'navigation.system',
    icon: 'Folder',
		isHidden: YesNo.No,
    children: [{
      code: 'system:user:list',
      menuType: 'page',
      path: '/system/users',
		componentPath: 'system/users',
		i18nKey: 'navigation.systemUsers',
      icon: 'User',
		isHidden: YesNo.No,
      children: [],
    }],
  }
}

function nestedDirectory(): AccessMenuNode {
  return {
    code: 'system',
    menuType: 'directory',
    path: null,
		componentPath: null,
		i18nKey: 'navigation.system',
    icon: 'Folder',
		isHidden: YesNo.No,
    children: [{
      code: 'system:security',
      menuType: 'directory',
      path: null,
		componentPath: null,
		i18nKey: 'navigation.systemAuthPlatforms',
      icon: 'Key',
		isHidden: YesNo.No,
      children: [{
        code: 'system:security:sessions',
        menuType: 'page',
        path: '/system/security/sessions',
			componentPath: 'system/sessions',
			i18nKey: 'navigation.systemSessions',
        icon: 'List',
			isHidden: YesNo.No,
        children: [],
      }],
    }],
  }
}
