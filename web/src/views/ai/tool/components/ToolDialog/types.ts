export interface ToolForm {
  code: string
  name: string
  description: string
  category: string
  method: string
  url: string
  timeoutMs: number
  status: number
  sortOrder: number
  parameters: string
}

export type ToolFormPayload = Omit<ToolForm, 'parameters'> & {
  parameters: Record<string, unknown>
}
