import { describe, expect, it } from 'vitest'

import type { RolePermissionTreeNode } from '@src/api/role.contract'
import { YesNo } from '@src/enums/yes-no'
import {
  buildRolePermissionMatrix,
  diffMenuIDs,
  expandDirectMenuIDs,
  getRoleMatrixGroupMenuIDs,
  getRoleMatrixMenuIDs,
  getRoleMatrixSelectionState,
  normalizeDirectMenuIDs,
  toggleMatrixAction,
  toggleMatrixGroup,
  toggleMatrixPage,
} from '@src/views/system/roles/role-permission-matrix'

describe('role permission matrix', () => {
  it('builds one stable root group and keeps nested and disabled permissions', () => {
    const groups = buildRolePermissionMatrix(menuTree())

    expect(groups).toHaveLength(1)
    expect(groups[0]?.groupId).toBe(1)
    expect(groups[0]?.rows.map((row) => row.pageId)).toEqual([2, 5])
    expect(groups[0]?.rows[1]?.actions[0]).toMatchObject({
      id: 6,
      isEnabled: YesNo.No,
    })
  })

  it('expands action grants with their page and preserves direct page grants', () => {
    const groups = buildRolePermissionMatrix(menuTree())

    expect(expandDirectMenuIDs(groups, [3, 5])).toEqual([2, 3, 5])
  })

  it('keeps page and action selection semantically valid', () => {
    const row = buildRolePermissionMatrix(menuTree())[0]?.rows[0]
    expect(row).toBeDefined()
    if (row === undefined) {
      return
    }

    expect(toggleMatrixAction([], row, 3, true)).toEqual([2, 3])
    expect(toggleMatrixAction([2, 3], row, 3, false)).toEqual([2])
    expect(toggleMatrixPage([2, 3], row, false)).toEqual([])
    expect(toggleMatrixPage([], row, true)).toEqual([2])
  })

  it('selects and clears complete groups', () => {
    const group = buildRolePermissionMatrix(menuTree())[0]
    expect(group).toBeDefined()
    if (group === undefined) {
      return
    }

    expect(getRoleMatrixGroupMenuIDs(group)).toEqual([2, 3, 5, 6])
    expect(toggleMatrixGroup([], group, true)).toEqual([2, 3, 5, 6])
    expect(toggleMatrixGroup([2, 3, 5, 6], group, false)).toEqual([])
    expect(getRoleMatrixMenuIDs([group])).toEqual([2, 3, 5, 6])
  })

  it('reports empty, partial, and complete selection states', () => {
    const menuIDs = [2, 3, 5, 6]

    expect(getRoleMatrixSelectionState(menuIDs, [])).toEqual({
      total: 4,
      selected: 0,
      checked: false,
      indeterminate: false,
    })
    expect(getRoleMatrixSelectionState(menuIDs, [2])).toEqual({
      total: 4,
      selected: 1,
      checked: false,
      indeterminate: true,
    })
    expect(getRoleMatrixSelectionState(menuIDs, new Set(menuIDs))).toEqual({
      total: 4,
      selected: 4,
      checked: true,
      indeterminate: false,
    })
  })

  it('normalizes effective permissions to minimal direct grants', () => {
    const groups = buildRolePermissionMatrix(menuTree())

    expect(normalizeDirectMenuIDs(groups, [2, 3])).toEqual([3])
    expect(normalizeDirectMenuIDs(groups, [2])).toEqual([2])
    expect(normalizeDirectMenuIDs(groups, [2, 3, 5, 6])).toEqual([3, 6])
  })

  it('calculates stable added and removed permission ids', () => {
    expect(diffMenuIDs([2, 3], [2, 4])).toEqual({ added: [4], removed: [3] })
  })
})

function menuTree(): RolePermissionTreeNode[] {
  return [
    {
      id: 1,
      parentId: null,
      menuType: 'directory',
      code: 'system',
      i18nKey: 'navigation.system',
      isEnabled: YesNo.Yes,
      children: [
        {
          id: 2,
          parentId: 1,
          menuType: 'page',
          code: 'system:role:list',
          i18nKey: 'navigation.systemRoles',
          isEnabled: YesNo.Yes,
          children: [
            {
              id: 3,
              parentId: 2,
              menuType: 'action',
              code: 'system:role:create',
              i18nKey: 'permission.roleCreate',
              isEnabled: YesNo.Yes,
              children: [],
            },
          ],
        },
        {
          id: 4,
          parentId: 1,
          menuType: 'directory',
          code: 'system:settings',
          i18nKey: 'navigation.system',
          isEnabled: YesNo.Yes,
          children: [
            {
              id: 5,
              parentId: 4,
              menuType: 'page',
              code: 'system:menu:list',
              i18nKey: 'navigation.systemMenus',
              isEnabled: YesNo.Yes,
              children: [
                {
                  id: 6,
                  parentId: 5,
                  menuType: 'action',
                  code: 'system:menu:delete',
                  i18nKey: 'permission.menuDelete',
                  isEnabled: YesNo.No,
                  children: [],
                },
              ],
            },
          ],
        },
      ],
    },
  ]
}
