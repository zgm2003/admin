// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/user/phone'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('sendPhoneCode preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.sendPhoneCode>[0] = { target: 'current' }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.sendPhoneCode(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/user/phone/send-code',
      input,
    )
  })
  it('sendPhoneCode propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.sendPhoneCode>[0] = { target: 'current' }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.sendPhoneCode(input)).rejects.toBe(error)
  })
  it('bindPhone preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.bindPhone>[0] = {
      nextPhone: 'sample',
      nextChallengeId: 'sample',
      nextCode: 'sample',
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.bindPhone(input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/user/phone', input)
  })
  it('bindPhone propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.bindPhone>[0] = {
      nextPhone: 'sample',
      nextChallengeId: 'sample',
      nextCode: 'sample',
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.bindPhone(input)).rejects.toBe(error)
  })
  it('getPhoneChangeLogs preserves the HTTP contract and backend data', async () => {
    const userId: Parameters<typeof api.getPhoneChangeLogs>[0] = 1
    const query: Parameters<typeof api.getPhoneChangeLogs>[1] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getPhoneChangeLogs(userId, query)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/user/account/${userId}/phone-change-log`,
      { params: query },
    )
  })
  it('getPhoneChangeLogs propagates request failures unchanged', async () => {
    const userId: Parameters<typeof api.getPhoneChangeLogs>[0] = 1
    const query: Parameters<typeof api.getPhoneChangeLogs>[1] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getPhoneChangeLogs(userId, query)).rejects.toBe(error)
  })
})
