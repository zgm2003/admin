import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'
export type JsonValue =
  null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue }
export type JsonObject = { [key: string]: JsonValue }
export interface ToolItem {
  id: string
  code: string
  name: string
  type: number
  currentVersion: number
  status: number
  createdAt: string
  updatedAt: string
}
export interface ToolVersion {
  id: string
  toolId: string
  version: number
  type: number
  description: string
  inputSchema: JsonObject
  outputSchema: JsonObject
  builtinKey: string | null
  httpDefinition: JsonObject | null
  timeoutMs: number
  outputLimitBytes: number
  hasSideEffect: number
  idempotencyMode: number
}
export interface VersionInput {
  type: number
  description: string
  inputSchema: JsonObject
  outputSchema: JsonObject
  builtinKey: string
  httpDefinition: JsonObject | null
  secretHeaders?: Record<string, string>
  timeoutMs: number
  outputLimitBytes: number
  hasSideEffect: number
  idempotencyMode: number
}
export interface ToolCreateInput {
  code: string
  name: string
  status: number
  version: VersionInput
}
export interface ToolUpdateInput {
  name: string
  version: VersionInput
}
export interface Option<T> {
  value: T
  label: string
}
export interface ToolOptions {
  statuses: Option<number>[]
  types: Option<number>[]
  builtins: Option<string>[]
  idempotencyModes: Option<number>[]
  defaults: { status: number; timeoutMs: number; outputLimitBytes: number }
  constraints: {
    maxPageSize: number
    minTimeoutMs: number
    maxTimeoutMs: number
    maxOutputLimitBytes: number
    maxSchemaBytes: number
    maxSchemaDepth: number
  }
}
const base = '/api/admin/v1/ai/tool'
export function getTools(query: PageRequest): Promise<PageResult<ToolItem>> {
  return request.get<PageResult<ToolItem>>(base, { params: query })
}
export function getTool(id: string): Promise<{ tool: ToolItem; version: ToolVersion }> {
  return request.get<{ tool: ToolItem; version: ToolVersion }>(`${base}/${id}`)
}
export function getToolOptions(): Promise<ToolOptions> {
  return request.get<ToolOptions>(`${base}/options`)
}
export function createTool(input: ToolCreateInput): Promise<{ id: string }> {
  return request.post<{ id: string }, ToolCreateInput>(base, {
    code: input.code,
    name: input.name,
    status: input.status,
    version: input.version,
  })
}
export function updateTool(id: string, input: ToolUpdateInput): Promise<Record<string, never>> {
  return request.put<Record<string, never>, ToolUpdateInput>(`${base}/${id}`, {
    name: input.name,
    version: input.version,
  })
}
export function updateToolStatus(id: string, status: number): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { status: number }>(`${base}/${id}/status`, {
    status,
  })
}
export function deleteTool(id: string): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`${base}/${id}`)
}
