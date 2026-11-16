import { request } from '@/utils/request'

export interface AiTool {
  id: number
  code: string
  apiName: string
  name: string
  description: string
  category: string
  kind: number
  kindName: string
  status: number
  statusName: string
  sortOrder: number
  method: string
  url: string
  timeoutMs: number
  parameters: Record<string, unknown>
  updatedAt: string
}

export interface CreateToolInput {
  code: string
  name: string
  description: string
  category: string
  method: string
  url: string
  timeoutMs: number
  status: number
  sortOrder: number
  parameters: Record<string, unknown>
}

export interface UpdateToolInput {
  name: string
  description: string
  category: string
  method: string
  url: string
  timeoutMs: number
  status: number
  sortOrder: number
  parameters: Record<string, unknown>
}

const base = '/api/admin/v1/ai/tools'

export function getTools(): Promise<AiTool[]> {
  return request.get<AiTool[]>(base)
}

export function getTool(code: string): Promise<AiTool> {
  return request.get<AiTool>(`${base}/${encodeURIComponent(code)}`)
}

export function createTool(input: CreateToolInput): Promise<{ id: number }> {
  return request.post<{ id: number }, CreateToolInput>(base, input)
}

export function updateTool(code: string, input: UpdateToolInput): Promise<Record<string, never>> {
  return request.put<Record<string, never>, UpdateToolInput>(
    `${base}/${encodeURIComponent(code)}`,
    input,
  )
}

export function updateToolStatus(code: string, status: number): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { status: number }>(
    `${base}/${encodeURIComponent(code)}/status`,
    { status },
  )
}

export function deleteTool(code: string): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`${base}/${encodeURIComponent(code)}`)
}
