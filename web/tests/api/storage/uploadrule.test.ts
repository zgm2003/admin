import { describe, expect, it, vi } from 'vitest'
import { request } from '@/utils/request'
import { listUploadRules, getUploadRulePageInit } from '@/api/storage/uploadrule'
vi.mock('@/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)
describe('upload rule API', () => {
  it('uses exact admin routes', async () => {
    requestMock.mockResolvedValueOnce({ list: [], total: 0, page: 1, pageSize: 20 })
    await listUploadRules({ page: 1, pageSize: 20, platformId: 2 })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/storage/upload-rules',
      params: { page: 1, pageSize: 20, platformId: 2 },
    })
    requestMock.mockResolvedValueOnce({ platforms: [], configs: [] })
    await getUploadRulePageInit()
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/storage/upload-rules/page-init',
    })
  })

  it('rejects null page-init collections instead of silently defaulting', async () => {
    requestMock.mockResolvedValueOnce({ platforms: null, configs: null })
    await expect(getUploadRulePageInit()).rejects.toThrow('platforms must be an array')
  })
})
