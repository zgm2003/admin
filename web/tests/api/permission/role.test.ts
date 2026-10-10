// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/permission/role'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getRoles preserves the HTTP contract and backend data', async () => {
    const query: Parameters<typeof api.getRoles>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getRoles(query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/role', {
      params: query,
    })
  })
  it('getRoles propagates request failures unchanged', async () => {
    const query: Parameters<typeof api.getRoles>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getRoles(query)).rejects.toBe(error)
  })
  it('createRole preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.createRole>[0] = { code: 'sample', name: 'sample' }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createRole(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/role', {
      code: input.code,
      name: input.name,
    })
  })
  it('createRole propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.createRole>[0] = { code: 'sample', name: 'sample' }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createRole(input)).rejects.toBe(error)
  })
  it('updateRole preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateRole>[0] = 1
    const input: Parameters<typeof api.updateRole>[1] = { name: 'sample' }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateRole(id, input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/permission/role/${id}`, {
      name: input.name,
    })
  })
  it('updateRole propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateRole>[0] = 1
    const input: Parameters<typeof api.updateRole>[1] = { name: 'sample' }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateRole(id, input)).rejects.toBe(error)
  })
  it('updateRoleStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateRoleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateRoleStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateRoleStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/role/${id}/status`,
      { isEnabled },
    )
  })
  it('updateRoleStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateRoleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateRoleStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateRoleStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('setDefaultRole preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.setDefaultRole>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.setDefaultRole(id)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/role/${id}/default`,
      undefined,
    )
  })
  it('setDefaultRole propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.setDefaultRole>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.setDefaultRole(id)).rejects.toBe(error)
  })
  it('deleteRole preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteRole>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteRole(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/permission/role/${id}`)
  })
  it('deleteRole propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteRole>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteRole(id)).rejects.toBe(error)
  })
  it('getRolePermissions preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.getRolePermissions>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getRolePermissions(id)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/role/${id}/permission`,
    )
  })
  it('getRolePermissions propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.getRolePermissions>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getRolePermissions(id)).rejects.toBe(error)
  })
  it('updateRolePermissions preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateRolePermissions>[0] = 1
    const input: Parameters<typeof api.updateRolePermissions>[1] = { menuIds: [1] }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateRolePermissions(id, input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/role/${id}/permission`,
      { menuIds: input.menuIds },
    )
  })
  it('updateRolePermissions propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateRolePermissions>[0] = 1
    const input: Parameters<typeof api.updateRolePermissions>[1] = { menuIds: [1] }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateRolePermissions(id, input)).rejects.toBe(error)
  })
})

describe('backend-owned form options', () => {
  it('preserves constraints and propagates failures', async () => {
    const options = { source: 'server' }
    vi.mocked(request.get).mockResolvedValueOnce(options)
    expect(await api.getRoleFormOptions()).toBe(options)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/role/options')
    const error = new Error('options unavailable')
    vi.mocked(request.get).mockRejectedValueOnce(error)
    await expect(api.getRoleFormOptions()).rejects.toBe(error)
  })
})
