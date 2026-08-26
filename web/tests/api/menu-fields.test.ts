import { describe, expect, it } from 'vitest'

import {
  isComponentPath,
  isMenuI18nKey,
  isMenuIcon,
  isMenuPath,
	menuCodePattern,
} from '@src/api/menu-fields'

describe('menu field protocol', () => {
  it.each(['navigation.accountUsers', 'reports.orders.list', 'permission.roleUpdate'])(
    'accepts i18n key %s',
    (value) => expect(isMenuI18nKey(value)).toBe(true),
  )

  it.each(['navigation', 'Navigation.users', 'navigation.system_users', ' navigation.accountUsers', 'navigation.accountUsers '])(
    'rejects i18n key %s',
    (value) => expect(isMenuI18nKey(value)).toBe(false),
  )

  it.each(['system', 'account:user:list', 'reports:order-items:list'])(
    'accepts menu code %s',
		(value) => expect(menuCodePattern.test(value)).toBe(true),
  )

	it.each(['/account/users', '/access/menus', '/reports/order-items'])(
    'accepts route path %s',
    (value) => expect(isMenuPath(value)).toBe(true),
  )

	it.each(['/login', '/register', '/dashboard', 'account/users', '/account/users/', '/system/:id', '/system//users', '/account/users?tab=1', '/account/users#top'])(
    'rejects route path %s',
    (value) => expect(isMenuPath(value)).toBe(false),
  )

  it.each(['account/users', 'reports/order-items'])(
    'accepts component path %s',
    (value) => expect(isComponentPath(value)).toBe(true),
  )

  it.each(['/account/users', 'account/users.vue', 'account/users/', 'system/:id', 'system/../users', 'system//users'])(
    'rejects component path %s',
    (value) => expect(isComponentPath(value)).toBe(false),
  )

  it('validates icon names without a component whitelist', () => {
    expect(isMenuIcon('Setting')).toBe(true)
    expect(isMenuIcon('mdi:shield')).toBe(true)
    expect(isMenuIcon('')).toBe(false)
    expect(isMenuIcon(' Setting ')).toBe(false)
    expect(isMenuIcon('x'.repeat(129))).toBe(false)
  })
})
