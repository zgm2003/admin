// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/system/operationLog'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getOperationLogs preserves the HTTP contract and backend data', async () => {
    const query: Parameters<typeof api.getOperationLogs>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = {
      list: [
        {
          id: 7,
          requestId: 'request-7',
          userId: null,
          userName: '',
          sessionId: null,
          platform: 'admin',
          method: 'PATCH',
          route: '/api/admin/v1/future/:id',
          module: 'future',
          action: 'future.newAction',
          actionLabel: '来自后端的新动作',
          clientIp: '127.0.0.1',
          userAgent: '',
          statusCode: 200,
          isSuccess: 1,
          latencyMs: 0,
          requestData: null,
          responseData: { code: 0 },
          createdAt: '2026-10-10T00:00:00Z',
          updatedAt: '2026-10-10T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 1,
      serverAdded: { label: 'new value', numericValue: 99 },
    }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getOperationLogs(query)
    expect(result).toBe(dataFromServer)
    expect(result.list[0]?.action).toBe('future.newAction')
    expect(result.list[0]?.actionLabel).toBe('来自后端的新动作')
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/operationlog', {
      params: query,
    })
  })
  it('getOperationLogs propagates request failures unchanged', async () => {
    const query: Parameters<typeof api.getOperationLogs>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getOperationLogs(query)).rejects.toBe(error)
  })
})
