import { describe, expect, it, vi } from 'vitest'

import { ProtocolError } from '../types/http'
import { parseAccessSnapshot } from './access.contract'

vi.mock('../access/route-views', () => ({
  routeViews: {
    systemUsers: async () => ({ default: {} }),
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
    { code: 'system', menuType: 'directory', path: null, viewKey: null, titleKey: 'navigation.main', icon: null },
    { code: 'system', menuType: 'directory', path: null, viewKey: null, titleKey: 'navigation.main', icon: null, children: [], extra: true },
    { code: 'system', menuType: 'unknown', path: null, viewKey: null, titleKey: 'navigation.main', icon: null, children: [] },
    { code: 'system:user:create', menuType: 'action', path: null, viewKey: null, titleKey: 'navigation.main', icon: null, children: [] },
    { code: 'system:user:view', menuType: 'page', path: null, viewKey: 'systemUsers', titleKey: 'navigation.dashboard', icon: null, children: [] },
    { code: 'system:user:view', menuType: 'page', path: '/system/users', viewKey: null, titleKey: 'navigation.dashboard', icon: null, children: [] },
    { code: 'system', menuType: 'directory', path: null, viewKey: 'systemUsers', titleKey: 'navigation.main', icon: null, children: [] },
    { code: 'system', menuType: 'directory', path: null, viewKey: null, titleKey: 'unknown.title', icon: null, children: [] },
    { code: 'system', menuType: 'directory', path: null, viewKey: null, titleKey: 'navigation.main', icon: 'Unknown', children: [] },
    { code: 'system:user:view', menuType: 'page', path: '/system/users', viewKey: 'unknownView', titleKey: 'navigation.dashboard', icon: null, children: [] },
  ])('rejects an invalid menu node: %j', (node: unknown) => {
    expect(() => parseAccessSnapshot({ roleCodes: [], menuTree: [node], permissionCodes: [] })).toThrow(ProtocolError)
  })

  it('rejects duplicate menu codes across the tree', () => {
    const child = validPageNode('system:user:view', '/system/users')
    const value: unknown = {
      roleCodes: [],
      menuTree: [validDirectoryNode('system', [child]), validDirectoryNode('system', [])],
      permissionCodes: [],
    }
    expect(() => parseAccessSnapshot(value)).toThrow(ProtocolError)
  })

  it('rejects duplicate page paths and page children', () => {
    const first = validPageNode('system:user:view', '/system/users')
    const second = validPageNode('system:team:view', '/system/users')
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
    titleKey: 'navigation.main',
    icon: 'Folder',
    children,
  }
}

function validPageNode(code: string, path: string): MenuFixture {
  return {
    code,
    menuType: 'page',
    path,
    viewKey: 'systemUsers',
    titleKey: 'navigation.dashboard',
    icon: 'User',
    children: [],
  }
}
