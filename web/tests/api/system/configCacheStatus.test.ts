// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/system/configCacheStatus'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getConfigCacheStatuses preserves the HTTP contract and backend data', async () => {
    const params: Parameters<typeof api.getConfigCacheStatuses>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getConfigCacheStatuses(params)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/system/config-cache-status',
      {
        params,
      },
    )
  })
  it('getConfigCacheStatuses propagates request failures unchanged', async () => {
    const params: Parameters<typeof api.getConfigCacheStatuses>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getConfigCacheStatuses(params)).rejects.toBe(error)
  })
})
