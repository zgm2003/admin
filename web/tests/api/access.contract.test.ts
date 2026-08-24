import { describe, expect, it, vi } from 'vitest'

import { ProtocolError } from '@src/types/http'
import { parseAccessSnapshot } from '@src/api/access.contract'

vi.mock('@src/access/route-views', () => ({
  routeViews: {
		'system-menus': async () => ({ default: {} }),
  },
}))

describe('access contract', () => {
  it('parses an exact directory-only snapshot', () => {
    const value: unknown = validDirectorySnapshot()
    expect(parseAccessSnapshot(value)).toEqual(value)
  })

  it.each([
    { roleCodes: null, menuTree: [], permissionCodes: [] },
    { roleCodes: [], menuTree: null, permissionCodes: [] },
    { roleCodes: [], menuTree: [], permissionCodes: null },
    { roleCodes: [], menuTree: [] },
    { roleCodes: [], menuTree: [], permissionCodes: [], extra: true },
  ])('rejects invalid snapshot fields: %j', (value: unknown) => {
    expect(() => parseAccessSnapshot(value)).toThrow(ProtocolError)
  })

  it.each([
    { roleCodes: ['registered_user', 'registered_user'] },
    { roleCodes: ['registered_user', 'ai_tester'] },
    { roleCodes: ['', 'registered_user'] },
  ])('rejects invalid role code arrays: $roleCodes', ({ roleCodes }: { roleCodes: unknown }) => {
    expect(() => parseAccessSnapshot({ roleCodes, menuTree: [], permissionCodes: [] })).toThrow(ProtocolError)
  })

  it.each([
    { permissionCodes: ['system:user:view', 'system:user:view'] },
    { permissionCodes: ['system:user:view', 'system:user:create'] },
    { permissionCodes: [''] },
  ])('rejects invalid permission code arrays: $permissionCodes', ({ permissionCodes }: { permissionCodes: unknown }) => {
    expect(() => parseAccessSnapshot({ roleCodes: [], menuTree: [], permissionCodes })).toThrow(ProtocolError)
  })

  it.each([
		{ code: 'system', menuType: 'directory', path: null, viewKey: null, titleKey: 'navigation.system', icon: null },
		{ code: 'system', menuType: 'directory', path: null, viewKey: null, titleKey: 'navigation.system', icon: null, children: [], extra: true },
		{ code: 'system', menuType: 'unknown', path: null, viewKey: null, titleKey: 'navigation.system', icon: null, children: [] },
		{ code: 'system:menu:create', menuType: 'action', path: null, viewKey: null, titleKey: 'permission.menuCreate', icon: null, children: [] },
		{ code: 'system:menu:list', menuType: 'page', path: null, viewKey: 'system-menus', titleKey: 'navigation.systemMenus', icon: null, children: [] },
		{ code: 'system:menu:list', menuType: 'page', path: '/system/menus', viewKey: null, titleKey: 'navigation.systemMenus', icon: null, children: [] },
		{ code: 'system', menuType: 'directory', path: null, viewKey: 'system-menus', titleKey: 'navigation.system', icon: null, children: [] },
		{ code: 'system', menuType: 'directory', path: '/system', viewKey: null, titleKey: 'navigation.system', icon: null, children: [] },
		{ code: 'system', menuType: 'directory', path: null, viewKey: null, titleKey: 'navigation.dashboard', icon: null, children: [] },
		{ code: 'system', menuType: 'directory', path: null, viewKey: null, titleKey: 'navigation.system', icon: 'Unknown', children: [] },
		{ code: 'system:menu:list', menuType: 'page', path: '/system/menus', viewKey: 'unknownView', titleKey: 'navigation.systemMenus', icon: null, children: [] },
  ])('rejects an invalid menu node: %j', (node: unknown) => {
    expect(() => parseAccessSnapshot({ roleCodes: [], menuTree: [node], permissionCodes: [] })).toThrow(ProtocolError)
  })

  it('rejects duplicate menu codes across the tree', () => {
		const child = validPageNode('system:menu:list', '/system/menus')
    const value: unknown = {
      roleCodes: [],
      menuTree: [validDirectoryNode('system', [child]), validDirectoryNode('system', [])],
      permissionCodes: [],
    }
    expect(() => parseAccessSnapshot(value)).toThrow(ProtocolError)
  })

  it('rejects duplicate page paths and page children', () => {
		const first = validPageNode('system:menu:list', '/system/menus')
		const second = validPageNode('system:other:list', '/system/menus')
    expect(() => parseAccessSnapshot({
      roleCodes: [],
      menuTree: [validDirectoryNode('system', [first, second])],
      permissionCodes: [],
    })).toThrow(ProtocolError)

    expect(() => parseAccessSnapshot({
      roleCodes: [],
      menuTree: [{ ...first, children: [validDirectoryNode('nested', [])] }],
      permissionCodes: [],
    })).toThrow(ProtocolError)
  })
})

function validDirectorySnapshot(): unknown {
  return {
    roleCodes: ['registered_user'],
    menuTree: [validDirectoryNode('system', [])],
    permissionCodes: [],
  }
}

interface MenuFixture {
  code: string
  menuType: string
  path: string | null
  viewKey: string | null
  titleKey: string
  icon: string | null
  children: unknown[]
}

function validDirectoryNode(code: string, children: unknown[]): MenuFixture {
  return {
    code,
    menuType: 'directory',
    path: null,
    viewKey: null,
		titleKey: 'navigation.system',
    icon: 'Folder',
    children,
  }
}

function validPageNode(code: string, path: string): MenuFixture {
  return {
    code,
    menuType: 'page',
    path,
		viewKey: 'system-menus',
		titleKey: 'navigation.systemMenus',
    icon: 'User',
    children: [],
  }
}
