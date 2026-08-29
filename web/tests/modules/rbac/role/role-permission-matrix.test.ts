import { describe, expect, it } from 'vitest'

import type { RolePermissionPlatform, RolePermissionTreeNode } from '@src/api/rbac/role'
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
} from '@src/modules/rbac/role/role-permission-matrix'

describe('role permission matrix', () => {
  it('builds platform groups and supports a Canvas root page', () => {
    const platforms = buildRolePermissionMatrix(permissionPlatforms())
    const groups = platforms.flatMap((platform) => platform.groups)

    expect(platforms.map((platform) => platform.platformCode)).toEqual(['admin', 'canvas'])
    expect(groups).toHaveLength(2)
    expect(groups[0]?.groupKey).toBe('menu:1')
    expect(groups[0]?.rows.map((row) => row.pageId)).toEqual([2, 5])
    expect(groups[0]?.rows[1]?.actions[0]).toMatchObject({
      id: 6,
      isEnabled: YesNo.No,
    })
    expect(platforms[1]?.platformIsEnabled).toBe(YesNo.No)
    expect(platforms[1]?.groups[0]).toMatchObject({
      groupKey: 'platform:2',
      rows: [{ pageId: 7, actions: [{ id: 8 }] }],
    })
  })

  it('expands action grants with their page and preserves direct page grants', () => {
    const groups = matrixGroups()

    expect(expandDirectMenuIDs(groups, [3, 5, 8])).toEqual([2, 3, 5, 7, 8])
  })

  it('keeps page and action selection semantically valid', () => {
    const row = matrixGroups()[0]?.rows[0]
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
    const group = matrixGroups()[0]
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
    const groups = matrixGroups()

    expect(normalizeDirectMenuIDs(groups, [2, 3])).toEqual([3])
    expect(normalizeDirectMenuIDs(groups, [2])).toEqual([2])
    expect(normalizeDirectMenuIDs(groups, [2, 3, 5, 6])).toEqual([3, 6])
  })

  it('calculates stable added and removed permission ids', () => {
    expect(diffMenuIDs([2, 3], [2, 4])).toEqual({ added: [4], removed: [3] })
  })
})

function matrixGroups() {
  return buildRolePermissionMatrix(permissionPlatforms()).flatMap((platform) => platform.groups)
}

function permissionPlatforms(): RolePermissionPlatform[] {
  return [
    { id: 1, code: 'admin', name: 'Admin', isEnabled: YesNo.Yes, menuTree: menuTree() },
    {
      id: 2,
      code: 'canvas',
      name: 'Canvas',
      isEnabled: YesNo.No,
      menuTree: [{
        id: 7,
        parentId: null,
        menuType: 'page',
        code: 'canvas:test:list',
        name: 'Test',
        isEnabled: YesNo.Yes,
        children: [{
          id: 8,
          parentId: 7,
          menuType: 'action',
          code: 'canvas:test:button',
          name: 'Test Button',
          isEnabled: YesNo.Yes,
          children: [],
        }],
      }],
    },
  ]
}

function menuTree(): RolePermissionTreeNode[] {
  return [
    {
      id: 1,
      parentId: null,
      menuType: 'directory',
      code: 'system',
			name: '系统管理',
      isEnabled: YesNo.Yes,
      children: [
        {
          id: 2,
          parentId: 1,
          menuType: 'page',
          code: 'rbac:role:list',
					name: '角色管理',
          isEnabled: YesNo.Yes,
          children: [
            {
              id: 3,
              parentId: 2,
              menuType: 'action',
              code: 'rbac:role:create',
							name: '新增角色',
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
					name: '系统设置',
          isEnabled: YesNo.Yes,
          children: [
            {
              id: 5,
              parentId: 4,
              menuType: 'page',
              code: 'rbac:menu:list',
							name: '菜单管理',
              isEnabled: YesNo.Yes,
              children: [
                {
                  id: 6,
                  parentId: 5,
                  menuType: 'action',
                  code: 'rbac:menu:delete',
									name: '删除菜单',
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
