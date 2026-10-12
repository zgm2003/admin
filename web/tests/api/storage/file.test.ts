// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import { request } from '@/utils/request'
import * as api from '@/api/storage/file'
vi.mock('@/utils/request', () => ({ request: { get: vi.fn(), post: vi.fn(), delete: vi.fn() } }))
beforeEach(() => vi.clearAllMocks())
it('preserves backend file pagination and forwards filters without owner input', async () => {
  const data = { list: [], total: 3, page: 2, pageSize: 20 }
  vi.mocked(request.get).mockResolvedValue(data)
  expect(await api.listFiles({ page: 2, pageSize: 20, keyword: 'a', category: 'image' })).toBe(data)
  expect(request.get).toHaveBeenCalledWith('/api/v1/storage/file', {
    params: { page: 2, pageSize: 20, keyword: 'a', category: 'image' },
  })
})
it('completes a single file with cancellation signal and returns confirmed key', async () => {
  const signal = new AbortController().signal
  const data = { fileId: 8, objectKey: 'final/key' }
  vi.mocked(request.post).mockResolvedValue(data)
  expect(await api.completeFileUpload(8, signal)).toBe(data)
  expect(request.post).toHaveBeenCalledWith('/api/v1/storage/file/8/complete', undefined, {
    signal,
  })
})
it('completes without a JSON request body when no signal is supplied', async () => {
  const data = { fileId: 8, objectKey: 'final/key' }
  vi.mocked(request.post).mockResolvedValue(data)
  expect(await api.completeFileUpload(8)).toBe(data)
  expect(request.post).toHaveBeenCalledWith('/api/v1/storage/file/8/complete', undefined, {})
})
it('requests an attachment URL separately from delete acceptance', async () => {
  await api.getFileURL(8, 'attachment')
  await api.deleteFile(8)
  expect(request.post).toHaveBeenCalledWith('/api/v1/storage/file/8/url', {
    disposition: 'attachment',
  })
  expect(request.delete).toHaveBeenCalledWith('/api/v1/storage/file/8')
})
