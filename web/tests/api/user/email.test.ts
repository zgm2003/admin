// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/user/email'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('sendEmailCode preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.sendEmailCode>[0] = { target: 'current' }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.sendEmailCode(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/user/email/send-code',
      input,
    )
  })
  it('sendEmailCode propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.sendEmailCode>[0] = { target: 'current' }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.sendEmailCode(input)).rejects.toBe(error)
  })
  it('bindEmail preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.bindEmail>[0] = {
      nextEmail: 'sample',
      nextChallengeId: 'sample',
      nextCode: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.bindEmail(input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/email', input)
  })
  it('bindEmail propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.bindEmail>[0] = {
      nextEmail: 'sample',
      nextChallengeId: 'sample',
      nextCode: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.bindEmail(input)).rejects.toBe(error)
  })
  it('getEmailChangeLogs preserves the HTTP contract and backend data', async () => {
    const userId: Parameters<typeof api.getEmailChangeLogs>[0] = 1
    const query: Parameters<typeof api.getEmailChangeLogs>[1] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getEmailChangeLogs(userId, query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/user/account/${userId}/email-change-log`,
      { params: query },
    )
  })
  it('getEmailChangeLogs propagates request failures unchanged', async () => {
    const userId: Parameters<typeof api.getEmailChangeLogs>[0] = 1
    const query: Parameters<typeof api.getEmailChangeLogs>[1] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getEmailChangeLogs(userId, query)).rejects.toBe(error)
  })
})
