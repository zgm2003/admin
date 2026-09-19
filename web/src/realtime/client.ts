import {
  buildRealtimeWebSocketURL,
  requestRealtimeTicket,
  type RealtimeTicket,
} from '@/api/realtime'
import { createCursorStore } from './cursor'
import {
  createLeaderElector,
  followerCheckMilliseconds,
  leaderHeartbeatMilliseconds,
  realtimeChannelName,
} from './leader'
import { parseRealtimeEnvelope, type RealtimeEnvelope } from './protocol'

interface SocketLike {
  onopen: ((event: Event) => void) | null
  onmessage: ((event: MessageEvent<string>) => void) | null
  onclose: ((event: CloseEvent) => void) | null
  send(value: string): void
  close(): void
}
interface ChannelLike {
  onmessage: ((event: MessageEvent<unknown>) => void) | null
  postMessage(value: unknown): void
  close(): void
}
interface Subject {
  platformCode: string
  userId: number
}
interface Handlers {
  event(event: RealtimeEnvelope): void | Promise<void>
  resync(): void | Promise<void>
  urgent(event: RealtimeEnvelope): void | Promise<void>
}
interface Dependencies {
  ticket?: () => Promise<RealtimeTicket>
  socket?: (url: string) => SocketLike
  storage?: Storage
  broadcastChannel?: ((name: string) => ChannelLike) | undefined
  random?: () => number
  location?: URL
}

