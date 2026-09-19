import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createLeaderElector, leaderLeaseKey, realtimeChannelName } from '@/realtime/leader'

describe('realtime leader', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
  })
  it('uses fixed isolated names and only one lease owner', () => {
    expect(realtimeChannelName('admin', 1)).toBe('admin:realtime:v1:admin:1')
    expect(leaderLeaseKey('admin', 1)).toBe('admin:realtime:leader:v1:admin:1')
    const first = createLeaderElector({
      platformCode: 'admin',
      userId: 1,
      storage: localStorage,
      now: () => Date.now(),
      randomId: () => 'first',
    })
    const second = createLeaderElector({
      platformCode: 'admin',
      userId: 1,
      storage: localStorage,
      now: () => Date.now(),
      randomId: () => 'second',
    })
    expect(first.tryAcquire()).toBe(true)
    expect(second.tryAcquire()).toBe(false)
    first.release()
    expect(second.tryAcquire()).toBe(true)
  })
  it('removes corrupt lease and permits takeover after six seconds', () => {
    const key = leaderLeaseKey('admin', 1)
    localStorage.setItem(key, 'bad')
    const first = createLeaderElector({
      platformCode: 'admin',
      userId: 1,
      storage: localStorage,
      now: () => Date.now(),
      randomId: () => 'first',
    })
    expect(first.tryAcquire()).toBe(true)
    const second = createLeaderElector({
      platformCode: 'admin',
      userId: 1,
      storage: localStorage,
      now: () => Date.now(),
      randomId: () => 'second',
    })
    vi.advanceTimersByTime(6001)
    expect(second.tryAcquire()).toBe(true)
  })
})
