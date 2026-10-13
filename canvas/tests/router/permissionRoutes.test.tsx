import { describe, expect, it } from 'vitest'
import { getPermissionRoutes } from '@/router/permissionRoutes'
import type { PermissionState } from '@/store/permission'

const state: PermissionState = { status: 'ready', error: '', roleCodes: [], permissionCodes: ['user:profile:view'], menuTree: [{ code: 'user:profile:view', menuType: 'page', path: '/user/profile', componentPath: 'user/profile', i18nKey: 'layout.user.profile', icon: null, isHidden: 1, children: [] }] }
describe('Canvas permission routes', () => {
  it('registers the directly granted hidden personal page', () => { expect(getPermissionRoutes(state).map(route => route.path)).toEqual(['/user/profile']) })
  it('never infers a page from actions or a view code without a menu node', () => {
    expect(getPermissionRoutes({ ...state, menuTree: [], permissionCodes: ['user:profile:update'] })).toEqual([])
    expect(getPermissionRoutes({ ...state, menuTree: [] })).toEqual([])
    expect(getPermissionRoutes({ ...state, permissionCodes: ['user:profile:update'] })).toEqual([])
  })
  it('removes pages on reset, reload or failure and rejects unknown component paths', () => {
    for (const status of ['idle', 'loading', 'error'] as const) expect(getPermissionRoutes({ ...state, status })).toEqual([])
    expect(getPermissionRoutes({ ...state, menuTree: [{ ...state.menuTree[0], componentPath: 'canvas/test', path: '/canvas/test' }] })).toEqual([])
  })
})
