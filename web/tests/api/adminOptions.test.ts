// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getSessionOptions } from '@/api/user/sessionOptions'
import { getLoginLogOptions } from '@/api/user/loginLogOptions'
import { getNotificationTaskOptions } from '@/api/message/notificationTaskOptions'
import { getNotificationOptions } from '@/api/message/notificationOptions'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({ request: { get: vi.fn() } }))
beforeEach(() => vi.resetAllMocks())

describe('backend-owned admin options', () => {
  it.each([
    [getSessionOptions, '/api/admin/v1/user/session/options'],
    [getLoginLogOptions, '/api/admin/v1/user/loginlog/options'],
    [getNotificationTaskOptions, '/api/admin/v1/message/notificationtask/options'],
    [getNotificationOptions, '/api/v1/message/notification/options'],
  ] as const)('passes the options DTO through from %s', async (load, url) => {
    const options = { serverAdded: true, statuses: [{ value: 99, label: 'Future state' }] }
    vi.mocked(request.get).mockResolvedValue(options)
    await expect(load()).resolves.toBe(options)
    expect(request.get).toHaveBeenCalledExactlyOnceWith(url)
  })
})
