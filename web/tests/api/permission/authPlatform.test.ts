// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/permission/authPlatform'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getAuthPlatforms preserves the HTTP contract and backend data', async () => {
    const query: Parameters<typeof api.getAuthPlatforms>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getAuthPlatforms(query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/authplatform', {
      params: query,
    })
  })
  it('getAuthPlatforms propagates request failures unchanged', async () => {
    const query: Parameters<typeof api.getAuthPlatforms>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getAuthPlatforms(query)).rejects.toBe(error)
  })
  it('createAuthPlatform preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.createAuthPlatform>[0] = {
      code: 'sample',
      name: 'sample',
      loginTypes: ['password'],
      accessTTLSeconds: 1,
      refreshTTLSeconds: 1,
      sessionCacheTTLSeconds: 1,
      accessCacheTTLSeconds: 1,
      bindDevice: 0,
      bindIP: 0,
      maxSessions: 1,
      allowRegister: 0,
      isEnabled: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createAuthPlatform(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/permission/authplatform', {
      code: input.code,
      name: input.name,
      loginTypes: input.loginTypes,
      accessTTLSeconds: input.accessTTLSeconds,
      refreshTTLSeconds: input.refreshTTLSeconds,
      sessionCacheTTLSeconds: input.sessionCacheTTLSeconds,
      accessCacheTTLSeconds: input.accessCacheTTLSeconds,
      bindDevice: input.bindDevice,
      bindIP: input.bindIP,
      maxSessions: input.maxSessions,
      allowRegister: input.allowRegister,
      isEnabled: input.isEnabled,
    })
  })
  it('createAuthPlatform propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.createAuthPlatform>[0] = {
      code: 'sample',
      name: 'sample',
      loginTypes: ['password'],
      accessTTLSeconds: 1,
      refreshTTLSeconds: 1,
      sessionCacheTTLSeconds: 1,
      accessCacheTTLSeconds: 1,
      bindDevice: 0,
      bindIP: 0,
      maxSessions: 1,
      allowRegister: 0,
      isEnabled: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createAuthPlatform(input)).rejects.toBe(error)
  })
  it('updateAuthPlatform preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateAuthPlatform>[0] = 1
    const input: Parameters<typeof api.updateAuthPlatform>[1] = {
      name: 'sample',
      loginTypes: ['password'],
      accessTTLSeconds: 1,
      refreshTTLSeconds: 1,
      sessionCacheTTLSeconds: 1,
      accessCacheTTLSeconds: 1,
      bindDevice: 0,
      bindIP: 0,
      maxSessions: 1,
      allowRegister: 0,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateAuthPlatform(id, input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/authplatform/${id}`,
      {
        name: input.name,
        loginTypes: input.loginTypes,
        accessTTLSeconds: input.accessTTLSeconds,
        refreshTTLSeconds: input.refreshTTLSeconds,
        sessionCacheTTLSeconds: input.sessionCacheTTLSeconds,
        accessCacheTTLSeconds: input.accessCacheTTLSeconds,
        bindDevice: input.bindDevice,
        bindIP: input.bindIP,
        maxSessions: input.maxSessions,
        allowRegister: input.allowRegister,
      },
    )
  })
  it('updateAuthPlatform propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateAuthPlatform>[0] = 1
    const input: Parameters<typeof api.updateAuthPlatform>[1] = {
      name: 'sample',
      loginTypes: ['password'],
      accessTTLSeconds: 1,
      refreshTTLSeconds: 1,
      sessionCacheTTLSeconds: 1,
      accessCacheTTLSeconds: 1,
      bindDevice: 0,
      bindIP: 0,
      maxSessions: 1,
      allowRegister: 0,
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateAuthPlatform(id, input)).rejects.toBe(error)
  })
  it('updateAuthPlatformStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateAuthPlatformStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateAuthPlatformStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateAuthPlatformStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/authplatform/${id}/status`,
      { isEnabled },
    )
  })
  it('updateAuthPlatformStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateAuthPlatformStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateAuthPlatformStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateAuthPlatformStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('deleteAuthPlatform preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteAuthPlatform>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteAuthPlatform(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/permission/authplatform/${id}`,
    )
  })
  it('deleteAuthPlatform propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteAuthPlatform>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteAuthPlatform(id)).rejects.toBe(error)
  })
})
