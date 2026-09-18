import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from '@/utils/request'
import { requestObjectURL, requestUploadCredentials } from '@/api/storage/upload'
import { ProtocolError } from '@/types/http'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

describe('storage upload API', () => {
  beforeEach(() => requestMock.mockReset())

  it('requests credentials with the unified rule code and file metadata', async () => {
    const result = {
      items: [
        {
          uploadUrl: 'https://cos.example/upload',
          objectKey: 'avatar/2026/08/30/a.png',
          method: 'PUT' as const,
          headers: { 'Content-Type': 'image/png' },
          expiresAt: '2026-08-30T00:10:00Z',
          publicUrl: 'https://cdn.example/avatar/2026/08/30/a.png',
        },
      ],
    }
    requestMock.mockResolvedValue(result)

    await expect(
      requestUploadCredentials('avatar', [
        { fileName: 'a.png', contentType: 'image/png', fileSizeBytes: 10 },
      ]),
    ).resolves.toEqual(result)
    expect(requestMock).toHaveBeenCalledWith({
      method: 'POST',
      url: '/api/v1/storage/upload-credential',
      data: {
        ruleCode: 'avatar',
        files: [{ fileName: 'a.png', contentType: 'image/png', fileSizeBytes: 10 }],
      },
    })
  })

  it('parses public and private object URLs using only the object key', async () => {
    requestMock.mockResolvedValueOnce({ url: 'https://cdn.example/object.png', expiresAt: null })
    await expect(requestObjectURL('object-key')).resolves.toEqual({
      url: 'https://cdn.example/object.png',
      expiresAt: null,
    })
    expect(requestMock).toHaveBeenLastCalledWith({
      method: 'POST',
      url: '/api/v1/storage/object-url',
      data: { objectKey: 'object-key' },
    })

    requestMock.mockResolvedValueOnce({
      url: 'https://cos.example/signed',
      expiresAt: '2026-09-17T00:10:00Z',
    })
    await expect(requestObjectURL('private-key')).resolves.toEqual({
      url: 'https://cos.example/signed',
      expiresAt: '2026-09-17T00:10:00Z',
    })
  })

  it('rejects malformed object URL responses', async () => {
    for (const value of [
      { url: '', expiresAt: null },
      { url: 'https://cdn.example/object.png' },
      { url: 'https://cdn.example/object.png', expiresAt: 'not-a-date' },
      { url: 'https://cdn.example/object.png', expiresAt: null, generation: 1 },
    ]) {
      requestMock.mockResolvedValueOnce(value)
      await expect(requestObjectURL('object-key')).rejects.toThrow(ProtocolError)
    }
  })
})
