import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { YesNo } from '@src/enums/yes-no'
import {
  createMenu,
  deleteMenu,
  getMenus,
  updateMenu,
  updateMenuStatus,
} from '@src/api/menu'
import type { CreateMenuInput, UpdateMenuInput } from '@src/api/menu'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('menu API', () => {
  beforeEach(() => {
    requestMock.mockReset()
  })

  it('loads the managed menu tree', async () => {
    requestMock.mockResolvedValue([])
    await expect(getMenus()).resolves.toEqual([])
    expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/admin/v1/menus' })

    requestMock.mockResolvedValue(null)
    await expect(getMenus()).resolves.toBeNull()
  })

  it('creates a menu with the exact payload and validates the result', async () => {
    const input: CreateMenuInput = {
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
