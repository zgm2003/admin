import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useNotificationStore } from '@/store/notification'
import * as api from '@/api/message/notification'

vi.mock('@/api/message/notification', async (original) => ({
  ...(await original()),
  getNotificationSummary: vi.fn(),
  readNotification: vi.fn(),
  readAllNotifications: vi.fn(),
  deleteNotification: vi.fn(),
}))

describe('notification store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })
  it('coalesces concurrent summary loads', async () => {
    vi.mocked(api.getNotificationSummary).mockResolvedValue({ unreadCount: 2, recent: [] })
    const store = useNotificationStore()
    await Promise.all([store.load(), store.load()])
    expect(api.getNotificationSummary).toHaveBeenCalledTimes(1)
    expect(store.unreadCount).toBe(2)
  })
  it('reset prevents a late response from restoring old account data', async () => {
    let resolve!: (value: api.NotificationSummary) => void
    vi.mocked(api.getNotificationSummary).mockReturnValue(
      new Promise((done) => {
        resolve = done
      }),
    )
    const store = useNotificationStore()
    const pending = store.load()
    store.reset()
    resolve({ unreadCount: 9, recent: [] })
    await pending
    expect(store.unreadCount).toBe(0)
  })
})
