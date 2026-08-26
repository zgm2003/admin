import { describe, expect, it } from 'vitest'

import { ProtocolError } from '@src/types/http'
import { parseAccessSnapshot } from '@src/api/access.contract'

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
		{ permissionCodes: ['account:user:list', 'account:user:list'] },
		{ permissionCodes: ['account:user:list', 'account:user:create'] },
    { permissionCodes: [''] },
  ])('rejects invalid permission code arrays: $permissionCodes', ({ permissionCodes }: { permissionCodes: unknown }) => {
    expect(() => parseAccessSnapshot({ roleCodes: [], menuTree: [], permissionCodes })).toThrow(ProtocolError)
  })

	it.each([
		{ code: 'system', menuType: 'directory', path: null, componentPath: null, i18nKey: 'navigation.system', icon: null, isHidden: 0 },
		{ code: 'system', menuType: 'directory', path: null, componentPath: null, i18nKey: 'navigation.system', icon: null, isHidden: 0, children: [], extra: true },
		{ code: 'system', menuType: 'unknown', path: null, componentPath: null, i18nKey: 'navigation.system', icon: null, isHidden: 0, children: [] },
		{ code: 'rbac:menu:create', menuType: 'action', path: null, componentPath: null, i18nKey: 'permission.menuCreate', icon: null, isHidden: 1, children: [] },
		{ code: 'account:user:list', menuType: 'page', path: null, componentPath: 'account/users', i18nKey: 'navigation.accountUsers', icon: null, isHidden: 0, children: [] },
		{ code: 'account:user:list', menuType: 'page', path: '/account/users', componentPath: null, i18nKey: 'navigation.accountUsers', icon: null, isHidden: 0, children: [] },
		{ code: 'system', menuType: 'directory', path: null, componentPath: 'system', i18nKey: 'navigation.system', icon: null, isHidden: 0, children: [] },
		{ code: 'system', menuType: 'directory', path: '/system', componentPath: null, i18nKey: 'navigation.system', icon: null, isHidden: 0, children: [] },
		{ code: 'system', menuType: 'directory', path: null, componentPath: null, i18nKey: 'navigation_system', icon: null, isHidden: 0, children: [] },
		{ code: 'system', menuType: 'directory', path: null, componentPath: null, i18nKey: 'navigation.system', icon: ' Unknown ', isHidden: 0, children: [] },
		{ code: 'account:user:list', menuType: 'page', path: '/account/users', componentPath: '/account/users', i18nKey: 'navigation.accountUsers', icon: null, isHidden: 0, children: [] },
		{ code: 'system', menuType: 'directory', path: null, componentPath: null, i18nKey: 'navigation.system', icon: null, isHidden: 2, children: [] },
	])('rejects an invalid menu node: %j', (node: unknown) => {
    expect(() => parseAccessSnapshot({ roleCodes: [], menuTree: [node], permissionCodes: [] })).toThrow(ProtocolError)
  })

  it('rejects duplicate menu codes across the tree', () => {
		const child = validPageNode('account:user:list', '/account/users')
    const value: unknown = {
      roleCodes: [],
      menuTree: [validDirectoryNode('system', [child]), validDirectoryNode('system', [])],
      permissionCodes: [],
    }
    expect(() => parseAccessSnapshot(value)).toThrow(ProtocolError)
  })

  it('rejects duplicate page paths and page children', () => {
		const first = validPageNode('account:user:list', '/account/users')
		const second = validPageNode('system:other:list', '/account/users')
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

	it('allows custom i18n keys, arbitrary valid icons, hidden pages, and shared component paths', () => {
		const first = { ...validPageNode('reports:orders:list', '/reports/orders'), i18nKey: 'reports.orders.list', icon: 'lucide:shield-check', isHidden: 1 }
		const second = { ...validPageNode('reports:archive:list', '/reports/archive'), componentPath: first.componentPath }
		const snapshot = parseAccessSnapshot({
			roleCodes: [],
			menuTree: [validDirectoryNode('reports', [first, second])],
			permissionCodes: [],
		})
		expect(snapshot.menuTree[0].children).toHaveLength(2)
	})

	it('rejects unexpected menu node fields', () => {
		expect(() => parseAccessSnapshot({
			roleCodes: [], menuTree: [{ ...validDirectoryNode('system', []), unexpected: true }], permissionCodes: [],
		})).toThrow(ProtocolError)
	})
})

function validDirectorySnapshot(): unknown {
  return {
    roleCodes: ['registered_user'],
		menuTree: [
			validDirectoryNode('account', [
				validPageNode('account:user:list', '/account/users', 'account/users', 'navigation.accountUsers'),
			]),
			validDirectoryNode('access', [
				validPageNode('rbac:menu:list', '/access/menus', 'access/menus', 'navigation.accessMenus'),
			]),
			validDirectoryNode('system', [
				validPageNode('audit:operation-log:list', '/system/operation-logs', 'system/operation-logs', 'navigation.systemOperationLogs'),
			]),
		],
    permissionCodes: [],
  }
}

interface MenuFixture {
  code: string
  menuType: string
  path: string | null
	componentPath: string | null
	i18nKey: string
  icon: string | null
	isHidden: number
  children: unknown[]
}

function validDirectoryNode(code: string, children: unknown[]): MenuFixture {
  return {
    code,
    menuType: 'directory',
    path: null,
		componentPath: null,
		i18nKey: `navigation.${code}`,
    icon: 'lucide:folder',
		isHidden: 0,
    children,
  }
}

function validPageNode(
	code: string,
	path: string,
	componentPath = 'account/users',
	i18nKey = 'navigation.accountUsers',
): MenuFixture {
  return {
    code,
    menuType: 'page',
    path,
		componentPath,
		i18nKey,
    icon: 'lucide:user-round',
		isHidden: 0,
    children: [],
  }
}