export class RealtimeRuntime {
  private subject: Subject | null = null
  private handlers: Handlers | null = null
  private socketValue: SocketLike | null = null
  private channel: ChannelLike | null = null
  private timers = new Set<number>()
  private stopped = true
  private leaderRequired = false
  private connecting = false
  private connectionEpoch = 0
  private reconnectScheduled = false
  private resumeScheduled = false
  private attempt = 0
  private seen = new Map<string, true>()
  private elector: ReturnType<typeof createLeaderElector> | null = null
  private readonly dependencies: Dependencies
  constructor(dependencies: Dependencies = {}) {
    this.dependencies = dependencies
  }
  start(subject: Subject, handlers: Handlers): void {
    this.stop()
    this.stopped = false
    this.subject = subject
    this.handlers = handlers
    const storage = this.dependencies.storage ?? localStorage
    this.elector = createLeaderElector({
      platformCode: subject.platformCode,
      userId: subject.userId,
      storage,
    })
    const channelFactory =
      this.dependencies.broadcastChannel ??
      (typeof BroadcastChannel === 'undefined'
        ? undefined
        : (name: string) => new BroadcastChannel(name))
    if (channelFactory === undefined) {
      this.leaderRequired = false
      void this.connect()
      return
    }
    this.leaderRequired = true
    this.channel = channelFactory(realtimeChannelName(subject.platformCode, subject.userId))
    this.channel.onmessage = (message) => {
      void this.receiveBroadcast(message.data)
    }
    this.electOrFollow()
  }
  stop(): void {
    this.stopped = true
    this.connectionEpoch++
    this.connecting = false
    this.reconnectScheduled = false
    this.resumeScheduled = false
    for (const timer of this.timers) window.clearTimeout(timer)
    this.timers.clear()
    this.closeActiveSocket()
    this.channel?.close()
    this.channel = null
    this.elector?.release()
    this.elector = null
    this.subject = null
    this.handlers = null
    this.seen.clear()
  }
  private electOrFollow(): void {
    if (this.stopped || this.elector === null) return
    if (this.elector.tryAcquire()) {
      if (this.socketValue === null && !this.connecting && !this.reconnectScheduled)
        void this.connect()
      this.schedule(() => this.heartbeat(), leaderHeartbeatMilliseconds)
    } else this.schedule(() => this.electOrFollow(), followerCheckMilliseconds)
  }
  private heartbeat(): void {
    if (this.stopped || this.elector === null) return
    if (this.elector.heartbeat()) {
      this.schedule(() => this.heartbeat(), leaderHeartbeatMilliseconds)
      return
    }
    this.closeActiveSocket()
    this.schedule(() => this.electOrFollow(), followerCheckMilliseconds)
  }
  private schedule(fn: () => void, delay: number): void {
    const timer = window.setTimeout(() => {
      this.timers.delete(timer)
      fn()
    }, delay)
    this.timers.add(timer)
  }
  private connectionAllowed(): boolean {
    return (
      !this.stopped &&
      this.subject !== null &&
      (!this.leaderRequired || this.elector?.isLeader() === true)
    )
  }
  private closeActiveSocket(): void {
    const socket = this.socketValue
    this.socketValue = null
    socket?.close()
  }
  private async connect(): Promise<void> {
    if (!this.connectionAllowed() || this.connecting || this.socketValue !== null) return
    this.connecting = true
    const epoch = this.connectionEpoch
    try {
      const ticket = await (this.dependencies.ticket ?? requestRealtimeTicket)()
      if (epoch !== this.connectionEpoch || !this.connectionAllowed()) return
      const url = buildRealtimeWebSocketURL(
        ticket.ticket,
        this.dependencies.location ?? window.location,
      ).toString()
      const socketFactory: (url: string) => SocketLike =
        this.dependencies.socket ?? ((value: string) => new WebSocket(value))
      const socket = socketFactory(url)
      this.socketValue = socket
      socket.onopen = () => {
        if (this.socketValue !== socket || !this.connectionAllowed()) return
        this.attempt = 0
        this.sendResume(socket)
      }
      socket.onmessage = (message: MessageEvent<string>) => {
        if (this.socketValue !== socket) return
        void this.receiveSocket(message.data).catch(() => this.failSocket(socket))
      }
      socket.onclose = () => {
        if (this.socketValue !== socket) return
        this.socketValue = null
        this.reconnect()
      }
    } catch {
      this.reconnect()
    } finally {
      if (epoch === this.connectionEpoch) this.connecting = false
    }
  }
  private failSocket(socket: SocketLike): void {
    if (this.socketValue !== socket) return
    this.socketValue = null
    socket.close()
    this.reconnect()
  }
  private sendResume(socket: SocketLike): void {
    if (this.socketValue !== socket || this.subject === null) return
    const afterSequence = createCursorStore(this.dependencies.storage ?? localStorage).read(
      this.subject.platformCode,
      this.subject.userId,
    )
    try {
      socket.send(
        JSON.stringify({
          type: 'realtime.resume.v1',
          requestId: crypto.randomUUID(),
          data: { afterSequence },
        }),
      )
    } catch {
      this.failSocket(socket)
    }
  }
  private scheduleResume(socket: SocketLike): void {
    if (this.resumeScheduled || this.socketValue !== socket) return
    this.resumeScheduled = true
    this.schedule(() => {
      this.resumeScheduled = false
      this.sendResume(socket)
    }, 250)
  }
  private reconnect(): void {
    if (!this.connectionAllowed() || this.reconnectScheduled) return
    const base = Math.min(30_000, 500 * 2 ** this.attempt++)
    const jitter = Math.floor(base * 0.2 * (this.dependencies.random ?? Math.random)())
    this.reconnectScheduled = true
    this.schedule(() => {
      this.reconnectScheduled = false
      void this.connect()
    }, base + jitter)
  }
  private remember(id: string): void {
    this.seen.set(id, true)
    if (this.seen.size > 2048) this.seen.delete(this.seen.keys().next().value!)
  }
  private async receiveSocket(raw: string): Promise<void> {
    const socket = this.socketValue
    if (socket === null) return
    const event = parseRealtimeEnvelope(JSON.parse(raw) as unknown)
    if (this.seen.has(event.eventId)) return
    await this.dispatch(event, true)
    this.remember(event.eventId)
    this.channel?.postMessage({ schemaVersion: 1, kind: 'event', event })
    if (event.type === 'notification.created.v1' || event.type === 'notification.stateChanged.v1')
      this.scheduleResume(socket)
  }
  private async receiveBroadcast(raw: unknown): Promise<void> {
    if (raw === null || typeof raw !== 'object' || Array.isArray(raw)) return
    const r = raw as Record<string, unknown>
    if (Object.keys(r).length !== 3 || r.schemaVersion !== 1 || r.kind !== 'event') return
    try {
      const event = parseRealtimeEnvelope(r.event)
      if (this.seen.has(event.eventId)) return
      await this.dispatch(event, false)
      this.remember(event.eventId)
    } catch {
      return
    }
  }
  private async dispatch(event: RealtimeEnvelope, leader: boolean): Promise<void> {
    if (this.subject === null || this.handlers === null) return
    if (event.type === 'realtime.resyncRequired.v1') {
      await this.handlers.resync()
      createCursorStore(this.dependencies.storage ?? localStorage).write(
        this.subject.platformCode,
        this.subject.userId,
        event.data.throughSequence,
      )
    } else {
      await this.handlers.event(event)
      if (event.type === 'realtime.resumed.v1')
        createCursorStore(this.dependencies.storage ?? localStorage).write(
          this.subject.platformCode,
          this.subject.userId,
          event.data.throughSequence,
        )
    }
    if (
      leader &&
      event.type === 'notification.created.v1' &&
      event.data.priority === 'urgent' &&
      this.elector?.isLeader() !== false
    )
      await this.handlers.urgent(event)
  }
}
