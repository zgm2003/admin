import { describe, expect, it } from 'vitest'

import { YesNo, type YesNo as YesNoValue } from '@src/enums/yes-no'
import { ProtocolError } from '@src/types/http'
import {
  parseManagedMenus,
  parseMenuIDResult,
  parseMenuStatusResult,
} from '@src/api/menu.contract'

describe('menu management contract', () => {
  it('parses the complete directory/page/action tree', () => {
    const value: unknown = validTree()
    expect(parseManagedMenus(value)).toEqual(value)
  })

  it.each([
    null,
    {},
    [null],
    [{ ...directoryNode(), children: null }],
    [{ ...directoryNode(), extra: true }],
		[withoutKey(directoryNode(), 'name')],
		[withoutKey(directoryNode(), 'isProtected')],
    [withoutKey(directoryNode(), 'updatedAt')],
  ])('rejects malformed roots or closed records: %j', (value: unknown) => {
    expect(() => parseManagedMenus(value)).toThrow(ProtocolError)
  })

  it.each([
    { field: 'id', value: 0 },
    { field: 'id', value: 1.5 },
    { field: 'parentId', value: 0 },
    { field: 'menuType', value: 'unknown' },
		{ field: 'name', value: '' },
		{ field: 'name', value: ' 报表' },
    { field: 'code', value: '' },
    { field: 'code', value: ' reports' },
		{ field: 'i18nKey', value: 'navigation_system' },
		{ field: 'icon', value: ' Unknown ' },
    { field: 'sortOrder', value: -1 },
    { field: 'sortOrder', value: 1.5 },
    { field: 'isEnabled', value: 2 },
		{ field: 'isHidden', value: 2 },
		{ field: 'isProtected', value: 2 },
    { field: 'createdAt', value: 'not-a-time' },
    { field: 'updatedAt', value: '' },
  ])('rejects invalid scalar $field=$value', ({ field, value }) => {
    expect(() => parseManagedMenus([{ ...directoryNode(), [field]: value }])).toThrow(ProtocolError)
  })

	it('rejects illegal nullability and node render shapes', () => {
		const root = directoryNode()
		expect(() => parseManagedMenus([{ ...root, path: '/system' }])).toThrow(ProtocolError)
		expect(() => parseManagedMenus([{ ...root, componentPath: 'system' }])).toThrow(ProtocolError)
		expect(() => parseManagedMenus([{ ...pageNode(), parentId: null }])).toThrow(ProtocolError)
		expect(() => parseManagedMenus([{ ...pageNode(), path: null }])).toThrow(ProtocolError)
		expect(() => parseManagedMenus([{ ...pageNode(), componentPath: '/account/users' }])).toThrow(ProtocolError)
		expect(() => parseManagedMenus([{ ...root, i18nKey: null }])).toThrow(ProtocolError)
		expect(() => parseManagedMenus([{ ...pageNode(), i18nKey: null }])).toThrow(ProtocolError)

    const tree = validTree()
    const directory = tree[0]
    const page = directory.children[0]
    const action = page.children[0]
    expect(() => parseManagedMenus([{ ...directory, children: [{ ...page, children: [{ ...action, icon: 'Key' }] }] }])).toThrow(ProtocolError)
    expect(() => parseManagedMenus([{ ...directory, children: [{ ...page, children: [{ ...action, path: '/action' }] }] }])).toThrow(ProtocolError)
		expect(() => parseManagedMenus([{ ...directory, children: [{ ...page, children: [{ ...action, i18nKey: 'permission.menuCreate' }] }] }])).toThrow(ProtocolError)
  })

  it('rejects invalid root, parent links, parent types, and action children', () => {
    const tree = validTree()
    const root = tree[0]
    const page = root.children[0]
    const action = page.children[0]

    expect(() => parseManagedMenus([{ ...root, parentId: 99 }])).toThrow(ProtocolError)
		expect(() => parseManagedMenus([{ ...root, menuType: 'page', path: '/system', componentPath: 'system' }])).toThrow(ProtocolError)
    expect(() => parseManagedMenus([{ ...root, children: [{ ...page, parentId: 99 }] }])).toThrow(ProtocolError)
    expect(() => parseManagedMenus([{ ...root, children: [{ ...action, parentId: root.id }] }])).toThrow(ProtocolError)
    expect(() => parseManagedMenus([{ ...root, children: [{ ...page, children: [{ ...action, children: [action] }] }] }])).toThrow(ProtocolError)
  })

	it('rejects duplicate IDs, codes, and active page paths across the tree', () => {
    const first = directoryNode()
    const second = { ...directoryNode(), id: 10, code: 'reports' }
    expect(() => parseManagedMenus([first, { ...second, id: first.id }])).toThrow(ProtocolError)
    expect(() => parseManagedMenus([first, { ...second, code: first.code }])).toThrow(ProtocolError)

    const tree = validTree()
    const duplicatePathPage = { ...pageNode(), id: 20, code: 'reports:list', parentId: second.id }
    expect(() => parseManagedMenus([tree[0], { ...second, children: [duplicatePathPage] }])).toThrow(ProtocolError)
	})

	it('allows pages to share a component path', () => {
		const firstRoot = { ...directoryNode(), id: 1, code: 'system', children: [pageNode()] }
		const secondPage = { ...pageNode(), id: 20, parentId: 10, code: 'reports:list', path: '/reports' }
		const secondRoot = { ...directoryNode(), id: 10, code: 'reports', children: [secondPage] }
		expect(parseManagedMenus([secondRoot, firstRoot])).toHaveLength(2)
	})

	it('rejects unexpected menu protocol fields', () => {
		expect(() => parseManagedMenus([{ ...directoryNode(), unexpected: null }])).toThrow(ProtocolError)
	})

  it('rejects siblings not sorted by sortOrder, code, and id', () => {
    const first = { ...directoryNode(), id: 1, code: 'zeta', sortOrder: 20 }
    const second = { ...directoryNode(), id: 2, code: 'alpha', sortOrder: 10 }
    expect(() => parseManagedMenus([first, second])).toThrow(ProtocolError)

    const sameSortA = { ...directoryNode(), id: 3, code: 'zeta', sortOrder: 10 }
    const sameSortB = { ...directoryNode(), id: 4, code: 'alpha', sortOrder: 10 }
    expect(() => parseManagedMenus([sameSortA, sameSortB])).toThrow(ProtocolError)
  })

  it('parses only exact mutation result records', () => {
    expect(parseMenuIDResult({ id: 7 })).toEqual({ id: 7 })
    expect(parseMenuStatusResult({ id: 7, isEnabled: 0 })).toEqual({ id: 7, isEnabled: 0 })
    for (const value of [{}, { id: 0 }, { id: 7, extra: true }, { id: '7' }]) {
      expect(() => parseMenuIDResult(value)).toThrow(ProtocolError)
    }
    for (const value of [{ id: 7 }, { id: 7, isEnabled: 2 }, { id: 7, isEnabled: 1, extra: true }]) {
      expect(() => parseMenuStatusResult(value)).toThrow(ProtocolError)
    }
  })
})

