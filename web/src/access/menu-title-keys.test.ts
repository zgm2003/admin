import { describe, expect, it } from 'vitest'

import { isMenuTitleKey, menuTitleKeys } from './menu-title-keys'

describe('menu title key protocol', () => {
  it('contains exactly the registered core menu title keys', () => {
    expect(menuTitleKeys).toEqual([
      'navigation.system',
      'navigation.systemMenus',
      'permission.menuCreate',
      'permission.menuUpdate',
      'permission.menuDelete',
      'navigation.systemRoles',
      'permission.roleCreate',
      'permission.roleUpdate',
      'permission.roleStatus',
      'permission.roleSetDefault',
      'permission.roleDelete',
      'permission.roleAuthorize',
			'navigation.systemUsers',
			'permission.userUpdate',
			'permission.userStatus',
			'permission.userDelete',
			'permission.userRoles',
      'navigation.systemAuthPlatforms',
      'permission.authPlatformCreate',
      'permission.authPlatformUpdate',
      'permission.authPlatformStatus',
      'permission.authPlatformDelete',
    ])
  })

  it('rejects application messages that are not registered menu titles', () => {
    expect(isMenuTitleKey('navigation.system')).toBe(true)
    expect(isMenuTitleKey('navigation.dashboard')).toBe(false)
    expect(isMenuTitleKey('menu.title')).toBe(false)
    expect(isMenuTitleKey('')).toBe(false)
  })
})
