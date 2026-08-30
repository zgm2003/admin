import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@src/utils/request'
import { requestUploadCredentials } from '@src/api/storage/upload'

vi.mock('@src/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

describe('storage upload API', () => {
  beforeEach(() => requestMock.mockReset())

  it('requests credentials with the unified rule code and file metadata', async () => {
    const result = { items: [{ uploadUrl: 'https://cos.example/upload', objectKey: 'avatar/2026/08/30/a.png', method: 'PUT' as const, headers: { 'Content-Type': 'image/png' }, expiresAt: '2026-08-30T00:10:00Z', publicUrl: 'https://cdn.example/avatar/2026/08/30/a.png' }] }
    requestMock.mockResolvedValue(result)

    await expect(requestUploadCredentials('avatar', [{ fileName: 'a.png', contentType: 'image/png', fileSizeBytes: 10 }])).resolves.toEqual(result)
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/storage/upload-credentials',
      data: { ruleCode: 'avatar', files: [{ fileName: 'a.png', contentType: 'image/png', fileSizeBytes: 10 }] },
    })
  })
})
