export interface AgentForm {
  name: string
  description: string
  systemPrompt: string
  providerModelId: number | null
  status: number
}

export function emptyAgentForm(): AgentForm {
  return {
    name: '',
    description: '',
    systemPrompt: '',
    providerModelId: null,
    status: 0,
  }
}
