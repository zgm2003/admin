// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/message/notification'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('getNotificationSummary preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getNotificationSummary()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/v1/message/notification/summary')
  })
  it('getNotificationSummary propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getNotificationSummary()).rejects.toBe(error)
  })
  it('listNotifications preserves the HTTP contract and backend data', async () => {
    const params: Parameters<typeof api.listNotifications>[0] = {}
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listNotifications(params)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/v1/message/notification', { params })
  })
  it('listNotifications propagates request failures unchanged', async () => {
    const params: Parameters<typeof api.listNotifications>[0] = {}
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listNotifications(params)).rejects.toBe(error)
  })
  it('readNotification preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.readNotification>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.readNotification(id)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/v1/message/notification/${id}/read`,
      undefined,
    )
  })
  it('readNotification propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.readNotification>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.readNotification(id)).rejects.toBe(error)
  })
  it('readAllNotifications preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.readAllNotifications()
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      '/api/v1/message/notification/read-all',
      undefined,
    )
  })
  it('readAllNotifications propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.readAllNotifications()).rejects.toBe(error)
  })
  it('deleteNotification preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteNotification>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteNotification(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(`/api/v1/message/notification/${id}`)
  })
  it('deleteNotification propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteNotification>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteNotification(id)).rejects.toBe(error)
  })
})
