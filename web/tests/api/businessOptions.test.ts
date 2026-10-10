// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getMailOptions } from '@/api/message/mail'
import { getSmsOptions } from '@/api/message/sms'
import { getSchedulerOptions } from '@/api/system/scheduler'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: { get: vi.fn() } }))
beforeEach(() => vi.resetAllMocks())

describe('business options thin API contracts', () => {
  it.each([
    ['message/mail', getMailOptions],
    ['message/sms', getSmsOptions],
    ['system/scheduler', getSchedulerOptions],
  ] as const)('preserves %s backend additions and errors unchanged', async (module, load) => {
    const result = { serverAdded: 99, labels: ['backend'] }
    vi.mocked(request.get).mockResolvedValueOnce(result)
    expect(await load()).toBe(result)
    expect(request.get).toHaveBeenCalledWith(`/api/admin/v1/${module}/options`)
    const error = new Error('fixture unavailable')
    vi.mocked(request.get).mockRejectedValueOnce(error)
    await expect(load()).rejects.toBe(error)
  })
})
