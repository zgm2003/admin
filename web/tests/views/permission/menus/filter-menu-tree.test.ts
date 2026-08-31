import { describe, expect, it } from 'vitest'
import type { ManagedMenuNode } from '@src/api/permission/menu'
import { filterManagedMenuTree } from '@/views/permission/menus/filter-menu-tree'

const action = (id: number, code: string, name: string): ManagedMenuNode => ({ id, platformId: 1, platformCode: 'admin', platformName: 'Admin', parentId: 2, menuType: 'action', name, code, i18nKey: null, path: null, componentPath: null, icon: null, sortOrder: id, isEnabled: 1, isHidden: 1, isProtected: 0, createdAt: '2026-08-19T02:00:00Z', updatedAt: '2026-08-19T02:00:00Z', children: [] })
const tree = (): ManagedMenuNode[] => [{ id: 1, platformId: 1, platformCode: 'admin', platformName: 'Admin', parentId: null, menuType: 'directory', name: '权限与认证', code: 'access', i18nKey: 'navigation.access', path: null, componentPath: null, icon: 'lucide:shield-check', sortOrder: 10, isEnabled: 1, isHidden: 0, isProtected: 0, createdAt: '2026-08-19T02:00:00Z', updatedAt: '2026-08-19T02:00:00Z', children: [{ id: 2, platformId: 1, platformCode: 'admin', platformName: 'Admin', parentId: 1, menuType: 'page', name: '菜单管理', code: 'permission:menu:list', i18nKey: 'navigation.accessMenus', path: '/access/menus', componentPath: 'access/menus', icon: 'lucide:panel-left', sortOrder: 10, isEnabled: 1, isHidden: 0, isProtected: 1, createdAt: '2026-08-19T02:00:00Z', updatedAt: '2026-08-19T02:00:00Z', children: [action(3, 'permission:menu:create', '新增菜单'), action(4, 'permission:menu:update', '修改菜单')] }] }]

describe('filterManagedMenuTree', () => {
  it('keeps matches, ancestors, and matched-node descendants without mutating input', () => {
    const source = tree()
    const result = filterManagedMenuTree(source, 'permission:menu:create')
    expect(result.map((node) => node.code)).toEqual(['access'])
    expect(result[0]?.children.map((node) => node.code)).toEqual(['permission:menu:list'])
    expect(result[0]?.children[0]?.children.map((node) => node.code)).toEqual(['permission:menu:create'])
    expect(source[0]?.children).toHaveLength(1)
  })

  it('matches name and path case-insensitively, clones full tree for empty keyword, and returns no result', () => {
    expect(filterManagedMenuTree(tree(), '权限与认证')).toHaveLength(1)
    expect(filterManagedMenuTree(tree(), '/ACCESS/MENUS')[0]?.children).toHaveLength(1)
    expect(filterManagedMenuTree(tree(), 'missing')).toEqual([])
    const cloned = filterManagedMenuTree(tree(), '  ')
    expect(cloned).not.toBe(tree())
  })
})
