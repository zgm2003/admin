// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/system/setting'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('URL-encodes setting keys without changing submitted values', async () => {
    vi.mocked(request.put).mockResolvedValue({})
    const input = { value: 'unchanged', valueType: 1 }
    await api.updateSetting('app.a/b ?', input)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/system/setting/app.a%2Fb%20%3F',
      input,
    )
  })
  it('getSettings preserves the HTTP contract and backend data', async () => {
    const params: Parameters<typeof api.getSettings>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getSettings(params)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/setting', { params })
  })
  it('getSettings propagates request failures unchanged', async () => {
    const params: Parameters<typeof api.getSettings>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getSettings(params)).rejects.toBe(error)
  })
  it('getBrandSettings preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getBrandSettings()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/setting/brand')
  })
  it('getBrandSettings propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getBrandSettings()).rejects.toBe(error)
  })
  it('updateBrandSettings preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.updateBrandSettings>[0] = {
      titleZhCN: 'sample',
      titleEnUS: 'sample',
      defaultAvatar: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateBrandSettings(input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/setting/brand', input)
  })
  it('updateBrandSettings propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.updateBrandSettings>[0] = {
      titleZhCN: 'sample',
      titleEnUS: 'sample',
      defaultAvatar: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateBrandSettings(input)).rejects.toBe(error)
  })
  it('getPublicLegalDocument preserves the HTTP contract and backend data', async () => {
    const kind: Parameters<typeof api.getPublicLegalDocument>[0] = 'userAgreement'
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getPublicLegalDocument(kind)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(`/api/v1/system/setting/legal/${kind}`)
  })
  it('getPublicLegalDocument propagates request failures unchanged', async () => {
    const kind: Parameters<typeof api.getPublicLegalDocument>[0] = 'userAgreement'
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getPublicLegalDocument(kind)).rejects.toBe(error)
  })
  it('getLegalDocument preserves the HTTP contract and backend data', async () => {
    const kind: Parameters<typeof api.getLegalDocument>[0] = 'userAgreement'
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getLegalDocument(kind)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/setting/legal/${kind}`,
    )
  })
  it('getLegalDocument propagates request failures unchanged', async () => {
    const kind: Parameters<typeof api.getLegalDocument>[0] = 'userAgreement'
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getLegalDocument(kind)).rejects.toBe(error)
  })
  it('updateLegalDocument preserves the HTTP contract and backend data', async () => {
    const kind: Parameters<typeof api.updateLegalDocument>[0] = 'userAgreement'
    const contentHtml: Parameters<typeof api.updateLegalDocument>[1] = 'sample'
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateLegalDocument(kind, contentHtml)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/setting/legal/${kind}`,
      { contentHtml },
    )
  })
  it('updateLegalDocument propagates request failures unchanged', async () => {
    const kind: Parameters<typeof api.updateLegalDocument>[0] = 'userAgreement'
    const contentHtml: Parameters<typeof api.updateLegalDocument>[1] = 'sample'
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateLegalDocument(kind, contentHtml)).rejects.toBe(error)
  })
  it('createSetting preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.createSetting>[0] = {
      key: 'sample',
      value: 'sample',
      valueType: 1,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createSetting(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/setting', input)
  })
  it('createSetting propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.createSetting>[0] = {
      key: 'sample',
      value: 'sample',
      valueType: 1,
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createSetting(input)).rejects.toBe(error)
  })
  it('updateSetting preserves the HTTP contract and backend data', async () => {
    const key: Parameters<typeof api.updateSetting>[0] = 'sample'
    const input: Parameters<typeof api.updateSetting>[1] = { value: 'sample', valueType: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateSetting(key, input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/setting/${encodeURIComponent(key)}`,
      input,
    )
  })
  it('updateSetting propagates request failures unchanged', async () => {
    const key: Parameters<typeof api.updateSetting>[0] = 'sample'
    const input: Parameters<typeof api.updateSetting>[1] = { value: 'sample', valueType: 1 }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateSetting(key, input)).rejects.toBe(error)
  })
  it('updateSettingStatus preserves the HTTP contract and backend data', async () => {
    const key: Parameters<typeof api.updateSettingStatus>[0] = 'sample'
    const isEnabled: Parameters<typeof api.updateSettingStatus>[1] = 0
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.updateSettingStatus(key, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/setting/${encodeURIComponent(key)}/status`,
      { isEnabled },
    )
  })
  it('updateSettingStatus propagates request failures unchanged', async () => {
    const key: Parameters<typeof api.updateSettingStatus>[0] = 'sample'
    const isEnabled: Parameters<typeof api.updateSettingStatus>[1] = 0
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.updateSettingStatus(key, isEnabled)).rejects.toBe(error)
  })
  it('deleteSetting preserves the HTTP contract and backend data', async () => {
    const key: Parameters<typeof api.deleteSetting>[0] = 'sample'
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteSetting(key)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/setting/${encodeURIComponent(key)}`,
    )
  })
  it('deleteSetting propagates request failures unchanged', async () => {
    const key: Parameters<typeof api.deleteSetting>[0] = 'sample'
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteSetting(key)).rejects.toBe(error)
  })
})
