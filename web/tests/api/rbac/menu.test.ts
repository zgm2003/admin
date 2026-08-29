import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { YesNo } from '@src/enums/yes-no'
import {
  createMenu,
  deleteMenu,
  getMenus,
  updateMenu,
  updateMenuStatus,
} from '@src/api/rbac/menu'
import type { CreateMenuInput, UpdateMenuInput } from '@src/api/rbac/menu'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('menu API', () => {
  beforeEach(() => {
    requestMock.mockReset()
  })

  it('loads and validates the platform menu catalog', async () => {
    const catalog = menuCatalog()
    requestMock.mockResolvedValue(catalog)
    await expect(getMenus({ platformId: 2 })).resolves.toEqual(catalog)
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/menus',
      params: { platformId: 2 },
    })
  })

  it.each([
    [],
    null,
    { platforms: menuCatalog().platforms },
    { platforms: menuCatalog().platforms, menuTree: [], extra: true },
    { platforms: [{ id: 2, code: 'canvas', name: 'Canvas', isEnabled: 2 }], menuTree: [] },
  ])('rejects invalid menu catalogs: %j', async (value) => {
    requestMock.mockResolvedValue(value)
    await expect(getMenus()).rejects.toThrow('menu catalog response is invalid')
  })

  it('creates a menu with the exact payload and validates the result', async () => {
    const input: CreateMenuInput = {
      platformId: 2,
      parentId: null,
      menuType: 'directory',
		name: '报表',
      code: 'reports',
      i18nKey: 'navigation.system',
      path: null,
		componentPath: null,
      icon: 'lucide:folder',
      sortOrder: 10,
      isEnabled: YesNo.Yes,
		isHidden: YesNo.No,
    }
    requestMock.mockResolvedValue({ id: 7 })
    await expect(createMenu(input)).resolves.toEqual({ id: 7 })
    expect(requestMock).toHaveBeenCalledWith({ method: 'POST', url: '/api/admin/v1/menus', data: input })
  })

  it('updates a menu without code or status', async () => {
    const input: UpdateMenuInput = {
      parentId: 1,
      menuType: 'page',
		name: '用户管理',
      i18nKey: 'navigation.accessMenus',
		path: '/account/users',
		componentPath: 'account/users',
      icon: 'lucide:panel-left',
      sortOrder: 10,
		isHidden: YesNo.No,
    }
    requestMock.mockResolvedValue({ id: 7 })
    await expect(updateMenu(7, input)).resolves.toEqual({ id: 7 })
    expect(requestMock).toHaveBeenCalledWith({ method: 'PUT', url: '/api/admin/v1/menus/7', data: input })
  })

  it('updates status with only isEnabled', async () => {
    requestMock.mockResolvedValue({ id: 7, isEnabled: 0 })
    await expect(updateMenuStatus(7, YesNo.No)).resolves.toEqual({ id: 7, isEnabled: 0 })
    expect(requestMock).toHaveBeenCalledWith({ method: 'PATCH', url: '/api/admin/v1/menus/7/status', data: { isEnabled: YesNo.No } })
  })

  it('deletes without a request body and returns the backend result', async () => {
    requestMock.mockResolvedValue({ id: 7 })
    await expect(deleteMenu(7)).resolves.toEqual({ id: 7 })
    expect(requestMock).toHaveBeenCalledWith({ method: 'DELETE', url: '/api/admin/v1/menus/7' })

    const result = { id: 7, extra: true }
    requestMock.mockResolvedValue(result)
    await expect(deleteMenu(7)).resolves.toBe(result)
  })
})

function menuCatalog() {
  return {
    platforms: [{ id: 2, code: 'canvas', name: 'Canvas', isEnabled: YesNo.Yes }],
    menuTree: [{
      id: 7,
      platformId: 2,
      platformCode: 'canvas',
      platformName: 'Canvas',
      parentId: null,
      menuType: 'page',
      name: 'Test',
      code: 'canvas:test',
      i18nKey: 'navigation.test',
      path: '/test',
      componentPath: 'test',
      icon: null,
      sortOrder: 10,
      isEnabled: YesNo.Yes,
      isHidden: YesNo.No,
      isProtected: YesNo.No,
      createdAt: '2026-08-27T00:00:00Z',
      updatedAt: '2026-08-27T00:00:00Z',
      children: [],
    }],
  }
}
