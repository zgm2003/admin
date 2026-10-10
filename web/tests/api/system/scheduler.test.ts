// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/system/scheduler'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('passes a numeric zero status instead of dropping a backend enum value', async () => {
    vi.mocked(request.get).mockResolvedValue([])
    await api.listJobs(0, 0)
    expect(request.get).toHaveBeenCalledWith('/api/admin/v1/system/scheduler/job', {
      params: { afterId: 0, limit: 100, status: 0 },
    })
  })
  it('listSchedules preserves the HTTP contract and backend data', async () => {
    const afterId: Parameters<typeof api.listSchedules>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listSchedules(afterId)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/scheduler/schedule', {
      params: { afterId, limit: 100 },
    })
  })
  it('listSchedules propagates request failures unchanged', async () => {
    const afterId: Parameters<typeof api.listSchedules>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listSchedules(afterId)).rejects.toBe(error)
  })
  it('listJobs preserves the HTTP contract and backend data', async () => {
    const afterId: Parameters<typeof api.listJobs>[0] = 1
    const status: Parameters<typeof api.listJobs>[1] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listJobs(afterId, status)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/scheduler/job', {
      params: { afterId, limit: 100, ...(status !== undefined ? { status } : {}) },
    })
  })
  it('listJobs propagates request failures unchanged', async () => {
    const afterId: Parameters<typeof api.listJobs>[0] = 1
    const status: Parameters<typeof api.listJobs>[1] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listJobs(afterId, status)).rejects.toBe(error)
  })
  it('listRuns preserves the HTTP contract and backend data', async () => {
    const jobId: Parameters<typeof api.listRuns>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.listRuns(jobId)
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/scheduler/job/${jobId}/run`,
      { params: { afterId: 0, limit: 100 } },
    )
  })
  it('listRuns propagates request failures unchanged', async () => {
    const jobId: Parameters<typeof api.listRuns>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.listRuns(jobId)).rejects.toBe(error)
  })
  it('getSchedulerOptions preserves the HTTP contract and backend data', async () => {
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.get).mockResolvedValue(dataFromServer)
    const result = await api.getSchedulerOptions()
    expect(result).toBe(dataFromServer)
    expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/scheduler/options')
  })
  it('getSchedulerOptions propagates request failures unchanged', async () => {
    const error = new Error('request failed')
    vi.mocked(request.get).mockRejectedValue(error)
    await expect(api.getSchedulerOptions()).rejects.toBe(error)
  })
  it('createSchedule preserves the HTTP contract and backend data', async () => {
    const input: Parameters<typeof api.createSchedule>[0] = {
      name: 'sample',
      description: 'sample',
      taskType: 'sample',
      cronExpression: 'sample',
      timezone: 'sample',
      params: {},
      isEnabled: false,
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.createSchedule(input)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/admin/v1/system/scheduler/schedule',
      input,
    )
  })
  it('createSchedule propagates request failures unchanged', async () => {
    const input: Parameters<typeof api.createSchedule>[0] = {
      name: 'sample',
      description: 'sample',
      taskType: 'sample',
      cronExpression: 'sample',
      timezone: 'sample',
      params: {},
      isEnabled: false,
    }
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.createSchedule(input)).rejects.toBe(error)
  })
  it('updateSchedule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.updateSchedule>[0] = 1
    const input: Parameters<typeof api.updateSchedule>[1] = {
      name: 'sample',
      description: 'sample',
      cronExpression: 'sample',
      timezone: 'sample',
      params: {},
    }
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.put).mockResolvedValue(dataFromServer)
    const result = await api.updateSchedule(id, input)
    expect(result).toBe(dataFromServer)
    expect(request.put).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/scheduler/schedule/${id}`,
      input,
    )
  })
  it('updateSchedule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.updateSchedule>[0] = 1
    const input: Parameters<typeof api.updateSchedule>[1] = {
      name: 'sample',
      description: 'sample',
      cronExpression: 'sample',
      timezone: 'sample',
      params: {},
    }
    const error = new Error('request failed')
    vi.mocked(request.put).mockRejectedValue(error)
    await expect(api.updateSchedule(id, input)).rejects.toBe(error)
  })
  it('setScheduleStatus preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.setScheduleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.setScheduleStatus>[1] = false
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.patch).mockResolvedValue(dataFromServer)
    const result = await api.setScheduleStatus(id, isEnabled)
    expect(result).toBe(dataFromServer)
    expect(request.patch).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/scheduler/schedule/${id}/status`,
      { isEnabled },
    )
  })
  it('setScheduleStatus propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.setScheduleStatus>[0] = 1
    const isEnabled: Parameters<typeof api.setScheduleStatus>[1] = false
    const error = new Error('request failed')
    vi.mocked(request.patch).mockRejectedValue(error)
    await expect(api.setScheduleStatus(id, isEnabled)).rejects.toBe(error)
  })
  it('deleteSchedule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.deleteSchedule>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.delete).mockResolvedValue(dataFromServer)
    const result = await api.deleteSchedule(id)
    expect(result).toBe(dataFromServer)
    expect(request.delete).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/scheduler/schedule/${id}`,
    )
  })
  it('deleteSchedule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.deleteSchedule>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.delete).mockRejectedValue(error)
    await expect(api.deleteSchedule(id)).rejects.toBe(error)
  })
  it('executeSchedule preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.executeSchedule>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.executeSchedule(id)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/scheduler/schedule/${id}/execute`,
      undefined,
    )
  })
  it('executeSchedule propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.executeSchedule>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.executeSchedule(id)).rejects.toBe(error)
  })
  it('retryJob preserves the HTTP contract and backend data', async () => {
    const id: Parameters<typeof api.retryJob>[0] = 1
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.retryJob(id)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      `/api/admin/v1/system/scheduler/job/${id}/retry`,
      undefined,
    )
  })
  it('retryJob propagates request failures unchanged', async () => {
    const id: Parameters<typeof api.retryJob>[0] = 1
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.retryJob(id)).rejects.toBe(error)
  })
})
