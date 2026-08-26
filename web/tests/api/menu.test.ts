import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { YesNo } from '@src/enums/yes-no'
import { ProtocolError } from '@src/types/http'
import {
  createMenu,
  deleteMenu,
  getMenus,
  updateMenu,
  updateMenuStatus,
} from '@src/api/menu'
import type { CreateMenuInput, UpdateMenuInput } from '@src/api/menu.contract'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))

const requestMock = vi.mocked(request)

describe('menu API', () => {
  beforeEach(() => {
    requestMock.mockReset()
  })

  it('loads and validates the managed menu tree', async () => {
    requestMock.mockResolvedValue([])
    await expect(getMenus()).resolves.toEqual([])
    expect(requestMock).toHaveBeenCalledWith({ method: 'GET', url: '/api/v1/menus' })

    requestMock.mockResolvedValue(null)
    await expect(getMenus()).rejects.toBeInstanceOf(ProtocolError)
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
    expect(requestMock).toHaveBeenCalledWith({ method: 'POST', url: '/api/v1/menus', data: input })
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
    expect(requestMock).toHaveBeenCalledWith({ method: 'PUT', url: '/api/v1/menus/7', data: input })
  })

  it('updates status with only isEnabled', async () => {
    requestMock.mockResolvedValue({ id: 7, isEnabled: 0 })
    await expect(updateMenuStatus(7, YesNo.No)).resolves.toEqual({ id: 7, isEnabled: 0 })
    expect(requestMock).toHaveBeenCalledWith({ method: 'PATCH', url: '/api/v1/menus/7/status', data: { isEnabled: YesNo.No } })
  })

  it('deletes without a request body and rejects malformed mutation results', async () => {
    requestMock.mockResolvedValue({ id: 7 })
    await expect(deleteMenu(7)).resolves.toEqual({ id: 7 })
    expect(requestMock).toHaveBeenCalledWith({ method: 'DELETE', url: '/api/v1/menus/7' })

    requestMock.mockResolvedValue({ id: 7, extra: true })
    await expect(deleteMenu(7)).rejects.toBeInstanceOf(ProtocolError)
  })
})
