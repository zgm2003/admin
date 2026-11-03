// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as agent from '@/api/ai/agent'
import * as tool from '@/api/ai/tool'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))
beforeEach(() => vi.resetAllMocks())
it('uses independent agent current/status actions and strips action fields from update', async () => {
  const input = {
    name: 'A',
    description: '',
    avatarFileId: null,
    capabilities: ['text'],
    systemPrompt: 'p',
    multiplierPpm: 1000000,
    generationParams: {},
    bindings: [],
    tools: [],
    status: 1,
    activeBindingId: '8',
  }
  await agent.updateAgent('3', input)
  expect(request.put).toHaveBeenCalledWith('/api/admin/v1/ai/agent/3', {
    name: 'A',
    description: '',
    avatarFileId: null,
    capabilities: ['text'],
    systemPrompt: 'p',
    multiplierPpm: 1000000,
    generationParams: {},
    bindings: [],
    tools: [],
  })
  await agent.switchAgentBinding('3', '8')
  expect(request.patch).toHaveBeenCalledWith('/api/admin/v1/ai/agent/3/binding/current', {
    bindingId: '8',
  })
  await agent.updateAgentStatus('3', 0)
  expect(request.patch).toHaveBeenCalledWith('/api/admin/v1/ai/agent/3/status', { status: 0 })
})
it('loads candidates through the matching create/update permission surface', async () => {
  await agent.getAgentCandidates(null, { kind: 'tool', page: 2, pageSize: 20, keyword: 'time' })
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/agent/candidate/create', {
    params: { kind: 'tool', page: 2, pageSize: 20, keyword: 'time' },
  })
  await agent.getAgentCandidates('3', { kind: 'providerModel', page: 1, pageSize: 20 })
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/agent/3/candidate/update', {
    params: { kind: 'providerModel', page: 1, pageSize: 20 },
  })
})
it('tool updates carry version configuration but no status or secrets from previous details', async () => {
  const version = {
    type: 2,
    description: 'calculate',
    inputSchema: {},
    outputSchema: {},
    builtinKey: 'calculator',
    httpDefinition: null,
    timeoutMs: 1000,
    outputLimitBytes: 65536,
    hasSideEffect: 0,
    idempotencyMode: 0,
  }
  const input = { name: 'Calculator', version, status: 0 }
  await tool.updateTool('2', input)
  expect(request.put).toHaveBeenCalledWith('/api/admin/v1/ai/tool/2', {
    name: 'Calculator',
    version,
  })
  await tool.updateToolStatus('2', 1)
  expect(request.patch).toHaveBeenCalledWith('/api/admin/v1/ai/tool/2/status', { status: 1 })
})
