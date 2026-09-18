import { describe, expect, it, vi } from 'vitest'
import { request } from '@/utils/request'
import { ProtocolError } from '@/types/http'
import {
  getUploadRulePageInit,
  listUploadRules,
  updateUploadRule,
} from '@/api/storage/uploadRule'
vi.mock('@/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)
describe('upload rule API', () => {
  it('uses exact admin routes', async () => {
    requestMock.mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20 })
    await listUploadRules({ page: 1, pageSize: 20, platformId: 2 })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/storage/uploadrule',
      params: { page: 1, pageSize: 20, platformId: 2 },
    })
    requestMock.mockResolvedValueOnce({ platforms: [], configs: [] })
    await getUploadRulePageInit()
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/storage/uploadrule/page-init',
    })
  })

  it('rejects null page-init collections instead of silently defaulting', async () => {
    requestMock.mockResolvedValueOnce({ platforms: null, configs: null })
    await expect(getUploadRulePageInit()).rejects.toThrow('platforms must be an array')
  })

  it('rejects unexpected version fields in rule responses', async () => {
    const rule = {
      id: 1,
      platformId: 1,
      platformCode: 'admin',
      platformName: 'Admin',
      codes: ['avatar'],
      name: 'Avatar',
      cosConfigId: 2,
      cosConfigName: 'Main',
      maxFileSizeBytes: 1024,
      allowedExtensions: ['png'],
      allowedMimeTypes: ['image/png'],
      accessMode: 'private',
      isEnabled: 1,
      remark: '',
      createdAt: '2026-09-17T00:00:00Z',
      updatedAt: '2026-09-17T00:00:00Z',
    }
    for (const field of ['generation', 'revision', 'currentVersion']) {
      requestMock.mockResolvedValueOnce({
        list: [{ ...rule, [field]: 1 }],
        total: 1,
        page: 1,
        pageSize: 20,
      })
      await expect(listUploadRules({ page: 1, pageSize: 20 })).rejects.toThrow(ProtocolError)
    }
  })

  it('updates only mutable rule fields', async () => {
    requestMock.mockResolvedValueOnce({})
    await updateUploadRule(9, {
      codes: ['avatar-v2'],
      name: 'Avatar v2',
      maxFileSizeBytes: 2048,
      allowedExtensions: ['png'],
      allowedMimeTypes: ['image/png'],
      remark: '',
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/storage/uploadrule/9',
      data: {
        codes: ['avatar-v2'],
        name: 'Avatar v2',
        maxFileSizeBytes: 2048,
        allowedExtensions: ['png'],
        allowedMimeTypes: ['image/png'],
        remark: '',
      },
    })
  })
})
