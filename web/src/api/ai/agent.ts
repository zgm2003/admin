import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'
import type { JsonObject, Option } from './tool'
export interface AgentItem {
  id: string
  code: string
  name: string
  description: string
  avatarFileId: string | null
  capabilities: string[]
  systemPrompt: string
  multiplierPpm: number
  activeBindingId: string | null
  generationParams: JsonObject
  status: number
  updatedAt: string
}
export interface BindingInput {
  id?: string
  providerModelId: string
  textProtocol: number
  parameters: JsonObject
  status: number
}
export interface AgentBinding extends BindingInput {
  id: string
  agentId: string
}
export interface ToolBindingInput {
  toolId: string
  exposedName: string
  parameterConstraints: JsonObject
  status: number
}
export interface AgentToolBinding extends ToolBindingInput {
  id: string
  agentId: string
}
export interface AgentUpdateInput {
  name: string
  description: string
  avatarFileId: string | null
  capabilities: string[]
  systemPrompt: string
  multiplierPpm: number
  generationParams: JsonObject
  bindings: BindingInput[]
  tools: ToolBindingInput[]
}
export interface AgentCreateInput extends AgentUpdateInput {
  code: string
  status: number
  activeProviderModelId: string | null
}
export interface AgentDetail {
  agent: AgentItem
  bindings: AgentBinding[]
  tools: AgentToolBinding[]
}
export interface AgentOptions {
  statuses: Option<number>[]
  capabilities: Option<string>[]
  textProtocols: Option<number>[]
  defaults: { status: number; multiplierPpm: number; textProtocol: number }
  constraints: {
    maxPageSize: number
    maxPromptBytes: number
    maxBoundTools: number
    maxToolCalls: number
    minMultiplierPpm: number
    maxMultiplierPpm: number
  }
}
export interface AgentCandidate {
  id: string
  name: string
  valid: boolean
  operations: string[]
  limits: JsonObject
}
export interface CandidateQuery extends PageRequest {
  kind: 'providerModel' | 'tool'
  keyword?: string
}
const base = '/api/admin/v1/ai/agent'
export function getAgents(query: PageRequest): Promise<PageResult<AgentItem>> {
  return request.get<PageResult<AgentItem>>(base, { params: query })
}
export function getAgent(id: string): Promise<AgentDetail> {
  return request.get<AgentDetail>(`${base}/${id}`)
}
export function getAgentOptions(): Promise<AgentOptions> {
  return request.get<AgentOptions>(`${base}/options`)
}
export function getAgentCandidates(
  id: string | null,
  query: CandidateQuery,
): Promise<PageResult<AgentCandidate>> {
  return request.get<PageResult<AgentCandidate>>(
    id === null ? `${base}/candidate/create` : `${base}/${id}/candidate/update`,
    { params: query },
  )
}
function configuration(input: AgentUpdateInput): AgentUpdateInput {
  return {
    name: input.name,
    description: input.description,
    avatarFileId: input.avatarFileId,
    capabilities: input.capabilities,
    systemPrompt: input.systemPrompt,
    multiplierPpm: input.multiplierPpm,
    generationParams: input.generationParams,
    bindings: input.bindings,
    tools: input.tools,
  }
}
export function createAgent(input: AgentCreateInput): Promise<{ id: string }> {
  return request.post<{ id: string }, AgentCreateInput>(base, {
    ...configuration(input),
    code: input.code,
    status: input.status,
    activeProviderModelId: input.activeProviderModelId,
  })
}
export function updateAgent(id: string, input: AgentUpdateInput): Promise<Record<string, never>> {
  return request.put<Record<string, never>, AgentUpdateInput>(`${base}/${id}`, configuration(input))
}
export function updateAgentStatus(id: string, status: number): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { status: number }>(`${base}/${id}/status`, {
    status,
  })
}
export function switchAgentBinding(id: string, bindingId: string): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { bindingId: string }>(
    `${base}/${id}/binding/current`,
    { bindingId },
  )
}
export function deleteAgent(id: string): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`${base}/${id}`)
}
