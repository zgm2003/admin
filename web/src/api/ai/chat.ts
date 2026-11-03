import { request } from '@/utils/request'
export interface CursorInput {
  beforeId?: string
  limit?: number
}
export interface CursorPage<T> {
  list: T[]
  hasMore: boolean
  nextBeforeId: string
}
export interface SequencePage<T> {
  list: T[]
  hasMore: boolean
  nextSequence: number
  resetReason?: string
}
export interface Conversation {
  id: string
  agentId: string
  title: string
  updatedAt: string
}
export interface Message {
  id: string
  role: number
  content: unknown
  sequence: number
  createdAt: string
}
export interface ParameterOption {
  operation: string
  key: string
  label: string
  options: Array<{ value: string | number; label: string }>
  min?: number
  max?: number
}
export interface AgentOption {
  id: string
  name: string
  capabilities: string[]
  operations: string[]
  parameters: Record<string, unknown>
  generationParams: Record<string, unknown>
  limits: Record<string, unknown>
  parameterOptions: ParameterOption[]
}
const base = '/api/admin/v1/ai/chat'
export function listConversations(params: CursorInput = {}) {
  return request.get<CursorPage<Conversation>>(`${base}/conversation`, { params })
}
export function createConversation(input: { agentId: string; title: string }) {
  return request.post<Conversation, typeof input>(`${base}/conversation`, input)
}
export function renameConversation(id: string, title: string) {
  return request.put<Conversation, { title: string }>(`${base}/conversation/${id}`, { title })
}
export function deleteConversation(id: string) {
  return request.delete<{ accepted: boolean }>(`${base}/conversation/${id}`)
}
export function listAgentOptions(params: CursorInput & { keyword?: string } = {}) {
  return request.get<CursorPage<AgentOption>>(`${base}/agent`, { params })
}
export function getConversation(id: string) {
  return request.get<Conversation>(`${base}/conversation/${id}`)
}
export function listMessages(id: string, params: { afterSequence?: number; limit?: number } = {}) {
  return request.get<SequencePage<Message>>(`${base}/conversation/${id}/message`, { params })
}
