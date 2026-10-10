// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/message/notificationTask'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('listNotificationTasks preserves the HTTP contract and backend data', async () => {
    const params: Parameters<typeof api.listNotificationTasks>[0] = { page: 1, pageSize: 1 }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listNotificationTasks(params)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/message/notificationtask', {
      params,
    })
  })
  it('listNotificationTasks propagates request failures unchanged', async () => {
    const params: Parameters<typeof api.listNotificationTasks>[0] = { page: 1, pageSize: 1 }
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listNotificationTasks(params)).rejects.toBe(error)
  })
  it('getNotificationTask preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.getNotificationTask>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getNotificationTask(id)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      `${'/api/admin/v1/message/notificationtask'}/${id}`,
    )
  })
  it('getNotificationTask propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.getNotificationTask>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getNotificationTask(id)).rejects.toBe(error)
  })
  it('getNotificationTaskForUpdate preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.getNotificationTaskForUpdate>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getNotificationTaskForUpdate(id)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      `${'/api/admin/v1/message/notificationtask'}/${id}/edit`,
    )
  })
  it('getNotificationTaskForUpdate propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.getNotificationTaskForUpdate>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getNotificationTaskForUpdate(id)).rejects.toBe(error)
  })
  it('createNotificationTask preserves the HTTP contract and backend data', async () => {
    const data: Parameters<typeof api.createNotificationTask>[0] = {
      platformId: 1,
      title: 'sample',
      contentHtml: 'sample',
      variant: 'info',
      priority: 'normal',
      linkType: 'none',
      link: 'sample',
      audienceType: 'user',
      targetIds: [1],
      scheduledAt: null,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createNotificationTask(data)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/message/notificationtask',
      data,
    )
  })
  it('createNotificationTask propagates request failures unchanged', async () => {
    const data: Parameters<typeof api.createNotificationTask>[0] = {
      platformId: 1,
      title: 'sample',
      contentHtml: 'sample',
      variant: 'info',
      priority: 'normal',
      linkType: 'none',
      link: 'sample',
      audienceType: 'user',
      targetIds: [1],
      scheduledAt: null,
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createNotificationTask(data)).rejects.toBe(error)
  })
  it('updateNotificationTask preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateNotificationTask>[0] = 1
    const data: Parameters<typeof api.updateNotificationTask>[1] = {
      platformId: 1,
      title: 'sample',
      contentHtml: 'sample',
      variant: 'info',
      priority: 'normal',
      linkType: 'none',
      link: 'sample',
      audienceType: 'user',
      targetIds: [1],
      scheduledAt: null,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateNotificationTask(id, data)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `${'/api/admin/v1/message/notificationtask'}/${id}`,
      data,
    )
  })
  it('updateNotificationTask propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateNotificationTask>[0] = 1
    const data: Parameters<typeof api.updateNotificationTask>[1] = {
      platformId: 1,
      title: 'sample',
      contentHtml: 'sample',
      variant: 'info',
      priority: 'normal',
      linkType: 'none',
      link: 'sample',
      audienceType: 'user',
      targetIds: [1],
      scheduledAt: null,
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateNotificationTask(id, data)).rejects.toBe(error)
  })
  it('deleteNotificationTask preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteNotificationTask>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteNotificationTask(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(
      `${'/api/admin/v1/message/notificationtask'}/${id}`,
    )
  })
  it('deleteNotificationTask propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteNotificationTask>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteNotificationTask(id)).rejects.toBe(error)
  })
  it('commandNotificationTask preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.commandNotificationTask>[0] = 1
    const command: Parameters<typeof api.commandNotificationTask>[1] = 'submit'
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.commandNotificationTask(id, command)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      `${'/api/admin/v1/message/notificationtask'}/${id}/${command}`,
      undefined,
    )
  })
  it('commandNotificationTask propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.commandNotificationTask>[0] = 1
    const command: Parameters<typeof api.commandNotificationTask>[1] = 'submit'
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.commandNotificationTask(id, command)).rejects.toBe(error)
  })
  it('listNotificationTaskOptions preserves the HTTP contract and backend data', async () => {
    const intent: Parameters<typeof api.listNotificationTaskOptions>[0] = 'create'
    const kind: Parameters<typeof api.listNotificationTaskOptions>[1] = 'user'
    const params: Parameters<typeof api.listNotificationTaskOptions>[2] = {}
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listNotificationTaskOptions(intent, kind, params)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      `${'/api/admin/v1/message/notificationtask'}/${intent}/option/${kind}`,
      { params },
    )
  })
  it('listNotificationTaskOptions propagates request failures unchanged', async () => {
    const intent: Parameters<typeof api.listNotificationTaskOptions>[0] = 'create'
    const kind: Parameters<typeof api.listNotificationTaskOptions>[1] = 'user'
    const params: Parameters<typeof api.listNotificationTaskOptions>[2] = {}
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listNotificationTaskOptions(intent, kind, params)).rejects.toBe(error)
  })
})
