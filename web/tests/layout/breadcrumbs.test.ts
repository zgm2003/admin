import { describe, expect, it } from 'vitest'

import type { AccessMenuNode } from '@src/api/access.contract'
import { resolveBreadcrumbs } from '@src/layout/breadcrumbs'

describe('resolveBreadcrumbs', () => {
  it('returns the fixed Dashboard breadcrumb', () => {
    expect(resolveBreadcrumbs('/dashboard', [])).toEqual([
      { path: '/dashboard', titleKey: 'navigation.dashboard' },
    ])
  })

  it('returns directory to leaf order without inventing a directory path', () => {
    expect(resolveBreadcrumbs('/system/users', [systemDirectory()])).toEqual([
      { path: null, titleKey: 'navigation.system' },
      { path: '/system/users', titleKey: 'navigation.systemUsers' },
    ])
  })

  it('resolves nested directories without changing the input tree', () => {
    const tree = [nestedDirectory()]
    const before = JSON.stringify(tree)

    expect(resolveBreadcrumbs('/system/security/sessions', tree)).toEqual([
      { path: null, titleKey: 'navigation.system' },
      { path: null, titleKey: 'navigation.systemAuthPlatforms' },
      { path: '/system/security/sessions', titleKey: 'navigation.systemSessions' },
    ])
    expect(JSON.stringify(tree)).toBe(before)
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
    viewKey: null,
    titleKey: 'navigation.system',
    icon: 'Folder',
    children: [{
      code: 'system:user:list',
      menuType: 'page',
      path: '/system/users',
      viewKey: 'system-users',
      titleKey: 'navigation.systemUsers',
      icon: 'User',
      children: [],
    }],
  }
}

function nestedDirectory(): AccessMenuNode {
  return {
    code: 'system',
    menuType: 'directory',
    path: null,
    viewKey: null,
    titleKey: 'navigation.system',
    icon: 'Folder',
    children: [{
      code: 'system:security',
      menuType: 'directory',
      path: null,
      viewKey: null,
      titleKey: 'navigation.systemAuthPlatforms',
      icon: 'Key',
      children: [{
        code: 'system:security:sessions',
        menuType: 'page',
        path: '/system/security/sessions',
        viewKey: 'system-sessions',
        titleKey: 'navigation.systemSessions',
        icon: 'List',
        children: [],
      }],
    }],
  }
}
