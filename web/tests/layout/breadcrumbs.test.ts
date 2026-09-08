import { describe, expect, it } from 'vitest'

import type { PermissionMenuNode } from '@/api/permission/permission'
import { YesNo } from '@/enums/yes-no'
import { resolveBreadcrumbs } from '@/layout/breadcrumbs'

describe('resolveBreadcrumbs', () => {
  it('returns the fixed Dashboard breadcrumb', () => {
    expect(resolveBreadcrumbs('/dashboard', [])).toEqual([
      { path: '/dashboard', i18nKey: 'navigation.dashboard' },
    ])
  })

  it('returns directory to leaf order without inventing a directory path', () => {
    expect(resolveBreadcrumbs('/user/account', [accountDirectory()])).toEqual([
      { path: null, i18nKey: 'navigation.user' },
      { path: '/user/account', i18nKey: 'navigation.userAccount' },
    ])
  })

  it('resolves nested directories without changing the input tree', () => {
    const tree = [nestedDirectory()]
    const before = JSON.stringify(tree)

    expect(resolveBreadcrumbs('/system/security/sessions', tree)).toEqual([
      { path: null, i18nKey: 'navigation.system' },
      { path: null, i18nKey: 'navigation.permissionAuthplatform' },
      { path: '/system/security/sessions', i18nKey: 'navigation.userSession' },
    ])
    expect(JSON.stringify(tree)).toBe(before)
  })

  it('resolves every business root and does not invent a static menu breadcrumb', () => {
    const tree = [accountDirectory(), accessDirectory(), systemDirectory()]
    expect(resolveBreadcrumbs('/permission/menu', [])).toBeNull()
    expect(resolveBreadcrumbs('/permission/menu', tree)).toEqual([
      { path: null, i18nKey: 'navigation.permission' },
      { path: '/permission/menu', i18nKey: 'navigation.permissionMenu' },
    ])
    expect(resolveBreadcrumbs('/system/operationlog', tree)).toEqual([
      { path: null, i18nKey: 'navigation.system' },
      { path: '/system/operationlog', i18nKey: 'navigation.systemOperationlog' },
    ])
  })

  it('keeps hidden pages in the breadcrumb source tree', () => {
    const root = accountDirectory()
    root.children[0].isHidden = YesNo.Yes
    expect(resolveBreadcrumbs('/user/account', [root])).toEqual([
      { path: null, i18nKey: 'navigation.user' },
      { path: '/user/account', i18nKey: 'navigation.userAccount' },
    ])
  })

  it('resolves the hidden profile breadcrumb from the access tree', () => {
    const root = accountDirectory()
    root.children = [
      pageNode('user:profile:list', '/user/profile', 'user/profile', 'layout.user.profile'),
    ]
    root.children[0].isHidden = YesNo.Yes
    expect(resolveBreadcrumbs('/user/profile', [root])).toEqual([
      { path: null, i18nKey: 'navigation.user' },
      { path: '/user/profile', i18nKey: 'layout.user.profile' },
    ])
  })

  it('returns null for an authenticated path missing from the access tree', () => {
    expect(resolveBreadcrumbs('/system/missing', [systemDirectory()])).toBeNull()
  })
})

function accountDirectory(): PermissionMenuNode {
  return {
    code: 'account',
    menuType: 'directory',
    path: null,
    componentPath: null,
    i18nKey: 'navigation.user',
    icon: 'lucide:folder',
    isHidden: YesNo.No,
    children: [
      {
        code: 'user:account:list',
        menuType: 'page',
        path: '/user/account',
        componentPath: 'user/account',
        i18nKey: 'navigation.userAccount',
        icon: 'User',
        isHidden: YesNo.No,
        children: [],
      },
    ],
  }
}

function accessDirectory(): PermissionMenuNode {
  return directoryNode(
    'access',
    'navigation.permission',
    pageNode(
      'permission:menu:list',
      '/permission/menu',
      'permission/menu',
      'navigation.permissionMenu',
    ),
  )
}

function systemDirectory(): PermissionMenuNode {
  return directoryNode(
    'system',
    'navigation.system',
    pageNode(
      'system:operationlog:list',
      '/system/operationlog',
      'system/operationlog',
      'navigation.systemOperationlog',
    ),
  )
}

function directoryNode(
  code: string,
  i18nKey: string,
  child: PermissionMenuNode,
): PermissionMenuNode {
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

function pageNode(
  code: string,
  path: string,
  componentPath: string,
  i18nKey: string,
): PermissionMenuNode {
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

function nestedDirectory(): PermissionMenuNode {
  return {
    code: 'system',
    menuType: 'directory',
    path: null,
    componentPath: null,
    i18nKey: 'navigation.system',
    icon: 'lucide:folder',
    isHidden: YesNo.No,
    children: [
      {
        code: 'system:security',
        menuType: 'directory',
        path: null,
        componentPath: null,
        i18nKey: 'navigation.permissionAuthplatform',
        icon: 'lucide:key-round',
        isHidden: YesNo.No,
        children: [
          {
            code: 'system:security:sessions',
            menuType: 'page',
            path: '/system/security/sessions',
            componentPath: 'user/session',
            i18nKey: 'navigation.userSession',
            icon: 'lucide:list-tree',
            isHidden: YesNo.No,
            children: [],
          },
        ],
      },
    ],
  }
}
