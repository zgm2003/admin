// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/storage/uploadRule'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('listUploadRules preserves the HTTP contract and backend data', async () => {
    const query: Parameters<typeof api.listUploadRules>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listUploadRules(query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/storage/uploadrule', {
      params: query,
    })
  })
  it('listUploadRules propagates request failures unchanged', async () => {
    const query: Parameters<typeof api.listUploadRules>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listUploadRules(query)).rejects.toBe(error)
  })
  it('getUploadRule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.getUploadRule>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getUploadRule(id)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/storage/uploadrule/${id}`)
  })
  it('getUploadRule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.getUploadRule>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getUploadRule(id)).rejects.toBe(error)
  })
  it('getUploadRulePageInit preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getUploadRulePageInit()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/storage/uploadrule/page-init',
    )
  })
  it('getUploadRulePageInit propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getUploadRulePageInit()).rejects.toBe(error)
  })
  it('createUploadRule preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.createUploadRule>[0] = {
      platformId: 1,
      isEnabled: 0,
      codes: ['sample'],
      name: 'sample',
      cosConfigId: 1,
      maxFileSizeBytes: 1,
      allowedExtensions: ['sample'],
      allowedMimeTypes: ['sample'],
      accessMode: 'private',
      remark: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createUploadRule(data)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/storage/uploadrule', data)
  })
  it('createUploadRule propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.createUploadRule>[0] = {
      platformId: 1,
      isEnabled: 0,
      codes: ['sample'],
      name: 'sample',
      cosConfigId: 1,
      maxFileSizeBytes: 1,
      allowedExtensions: ['sample'],
      allowedMimeTypes: ['sample'],
      accessMode: 'private',
      remark: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createUploadRule(data)).rejects.toBe(error)
  })
  it('updateUploadRule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateUploadRule>[0] = 1
    const data: Parameters<typeof api.updateUploadRule>[1] = {
      codes: ['sample'],
      name: 'sample',
      maxFileSizeBytes: 1,
      allowedExtensions: ['sample'],
      allowedMimeTypes: ['sample'],
      remark: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateUploadRule(id, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/storage/uploadrule/${id}`,
      data,
    )
  })
  it('updateUploadRule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateUploadRule>[0] = 1
    const data: Parameters<typeof api.updateUploadRule>[1] = {
      codes: ['sample'],
      name: 'sample',
      maxFileSizeBytes: 1,
      allowedExtensions: ['sample'],
      allowedMimeTypes: ['sample'],
      remark: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateUploadRule(id, data)).rejects.toBe(error)
  })
  it('updateUploadRuleStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateUploadRuleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateUploadRuleStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateUploadRuleStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/storage/uploadrule/${id}/status`,
      { isEnabled },
    )
  })
  it('updateUploadRuleStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateUploadRuleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.updateUploadRuleStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateUploadRuleStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('deleteUploadRule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteUploadRule>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteUploadRule(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(`/api/admin/v1/storage/uploadrule/${id}`)
  })
  it('deleteUploadRule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteUploadRule>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteUploadRule(id)).rejects.toBe(error)
  })
})
