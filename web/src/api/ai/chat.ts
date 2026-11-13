import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface ChatAgent {
  id: number
  name: string
  description: string
  scene: number
  providerName: string
  modelName: string
  timeoutMs: number
}

export interface ChatConversation {
  id: number
  agentId: number
  agentName: string
  title: string
  modelName: string
  messageCount: number
  lastMessageAt: string
  updatedAt: string
}

export interface ChatMessageUsage {
  inputTokens: number
  cachedInputTokens: number
  outputTokens: number
  amount: string
}

export interface ChatMessage {
  id: number
  seq: number
  role: number
  content: string
  status: number
  runId: number | null
  deliverySeq: number
  createdAt: string
  usage?: ChatMessageUsage
}

export interface ChatActiveRun {
  id: number
  requestId: string
  status: number
  billingStatus: number
  assistantMessageId: number
  deliverySeq: number
}

export interface ChatMessagePage {
  list: ChatMessage[]
  hasMore: boolean
  activeRun?: ChatActiveRun
}

export interface ChatBalance {
  balanceUnits: number
  balance: string
  canSend: boolean
}

export interface SendMessageInput {
  content: string
  requestId: string
}

export interface SendMessageResult {
  runId: number
  userMessage: ChatMessage
  assistantMessage: ChatMessage
}

export interface ChatRun {
  id: number
  requestId: string
  userId: number
  username: string
  conversationId: number
  agentId: number
  agentName: string
  providerName: string
  modelKey: string
  modelName: string
  status: number
  billingStatus: number
  errorCode: string
  errorMessage: string
  inputTokens: number
  cachedInputTokens: number
  outputTokens: number
  amount: string
  amountUnits: number
  balanceBefore: string
  balanceAfter: string
  firstTokenMs: number | null
  durationMs: number | null
  createdAt: string
  startedAt: string | null
  finishedAt: string | null
}

export interface ChatRunEvent {
  seq: number
  type: string
  message: string
  createdAt: string
}

export interface ChatRunDetail extends ChatRun {
  events: ChatRunEvent[]
}

export interface ChatRunQuery extends PageRequest {
  keyword?: string
  userId?: number
  status?: number
}

const base = '/api/admin/v1/ai/chat'

export function getChatAgents(): Promise<{ list: ChatAgent[] }> {
  return request.get<{ list: ChatAgent[] }>(`${base}/agents`)
}

export function getChatBalance(): Promise<ChatBalance> {
  return request.get<ChatBalance>(`${base}/balance`)
}

export function getConversations(
  query: PageRequest & { keyword?: string },
): Promise<PageResult<ChatConversation>> {
  return request.get<PageResult<ChatConversation>>(`${base}/conversations`, { params: query })
}

export function createConversation(input: {
  agentId: number
  title?: string
}): Promise<{ id: number }> {
  return request.post<{ id: number }, { agentId: number; title?: string }>(
    `${base}/conversations`,
    input,
  )
}

export function renameConversation(id: number, title: string): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { title: string }>(`${base}/conversations/${id}`, {
    title,
  })
}

export function deleteConversation(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`${base}/conversations/${id}`)
}

export function getMessages(
  id: number,
  query: { beforeSeq?: number; limit?: number } = {},
): Promise<ChatMessagePage> {
  return request.get<ChatMessagePage>(`${base}/conversations/${id}/messages`, { params: query })
}

export function sendMessage(id: number, input: SendMessageInput): Promise<SendMessageResult> {
  return request.post<SendMessageResult, SendMessageInput>(
    `${base}/conversations/${id}/messages`,
    input,
  )
}

export function cancelRun(id: number): Promise<Record<string, never>> {
  return request.post<Record<string, never>>(`${base}/runs/${id}/cancel`, undefined)
}

export function getRuns(query: ChatRunQuery, viewAll: boolean): Promise<PageResult<ChatRun>> {
  return request.get<PageResult<ChatRun>>(`${base}${viewAll ? '/manage/runs' : '/runs'}`, {
    params: query,
  })
}

export function getRunDetail(id: number, viewAll: boolean): Promise<ChatRunDetail> {
  return request.get<ChatRunDetail>(`${base}${viewAll ? '/manage/runs' : '/runs'}/${id}`)
}
