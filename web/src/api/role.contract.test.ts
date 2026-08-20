import { describe, expect, it } from 'vitest'

import { YesNo } from '../enums/yes-no'
import { ProtocolError } from '../types/http'
import {
  parseEmptyResult,
  parseRoleDefaultResult,
  parseRoleIDResult,
  parseRolePage,
  parseRolePermissionResult,
  parseRolePermissions,
  parseRoleStatusResult,
} from './role.contract'

describe('role contract', () => {
  it('parses the exact paged role response', () => {
    const result = parseRolePage(rolePage())

    expect(result).toEqual(rolePage())
    expect(result.list).toHaveLength(1)
  })

  it('rejects malformed role pages and list items', () => {
    const valid = rolePage()
    const validItem = valid.list[0]
    const invalidValues: unknown[] = [
      null,
      [],
      { ...valid, list: null },
      { ...valid, extra: true },
      { ...valid, page: 0 },
      { ...valid, pageSize: 1.5 },
      { ...valid, total: -1 },
      { ...valid, list: [{ ...validItem, id: 0 }] },
      { ...valid, list: [{ ...validItem, id: Number.MAX_SAFE_INTEGER + 1 }] },
      { ...valid, list: [{ ...validItem, isEnabled: 2 }] },
      { ...valid, list: [{ ...validItem, userCount: -1 }] },
      { ...valid, list: [{ ...validItem, permissionCount: 0.5 }] },
      { ...valid, list: [{ ...validItem, createdAt: 'not-a-time' }] },
      { ...valid, list: [{ ...validItem }, { ...validItem }] },
      { ...valid, list: [{ ...validItem }, { ...validItem, id: 2 }] },
      { ...valid, list: [{ ...validItem, unexpected: true }] },
    ]

    for (const value of invalidValues) {
      expect(() => parseRolePage(value)).toThrow(ProtocolError)
    }
  })

  it('parses only exact mutation result records', () => {
    expect(parseRoleIDResult({ id: 7 })).toEqual({ id: 7 })
    expect(parseEmptyResult({})).toEqual({})
    expect(parseRoleStatusResult({ id: 7, isEnabled: YesNo.No })).toEqual({
      id: 7,
      isEnabled: YesNo.No,
    })
    expect(parseRoleDefaultResult({ id: 7, isDefault: YesNo.Yes })).toEqual({
      id: 7,
      isDefault: YesNo.Yes,
    })
    expect(parseRolePermissionResult({ id: 7, permissionCount: 2 })).toEqual({
      id: 7,
      permissionCount: 2,
    })

    const invalidCases: Array<() => unknown> = [
      () => parseRoleIDResult({ id: -1 }),
      () => parseEmptyResult(null),
      () => parseEmptyResult([]),
      () => parseEmptyResult({ id: 7 }),
      () => parseRoleStatusResult({ id: 7, isEnabled: 2 }),
      () => parseRoleDefaultResult({ id: 7, isDefault: 0, extra: true }),
      () => parseRolePermissionResult({ id: 7, permissionCount: -1 }),
    ]
    for (const parse of invalidCases) {
      expect(parse).toThrow(ProtocolError)
    }
  })

  it('parses a complete permission tree with disabled nodes and sorted direct IDs', () => {
    const value = rolePermissions()

    expect(parseRolePermissions(value)).toEqual(value)
  })

  it('rejects corrupt permission tree shapes and protocols', () => {
    const value = rolePermissions()
    const directory = value.menuTree[0]
    const page = directory.children[0]
    const action = page.children[0]
    const invalidTrees: unknown[] = [
      { ...value, menuTree: null },
      { ...value, role: { ...value.role, isEnabled: 2 } },
      { ...value, role: { ...value.role, extra: true } },
      { ...value, menuTree: [{ ...directory, id: 0 }] },
      { ...value, menuTree: [{ ...directory, menuType: 'page' }] },
      { ...value, menuTree: [{ ...directory, i18nKey: 'role.title' }] },
      { ...value, menuTree: [{ ...directory, isEnabled: 2 }] },
      { ...value, menuTree: [{ ...directory, children: null }] },
      { ...value, menuTree: [{ ...directory, children: [{ ...page, parentId: 99 }] }] },
      { ...value, menuTree: [{ ...directory, children: [{ ...action, parentId: directory.id }] }] },
      {
        ...value,
        menuTree: [
          {
            ...directory,
            children: [
              {
                ...page,
                children: [{ ...action, children: [{ ...action, id: 4, parentId: action.id }] }],
              },
            ],
          },
        ],
      },
      { ...value, menuTree: [directory, { ...directory, code: 'reports' }] },
      { ...value, menuTree: [directory, { ...directory, id: 4 }] },
    ]

    for (const invalid of invalidTrees) {
      expect(() => parseRolePermissions(invalid)).toThrow(ProtocolError)
    }
  })

  it('rejects invalid direct permission IDs', () => {
    const value = rolePermissions()
    const invalidMenuIDs: unknown[] = [
      null,
      [0],
      [2, 2],
      [3, 2],
      [1],
      [99],
    ]

    for (const menuIds of invalidMenuIDs) {
      expect(() => parseRolePermissions({ ...value, menuIds })).toThrow(ProtocolError)
    }
  })
})

function rolePage() {
  return {
    list: [
      {
        id: 1,
        code: 'tester',
        name: 'Tester',
        isDefault: YesNo.No,
        isEnabled: YesNo.Yes,
        userCount: 0,
        permissionCount: 1,
        createdAt: '2026-08-19T00:00:00Z',
        updatedAt: '2026-08-19T00:00:00.123456Z',
      },
    ],
    total: 1,
    page: 1,
    pageSize: 20,
  }
}

function rolePermissions() {
  return {
    role: {
      id: 2,
      code: 'tester',
      name: 'Tester',
      isDefault: YesNo.No,
      isEnabled: YesNo.Yes,
    },
    menuTree: [
      {
        id: 1,
        parentId: null,
        menuType: 'directory' as const,
        code: 'system',
        i18nKey: 'navigation.system' as const,
        isEnabled: YesNo.Yes,
        children: [
          {
            id: 2,
            parentId: 1,
            menuType: 'page' as const,
            code: 'system:role:list',
            i18nKey: 'navigation.systemRoles' as const,
            isEnabled: YesNo.Yes,
            children: [
              {
                id: 3,
                parentId: 2,
                menuType: 'action' as const,
                code: 'system:role:create',
                i18nKey: 'permission.roleCreate' as const,
                isEnabled: YesNo.No,
                children: [],
              },
            ],
          },
        ],
      },
    ],
    menuIds: [2],
  }
}
