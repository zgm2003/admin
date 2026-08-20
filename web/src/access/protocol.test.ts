import { describe, expect, it } from 'vitest'

import { hasRouteViewKey, isMenuIconKey } from './protocol'

describe('access protocol registries', () => {
  it('accepts only explicitly registered menu icons', () => {
    expect(isMenuIconKey('Folder')).toBe(true)
    expect(isMenuIconKey('User')).toBe(true)
    expect(isMenuIconKey('Unknown')).toBe(false)
  })

	it('registers the menu-management view explicitly', () => {
		expect(hasRouteViewKey('system-menus')).toBe(true)
		expect(hasRouteViewKey('system-users')).toBe(true)
		expect(hasRouteViewKey('systemUsers')).toBe(false)
    expect(hasRouteViewKey('')).toBe(false)
  })
})
