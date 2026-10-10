// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/user/session'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getSessions preserves the HTTP contract and backend data', async () => {
    const query: Parameters<typeof api.getSessions>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getSessions(query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/session', {
      params: query,
    })
  })
  it('getSessions propagates request failures unchanged', async () => {
    const query: Parameters<typeof api.getSessions>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getSessions(query)).rejects.toBe(error)
  })
  it('getSessionStats preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getSessionStats()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/session/stats')
  })
  it('getSessionStats propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getSessionStats()).rejects.toBe(error)
  })
  it('revokeSession preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.revokeSession>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.revokeSession(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/session/' + id)
  })
  it('revokeSession propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.revokeSession>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.revokeSession(id)).rejects.toBe(error)
  })
  it('revokeSessions preserves the HTTP contract and backend data', async () => {
    const ids: Parameters<typeof api.revokeSessions>[0] = [1]
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.revokeSessions(ids)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/session', {
      data: { ids },
    })
  })
  it('revokeSessions propagates request failures unchanged', async () => {
    const ids: Parameters<typeof api.revokeSessions>[0] = [1]
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.revokeSessions(ids)).rejects.toBe(error)
  })
})
