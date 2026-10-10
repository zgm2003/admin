// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/storage/cosConfig'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('listCosConfigs preserves the HTTP contract and backend data', async () => {
    const query: Parameters<typeof api.listCosConfigs>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listCosConfigs(query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/storage/cosconfig', {
      params: query,
    })
  })
  it('listCosConfigs propagates request failures unchanged', async () => {
    const query: Parameters<typeof api.listCosConfigs>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listCosConfigs(query)).rejects.toBe(error)
  })
  it('getCosConfig preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.getCosConfig>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getCosConfig(id)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/storage/cosconfig/${id}`)
  })
  it('getCosConfig propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.getCosConfig>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getCosConfig(id)).rejects.toBe(error)
  })
  it('createCosConfig preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.createCosConfig>[0] = {
      name: 'sample',
      appId: 'sample',
      secretId: 'sample',
      secretKey: 'sample',
      bucket: 'sample',
      region: 'sample',
      isEnabled: 0,
      remark: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createCosConfig(data)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/storage/cosconfig', data)
  })
  it('createCosConfig propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.createCosConfig>[0] = {
      name: 'sample',
      appId: 'sample',
      secretId: 'sample',
      secretKey: 'sample',
      bucket: 'sample',
      region: 'sample',
      isEnabled: 0,
      remark: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createCosConfig(data)).rejects.toBe(error)
  })
  it('updateCosConfig preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateCosConfig>[0] = 1
    const data: Parameters<typeof api.updateCosConfig>[1] = {
      name: 'sample',
      bucket: 'sample',
      region: 'sample',
      remark: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateCosConfig(id, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/storage/cosconfig/${id}`,
      data,
    )
  })
  it('updateCosConfig propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateCosConfig>[0] = 1
    const data: Parameters<typeof api.updateCosConfig>[1] = {
      name: 'sample',
      bucket: 'sample',
      region: 'sample',
      remark: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateCosConfig(id, data)).rejects.toBe(error)
  })
  it('updateCosConfigStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateCosConfigStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateCosConfigStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateCosConfigStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/storage/cosconfig/${id}/status`,
      { isEnabled },
    )
  })
  it('updateCosConfigStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateCosConfigStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateCosConfigStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateCosConfigStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('testCosConfig preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.testCosConfig>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.testCosConfig(id)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/storage/cosconfig/${id}/test`,
      undefined,
    )
  })
  it('testCosConfig propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.testCosConfig>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.testCosConfig(id)).rejects.toBe(error)
  })
  it('deleteCosConfig preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteCosConfig>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteCosConfig(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/storage/cosconfig/${id}`)
  })
  it('deleteCosConfig propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteCosConfig>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteCosConfig(id)).rejects.toBe(error)
  })
})
