// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/user/account'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getUsers preserves the HTTP contract and backend data', async () => {
    const query: Parameters<typeof api.getUsers>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getUsers(query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/account', {
      params: query,
    })
  })
  it('getUsers propagates request failures unchanged', async () => {
    const query: Parameters<typeof api.getUsers>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getUsers(query)).rejects.toBe(error)
  })
  it('getUserRoleOptions preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getUserRoleOptions()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/account/role-options')
  })
  it('getUserRoleOptions propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getUserRoleOptions()).rejects.toBe(error)
  })
  it('updateUser preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateUser>[0] = 1
    const input: Parameters<typeof api.updateUser>[1] = { username: 'sample' }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateUser(id, input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/user/account/${id}`, input)
  })
  it('updateUser propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateUser>[0] = 1
    const input: Parameters<typeof api.updateUser>[1] = { username: 'sample' }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateUser(id, input)).rejects.toBe(error)
  })
  it('updateUserStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateUserStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateUserStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateUserStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/user/account/${id}/status`,
      { isEnabled },
    )
  })
  it('updateUserStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateUserStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateUserStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateUserStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('deleteUser preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteUser>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteUser(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/user/account/${id}`)
  })
  it('deleteUser propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteUser>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteUser(id)).rejects.toBe(error)
  })
  it('getUserRoles preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.getUserRoles>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getUserRoles(id)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/user/account/${id}/role`)
  })
  it('getUserRoles propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.getUserRoles>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getUserRoles(id)).rejects.toBe(error)
  })
  it('updateUserRoles preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateUserRoles>[0] = 1
    const input: Parameters<typeof api.updateUserRoles>[1] = { roleIds: [1] }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateUserRoles(id, input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/user/account/${id}/role`, {
      roleIds: input.roleIds,
    })
  })
  it('updateUserRoles propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateUserRoles>[0] = 1
    const input: Parameters<typeof api.updateUserRoles>[1] = { roleIds: [1] }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateUserRoles(id, input)).rejects.toBe(error)
  })
})

describe('backend-owned form options', () => {
  it('preserves constraints and propagates failures', async () => {
    const options = { source: 'server' }
    vi.mocked(request.get).mockResolvedValueOnce(options)
    expect(await api.getUserFormOptions()).toBe(options)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/account/options')
    const error = new Error('options unavailable')
    vi.mocked(request.get).mockRejectedValueOnce(error)
    await expect(api.getUserFormOptions()).rejects.toBe(error)
  })
})