const timestamp = '2026-08-19T02:00:00Z'

interface MenuFixture {
  id: number
  parentId: number | null
  menuType: string
	name: string
  code: string
	i18nKey: string | null
  path: string | null
	componentPath: string | null
  icon: string | null
  sortOrder: number
  isEnabled: number
	isHidden: number
	isProtected: YesNoValue
  createdAt: string
  updatedAt: string
  children: MenuFixture[]
}

function directoryNode(): MenuFixture {
  return {
    id: 1,
    parentId: null,
    menuType: 'directory',
		name: '系统管理',
    code: 'system',
    i18nKey: 'navigation.system',
    path: null,
		componentPath: null,
    icon: 'Setting',
    sortOrder: 100,
    isEnabled: 1,
		isHidden: 0,
		isProtected: YesNo.No,
    createdAt: timestamp,
    updatedAt: timestamp,
    children: [],
  }
}

function pageNode(): MenuFixture {
  return {
    id: 2,
    parentId: 1,
    menuType: 'page',
		name: '菜单管理',
    code: 'rbac:menu:list',
    i18nKey: 'navigation.accessMenus',
		path: '/account/users',
		componentPath: 'account/users',
    icon: 'Menu',
    sortOrder: 10,
    isEnabled: 1,
		isHidden: 0,
		isProtected: YesNo.Yes,
    createdAt: timestamp,
    updatedAt: timestamp,
    children: [],
  }
}

function validTree(): MenuFixture[] {
  const action: MenuFixture = {
    id: 3,
    parentId: 2,
    menuType: 'action',
		name: '新增菜单',
    code: 'rbac:menu:create',
		i18nKey: null,
    path: null,
		componentPath: null,
    icon: null,
    sortOrder: 10,
    isEnabled: 1,
		isHidden: 1,
		isProtected: YesNo.Yes,
    createdAt: timestamp,
    updatedAt: timestamp,
    children: [],
  }
  const page = { ...pageNode(), children: [action] }
  return [{ ...directoryNode(), children: [page] }]
}

function withoutKey(value: MenuFixture, key: keyof MenuFixture): Record<string, unknown> {
  return Object.fromEntries(Object.entries(value).filter(([entryKey]) => entryKey !== key))
}
