import { describe, expect, it } from 'vitest'

import {
  closeAllRouteTabs,
  closeOtherRouteTabs,
  closeRouteTab,
} from '@/layout/components/RouteTabs/route-tabs'
import type { RouteTab } from '@/layout/components/RouteTabs/route-tabs'

const dashboardTab: RouteTab = {
  path: '/dashboard',
  i18nKey: 'navigation.dashboard',
  affix: true,
}
const usersTab: RouteTab = {
  path: '/account/users',
  i18nKey: 'navigation.main',
  affix: false,
}
const rolesTab: RouteTab = {
  path: '/access/roles',
  i18nKey: 'reports.orders.list',
  affix: false,
}

describe('route tab operations', () => {
  it('closes the active tab and chooses the nearest remaining tab', () => {
    expect(closeRouteTab([dashboardTab, usersTab, rolesTab], usersTab.path, usersTab.path)).toEqual(
      {
        tabs: [dashboardTab, rolesTab],
        nextPath: dashboardTab.path,
      },
    )
  })

  it('does not close an affixed tab', () => {
    expect(closeRouteTab([dashboardTab, usersTab], dashboardTab.path, dashboardTab.path)).toEqual({
      tabs: [dashboardTab, usersTab],
    })
  })

  it('keeps affixed tabs while closing other tabs and all tabs', () => {
    expect(
      closeOtherRouteTabs([dashboardTab, usersTab, rolesTab], rolesTab.path, usersTab.path),
    ).toEqual({
      tabs: [dashboardTab, rolesTab],
      nextPath: rolesTab.path,
    })
    expect(closeAllRouteTabs([dashboardTab, usersTab, rolesTab])).toEqual({
      tabs: [dashboardTab],
      nextPath: dashboardTab.path,
    })
  })
})
