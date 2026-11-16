import { request } from '@/utils/request'

export interface AiTool {
  id: number
  code: string
  name: string
  description: string
  category: string
  status: number
  statusName: string
  sortOrder: number
  parameters: Record<string, unknown>
  updatedAt: string
}

const base = '/api/admin/v1/ai/tools'

export function getTools(): Promise<AiTool[]> {
  return request.get<AiTool[]>(base)
}

export function updateToolStatus(code: string, status: number): Promise<Record<string, never>> {
  return request.patch<Record<string, never>, { status: number }>(
    `${base}/${encodeURIComponent(code)}/status`,
    { status },
  )
}
