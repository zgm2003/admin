import type { RealtimeEnvelope } from './protocol'

export type ChatRealtimeEvent = Extract<
  RealtimeEnvelope,
  {
    type:
      | 'ai.chat.started.v1'
      | 'ai.chat.delta.v1'
      | 'ai.chat.completed.v1'
      | 'ai.chat.failed.v1'
      | 'ai.chat.canceled.v1'
  }
>

type ChatListener = (event: ChatRealtimeEvent) => void
type ReconnectListener = () => void

const chatListeners = new Set<ChatListener>()
const reconnectListeners = new Set<ReconnectListener>()

export function isChatRealtimeEvent(event: RealtimeEnvelope): event is ChatRealtimeEvent {
  return (
    event.type === 'ai.chat.started.v1' ||
    event.type === 'ai.chat.delta.v1' ||
    event.type === 'ai.chat.completed.v1' ||
    event.type === 'ai.chat.failed.v1' ||
    event.type === 'ai.chat.canceled.v1'
  )
}

export function emitChatRealtime(event: ChatRealtimeEvent): void {
  for (const listener of chatListeners) listener(event)
}

export function subscribeChatRealtime(listener: ChatListener): () => void {
  chatListeners.add(listener)
  return () => chatListeners.delete(listener)
}

export function emitChatReconnect(): void {
  for (const listener of reconnectListeners) listener()
}

export function subscribeChatReconnect(listener: ReconnectListener): () => void {
  reconnectListeners.add(listener)
  return () => reconnectListeners.delete(listener)
}
