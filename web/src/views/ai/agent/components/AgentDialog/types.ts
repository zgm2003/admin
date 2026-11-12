export interface AgentForm {
  name: string
  description: string
  systemPrompt: string
  scene: number
  type: number
  providerModelId: number | null
  status: number
}

export function emptyAgentForm(): AgentForm {
  return {
    name: '',
    description: '',
    systemPrompt: '',
    scene: 1,
    type: 1,
    providerModelId: null,
    status: 0,
  }
}
