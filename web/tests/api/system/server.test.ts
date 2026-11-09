import { beforeEach, expect, it, vi } from 'vitest'
import { getServerSnapshot } from '@/api/system/server'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: { get: vi.fn() } }))
beforeEach(() => vi.clearAllMocks())

it('uses the protected same-origin server endpoint and preserves the returned DTO', async () => {
  const snapshot = { hostname: 'node-a', cpuPercent: 12.5 }
  vi.mocked(request.get).mockResolvedValue(snapshot)
  expect(await getServerSnapshot()).toBe(snapshot)
  expect(request.get).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/system/server')
})

it('propagates the request failure without a fabricated snapshot', async () => {
  const error = new Error('unavailable')
  vi.mocked(request.get).mockRejectedValue(error)
  await expect(getServerSnapshot()).rejects.toBe(error)
})
