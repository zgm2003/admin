export interface AgentForm {
  name: string
  description: string
  avatar: string
  systemPrompt: string
  scene: number
  type: number
  providerModelId: number | null
  timeoutMs: number
  status: number
}

export function emptyAgentForm(): AgentForm {
  return {
    name: '',
    description: '',
    avatar: '',
    systemPrompt: '',
    scene: 1,
    type: 1,
    providerModelId: null,
    timeoutMs: 60000,
    status: 0,
  }
}
