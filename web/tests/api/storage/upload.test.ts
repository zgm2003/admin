// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/storage/upload'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
  refreshAccessCredential: vi.fn(),
}))
beforeEach(() => vi.resetAllMocks())
describe('thin API HTTP contract', () => {
  it('requestUploadCredentials preserves the HTTP contract and backend data', async () => {
    const ruleCode: Parameters<typeof api.requestUploadCredentials>[0] = 'sample'
    const files: Parameters<typeof api.requestUploadCredentials>[1] = [
      { fileName: 'sample', contentType: 'sample', fileSizeBytes: 1 },
    ]
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.requestUploadCredentials(ruleCode, files)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/v1/storage/upload-credential', {
      ruleCode,
      files,
    })
  })
  it('requestUploadCredentials propagates request failures unchanged', async () => {
    const ruleCode: Parameters<typeof api.requestUploadCredentials>[0] = 'sample'
    const files: Parameters<typeof api.requestUploadCredentials>[1] = [
      { fileName: 'sample', contentType: 'sample', fileSizeBytes: 1 },
    ]
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.requestUploadCredentials(ruleCode, files)).rejects.toBe(error)
  })
  it('requestObjectURL preserves the HTTP contract and backend data', async () => {
    const controller = new AbortController()
    const objectKey: Parameters<typeof api.requestObjectURL>[0] = 'sample'
    const signal: Parameters<typeof api.requestObjectURL>[1] = controller.signal
    const dataFromServer = { id: 7, serverAdded: { label: 'new value', numericValue: 99 } }
    vi.mocked(request.post).mockResolvedValue(dataFromServer)
    const result = await api.requestObjectURL(objectKey, signal)
    expect(result).toBe(dataFromServer)
    expect(request.post).toHaveBeenCalledExactlyOnceWith(
      '/api/v1/storage/object-url',
      { objectKey },
      { ...(signal ? { signal } : {}) },
    )
  })
  it('requestObjectURL propagates request failures unchanged', async () => {
    const controller = new AbortController()
    const objectKey: Parameters<typeof api.requestObjectURL>[0] = 'sample'
    const signal: Parameters<typeof api.requestObjectURL>[1] = controller.signal
    const error = new Error('request failed')
    vi.mocked(request.post).mockRejectedValue(error)
    await expect(api.requestObjectURL(objectKey, signal)).rejects.toBe(error)
  })
})
