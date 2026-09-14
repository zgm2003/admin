import { beforeEach, describe, expect, it, vi } from 'vitest'

import { grantQueueMonitor, QUEUE_MONITOR_UI_URL } from '@/api/system/queueMonitor'
import { ProtocolError } from '@/types/http'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

describe('queue monitor API', () => {
  beforeEach(() => requestMock.mockReset())

  it('uses the same-origin UI path', () => {
    expect(QUEUE_MONITOR_UI_URL).toBe('/api/admin/v1/system/queuemonitor/ui/')
  })

  it('posts for a grant and strictly parses expiresAt', async () => {
    requestMock.mockResolvedValueOnce({ expiresAt: '2026-09-14T12:00:00Z' })
    await expect(grantQueueMonitor()).resolves.toEqual({ expiresAt: '2026-09-14T12:00:00Z' })
    expect(requestMock).toHaveBeenCalledWith({ method: 'POST', url: '/api/admin/v1/system/queuemonitor/grant' })

    requestMock.mockResolvedValueOnce({ expiresAt: 'bad' })
    await expect(grantQueueMonitor()).rejects.toBeInstanceOf(ProtocolError)
    requestMock.mockResolvedValueOnce({ expiresAt: '2026-09-14T12:00:00Z', token: 'leak' })
    await expect(grantQueueMonitor()).rejects.toBeInstanceOf(ProtocolError)
  })
})
