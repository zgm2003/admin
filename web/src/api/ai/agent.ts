import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface AgentActions {
  update: boolean
  status: boolean
  delete: boolean
}

export interface AiAgent {
  id: number
  name: string
  description: string
  avatar: string
  systemPrompt: string
  scene: number
  sceneName: string
  type: number
  typeName: string
  providerModelId: number
  timeoutMs: number
  providerName: string
  modelName: string
  modelKey: string
  modelAvailable: boolean
  status: number
  statusName: string
  version: number
  actions: AgentActions
  createdAt: string
  updatedAt: string
}

export interface AgentQuery extends PageRequest {
  keyword?: string
  type?: number
  scene?: number
  status?: number
}

export interface AgentOption {
  value: number
  label: string
}

export interface AgentOptions {
  types: AgentOption[]
  scenes: AgentOption[]
  statuses: AgentOption[]
}

export interface AgentModelOption {
  value: number
  label: string
  modelKey: string
  modelKind: number
}

export interface AgentProviderModelOption {
  value: number
  label: string
  children: AgentModelOption[]
}

export interface AgentModelOptions {
  providers: AgentProviderModelOption[]
}

export interface CreateAgentInput {
  name: string
  description: string
  avatar: string
  systemPrompt: string
  scene: number
  type: number
  providerModelId: number
  timeoutMs: number
  status: number
}

export type UpdateAgentInput = CreateAgentInput

const base = '/api/admin/v1/ai/agent'

export function getAgents(query: AgentQuery): Promise<PageResult<AiAgent>> {
  return request.get<PageResult<AiAgent>>(base, { params: query })
}

export function getAgent(id: number): Promise<AiAgent> {
  return request.get<AiAgent>(`${base}/${id}`)
}

export function getAgentOptions(): Promise<AgentOptions> {
  return request.get<AgentOptions>(`${base}/options`)
}

export function getAgentModelOptions(): Promise<AgentModelOptions> {
  return request.get<AgentModelOptions>(`${base}/model-options`)
}

export function createAgent(input: CreateAgentInput): Promise<{ id: number }> {
  return request.post<{ id: number }, CreateAgentInput>(base, input)
}

export function updateAgent(id: number, input: UpdateAgentInput): Promise<Record<string, never>> {
  return request.put<Record<string, never>, UpdateAgentInput>(`${base}/${id}`, input)
}

export function updateAgentStatus(id: number, status: number): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { status: number }>(`${base}/${id}/status`, {
    status,
  })
}

export function deleteAgent(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`${base}/${id}`)
}
