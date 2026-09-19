import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RealtimeRuntime } from '@/realtime/client'

class Socket {
  static instances: Socket[] = []
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent<string>) => void) | null = null
  onclose: (() => void) | null = null
  sent: string[] = []
  closed = false
  url: string
  constructor(url: string) {
    this.url = url
    Socket.instances.push(this)
  }
  send(value: string) {
    this.sent.push(value)
  }
  close() {
    this.closed = true
  }
}

class Channel {
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null
  closed = false
  postMessage = vi.fn()
  close() {
    this.closed = true
  }
}

const notificationEvent = JSON.stringify({
  eventId: '2ec9ca86-e265-4551-a15a-05c333326db0',
  type: 'notification.created.v1',
  requestId: 'r',
  sequence: 5,
  occurredAt: '2026-09-18T12:00:00Z',
  durability: 'durable',
  data: {
    notificationId: 7,
    title: 'Notice',
    summary: 'Body',
    variant: 'info',
    priority: 'normal',
    linkType: 'none',
    link: '',
    publishedAt: '2026-09-18T12:00:00Z',
  },
})

describe('RealtimeRuntime', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    Socket.instances = []
  })
  it('gets one ticket, resumes immediately, and advances cursor only on resumed', async () => {
    const ticket = vi.fn().mockResolvedValue({ ticket: 'raw', expiresAt: '2026-09-18T12:00:00Z' })
    const runtime = new RealtimeRuntime({
      ticket,
      socket: (url) => new Socket(url),
      storage: localStorage,
      broadcastChannel: undefined,
      random: () => 0,
    })
    runtime.start(
      { platformCode: 'admin', userId: 1 },
      { event: vi.fn(), resync: vi.fn(), urgent: vi.fn() },
    )
    await vi.runAllTicks()
    const socket = Socket.instances[0]!
    socket.onopen?.()
    expect(ticket).toHaveBeenCalledTimes(1)
    expect(JSON.parse(socket.sent[0]!).type).toBe('realtime.resume.v1')
    socket.onmessage?.(
      new MessageEvent('message', {
        data: JSON.stringify({
          eventId: '2ec9ca86-e265-4551-a15a-05c333326db0',
          type: 'realtime.resumed.v1',
          requestId: 'r',
          sequence: 5,
          occurredAt: '2026-09-18T12:00:00Z',
          durability: 'durable',
          data: { throughSequence: 5 },
        }),
      }),
    )
    await vi.waitFor(() =>
      expect(localStorage.getItem('admin:realtime:cursor:v1:admin:1')).not.toBeNull(),
    )
    runtime.stop()
    expect(
      JSON.parse(localStorage.getItem('admin:realtime:cursor:v1:admin:1')!).throughSequence,
    ).toBe(5)
    expect(socket.closed).toBe(true)
  })

  it('keeps one socket while the leader heartbeat renews its lease', async () => {
    const ticket = vi.fn().mockResolvedValue({ ticket: 'raw', expiresAt: '2026-09-18T12:00:00Z' })
    const runtime = new RealtimeRuntime({
      ticket,
      socket: (url) => new Socket(url),
      storage: localStorage,
      broadcastChannel: () => new Channel(),
      random: () => 0,
    })
    runtime.start(
      { platformCode: 'admin', userId: 1 },
      { event: vi.fn(), resync: vi.fn(), urgent: vi.fn() },
    )
    await vi.runAllTicks()

    await vi.advanceTimersByTimeAsync(8_000)

    expect(ticket).toHaveBeenCalledTimes(1)
    expect(Socket.instances).toHaveLength(1)
    runtime.stop()
  })

  it('retries a failed event handler and ignores a stale socket close', async () => {
    const event = vi
      .fn()
      .mockRejectedValueOnce(new Error('store failed'))
      .mockResolvedValueOnce(undefined)
    const ticket = vi.fn().mockResolvedValue({ ticket: 'raw', expiresAt: '2026-09-18T12:00:00Z' })
    const runtime = new RealtimeRuntime({
      ticket,
      socket: (url) => new Socket(url),
      storage: localStorage,
      broadcastChannel: () => new Channel(),
      random: () => 0,
    })
    runtime.start({ platformCode: 'admin', userId: 1 }, { event, resync: vi.fn(), urgent: vi.fn() })
    await vi.runAllTicks()
    const first = Socket.instances[0]!

    first.onmessage?.(new MessageEvent('message', { data: notificationEvent }))
    await vi.waitFor(() => expect(first.closed).toBe(true))
    await vi.advanceTimersByTimeAsync(500)
    expect(Socket.instances).toHaveLength(2)

    const second = Socket.instances[1]!
    first.onclose?.()
    await vi.advanceTimersByTimeAsync(1_000)
    expect(Socket.instances).toHaveLength(2)

    second.onmessage?.(new MessageEvent('message', { data: notificationEvent }))
    await vi.runAllTicks()
    await Promise.resolve()
    expect(event).toHaveBeenCalledTimes(2)
    runtime.stop()
  })

  it('coalesces live events into a server-confirmed resume without advancing the cursor early', async () => {
    const runtime = new RealtimeRuntime({
      ticket: vi.fn().mockResolvedValue({
        ticket: 'raw',
        expiresAt: '2026-09-18T12:00:00Z',
      }),
      socket: (url) => new Socket(url),
      storage: localStorage,
      broadcastChannel: undefined,
      random: () => 0,
    })
    runtime.start(
      { platformCode: 'admin', userId: 1 },
      { event: vi.fn(), resync: vi.fn(), urgent: vi.fn() },
    )
    await vi.runAllTicks()
    const socket = Socket.instances[0]!
    socket.onopen?.()
    socket.onmessage?.(new MessageEvent('message', { data: notificationEvent }))
    socket.onmessage?.(
      new MessageEvent('message', {
        data: notificationEvent.replace(
          '2ec9ca86-e265-4551-a15a-05c333326db0',
          '3ec9ca86-e265-4551-a15a-05c333326db0',
        ),
      }),
    )
    await vi.runAllTicks()
    await Promise.resolve()

    expect(localStorage.getItem('admin:realtime:cursor:v1:admin:1')).toBeNull()
    await vi.advanceTimersByTimeAsync(249)
    expect(socket.sent).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(socket.sent).toHaveLength(2)
    expect(JSON.parse(socket.sent[1]!).data.afterSequence).toBe(0)
    runtime.stop()
  })

  it('processes socket frames serially before advancing the resumed cursor', async () => {
    let releaseEvent: (() => void) | undefined
    const blocked = new Promise<void>((resolve) => {
      releaseEvent = resolve
    })
    const event = vi.fn().mockReturnValue(blocked)
    const runtime = new RealtimeRuntime({
      ticket: vi.fn().mockResolvedValue({ ticket: 'raw', expiresAt: '2026-09-18T12:00:00Z' }),
      socket: (url) => new Socket(url),
      storage: localStorage,
      broadcastChannel: undefined,
    })
    runtime.start({ platformCode: 'admin', userId: 1 }, { event, resync: vi.fn(), urgent: vi.fn() })
    await vi.runAllTicks()
    const socket = Socket.instances[0]!
    socket.onmessage?.(new MessageEvent('message', { data: notificationEvent }))
    socket.onmessage?.(
      new MessageEvent('message', {
        data: JSON.stringify({
          eventId: '3ec9ca86-e265-4551-a15a-05c333326db0',
          type: 'realtime.resumed.v1',
          requestId: 'resume',
          sequence: 5,
          occurredAt: '2026-09-18T12:00:00Z',
          durability: 'durable',
          data: { throughSequence: 5 },
        }),
      }),
    )
    await vi.runAllTicks()
    expect(localStorage.getItem('admin:realtime:cursor:v1:admin:1')).toBeNull()

    releaseEvent?.()
    await vi.waitFor(() =>
      expect(
        JSON.parse(localStorage.getItem('admin:realtime:cursor:v1:admin:1')!).throughSequence,
      ).toBe(5),
    )
    runtime.stop()
  })

  it('deduplicates frames that arrive while the first matching handler is pending', async () => {
    let releaseEvent: (() => void) | undefined
    const blocked = new Promise<void>((resolve) => {
      releaseEvent = resolve
    })
    const event = vi.fn().mockReturnValue(blocked)
    const runtime = new RealtimeRuntime({
      ticket: vi.fn().mockResolvedValue({ ticket: 'raw', expiresAt: '2026-09-18T12:00:00Z' }),
      socket: (url) => new Socket(url),
      storage: localStorage,
      broadcastChannel: undefined,
    })
    runtime.start({ platformCode: 'admin', userId: 1 }, { event, resync: vi.fn(), urgent: vi.fn() })
    await vi.runAllTicks()
    const socket = Socket.instances[0]!
    socket.onmessage?.(new MessageEvent('message', { data: notificationEvent }))
    socket.onmessage?.(new MessageEvent('message', { data: notificationEvent }))
    await vi.runAllTicks()
    expect(event).toHaveBeenCalledTimes(1)

    releaseEvent?.()
    await vi.runAllTicks()
    expect(event).toHaveBeenCalledTimes(1)
    runtime.stop()
  })

  it('drops pending cursor and urgent side effects after the authenticated subject changes', async () => {
    let releaseEvent: (() => void) | undefined
    const blocked = new Promise<void>((resolve) => {
      releaseEvent = resolve
    })
    const oldUrgent = vi.fn()
    const runtime = new RealtimeRuntime({
      ticket: vi.fn().mockResolvedValue({ ticket: 'raw', expiresAt: '2026-09-18T12:00:00Z' }),
      socket: (url) => new Socket(url),
      storage: localStorage,
      broadcastChannel: undefined,
    })
    runtime.start(
      { platformCode: 'admin', userId: 1 },
      { event: vi.fn().mockReturnValue(blocked), resync: vi.fn(), urgent: oldUrgent },
    )
    await vi.runAllTicks()
    const oldSocket = Socket.instances[0]!
    oldSocket.onmessage?.(
      new MessageEvent('message', { data: notificationEvent.replace('"normal"', '"urgent"') }),
    )
    await vi.runAllTicks()

    runtime.start(
      { platformCode: 'admin', userId: 2 },
      { event: vi.fn(), resync: vi.fn(), urgent: vi.fn() },
    )
    releaseEvent?.()
    await vi.runAllTicks()

    expect(oldUrgent).not.toHaveBeenCalled()
    expect(localStorage.getItem('admin:realtime:cursor:v1:admin:1')).toBeNull()
    runtime.stop()
  })
})
