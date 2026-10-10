// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/permission/menu'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('omits platform query config when loading the full menu catalog', async () => {
    const result = { platforms: [], menuTree: [], serverAdded: true }
    vi.mocked(request.get).mockResolvedValue(result)
    await expect(api.getMenus()).resolves.toBe(result)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/menu')
  })

  it('leaves invalid platform query rejection to the backend', async () => {
    const error = new Error('backend rejected the platform')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getMenus({ platformId: 0 })).rejects.toBe(error)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/menu', {
      params: { platformId: 0 },
    })
  })
  it('getMenus preserves the HTTP contract and backend data', async () => {
    const query: Parameters<typeof api.getMenus>[0] = { platformId: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getMenus(query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/menu', {
      params: { platformId: query.platformId },
    })
  })
  it('getMenus propagates request failures unchanged', async () => {
    const query: Parameters<typeof api.getMenus>[0] = { platformId: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getMenus(query)).rejects.toBe(error)
  })
  it('createMenu preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.createMenu>[0] = {
      platformId: 1,
      parentId: null,
      menuType: 'directory',
      name: 'sample',
      code: 'sample',
      i18nKey: null,
      path: null,
      componentPath: null,
      icon: null,
      remark: null,
      sortOrder: 1,
      isEnabled: 0,
      isHidden: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createMenu(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/menu', input)
  })
  it('createMenu propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.createMenu>[0] = {
      platformId: 1,
      parentId: null,
      menuType: 'directory',
      name: 'sample',
      code: 'sample',
      i18nKey: null,
      path: null,
      componentPath: null,
      icon: null,
      remark: null,
      sortOrder: 1,
      isEnabled: 0,
      isHidden: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createMenu(input)).rejects.toBe(error)
  })
  it('updateMenu preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateMenu>[0] = 1
    const input: Parameters<typeof api.updateMenu>[1] = {
      parentId: null,
      menuType: 'directory',
      name: 'sample',
      i18nKey: null,
      path: null,
      componentPath: null,
      icon: null,
      remark: null,
      sortOrder: 1,
      isHidden: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateMenu(id, input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/menu/${id}`,
      input,
    )
  })
  it('updateMenu propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateMenu>[0] = 1
    const input: Parameters<typeof api.updateMenu>[1] = {
      parentId: null,
      menuType: 'directory',
      name: 'sample',
      i18nKey: null,
      path: null,
      componentPath: null,
      icon: null,
      remark: null,
      sortOrder: 1,
      isHidden: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateMenu(id, input)).rejects.toBe(error)
  })
  it('updateMenuStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateMenuStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateMenuStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateMenuStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/menu/${id}/status`,
      { isEnabled },
    )
  })
  it('updateMenuStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateMenuStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateMenuStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateMenuStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('deleteMenu preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteMenu>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteMenu(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/permission/menu/${id}`)
  })
  it('deleteMenu propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteMenu>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteMenu(id)).rejects.toBe(error)
  })
  it('rebuildAccessCache preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.rebuildAccessCache()
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/permission/menu/access-cache/rebuild',
      undefined,
    )
  })
  it('rebuildAccessCache propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.rebuildAccessCache()).rejects.toBe(error)
  })
})
