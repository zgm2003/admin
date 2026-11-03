import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import ElementPlus from 'element-plus'

import * as agentApi from '@/api/ai/agent'

import * as toolApi from '@/api/ai/tool'

import AgentPage from '@/views/ai/agent/index.vue'

import ToolPage from '@/views/ai/tool/index.vue'

import { appI18n, setLocale } from '@/i18n'

import { pinia } from '@/store'

import { usePermissionStore } from '@/store/permission'

import { zhCN as agentZh, enUS as agentEn } from '@/i18n/messages/aiAgent'

import { zhCN as toolZh, enUS as toolEn } from '@/i18n/messages/aiTool'

vi.mock('@/api/ai/agent', () => ({
  getAgents: vi.fn(),
  getAgent: vi.fn(),
  getAgentOptions: vi.fn(),
  getAgentCandidates: vi.fn(),
  createAgent: vi.fn(),
  updateAgent: vi.fn(),
  updateAgentStatus: vi.fn(),
  switchAgentBinding: vi.fn(),
  deleteAgent: vi.fn(),
}))

vi.mock('@/api/ai/tool', () => ({
  getTools: vi.fn(),
  getTool: vi.fn(),
  getToolOptions: vi.fn(),
  createTool: vi.fn(),
  updateTool: vi.fn(),
  updateToolStatus: vi.fn(),
  deleteTool: vi.fn(),
}))

const agent = {
  id: '1',
  code: 'agent',
  name: 'Assistant',
  description: 'desc',
  avatarFileId: null,
  capabilities: ['text'],
  systemPrompt: 'Original prompt',
  multiplierPpm: 1000000,
  activeBindingId: '11',
  generationParams: {},
  status: 1,
  updatedAt: '2026-10-28T00:00:00Z',
}

const bindings = [
  { id: '11', agentId: '1', providerModelId: '21', textProtocol: 1, parameters: {}, status: 1 },
  { id: '12', agentId: '1', providerModelId: '22', textProtocol: 1, parameters: {}, status: 1 },
]

const tool = {
  id: '2',
  code: 'calculator',
  name: 'Calculator',
  type: 2,
  currentVersion: 1,
  status: 1,
  createdAt: '2026-10-28T00:00:00Z',
  updatedAt: '2026-10-28T00:00:00Z',
}

const version = {
  id: '31',
  toolId: '2',
  version: 1,
  type: 2,
  description: 'Calculate',
  inputSchema: { type: 'object' },
  outputSchema: { type: 'object' },
  builtinKey: 'calculator',
  httpDefinition: null,
  timeoutMs: 1000,
  outputLimitBytes: 65536,
  hasSideEffect: 0,
  idempotencyMode: 0,
}

const wrappers: VueWrapper[] = []

function permissions(codes: string[]): void {
  usePermissionStore(pinia).permissionCodes = codes
}

async function agentPage(): Promise<VueWrapper> {
  const w = mount(AgentPage, {
    global: {
      plugins: [pinia, appI18n, ElementPlus],
      stubs: {
        AppDialog: {
          props: ['modelValue'],
          template: '<section v-if="modelValue"><slot /><slot name="footer" /></section>',
        },
      },
    },
  })
  wrappers.push(w)
  await flushPromises()
  return w
}

async function toolPage(): Promise<VueWrapper> {
  const w = mount(ToolPage, {
    global: {
      plugins: [pinia, appI18n, ElementPlus],
      stubs: {
        AppDialog: {
          props: ['modelValue'],
          template: '<section v-if="modelValue"><slot /><slot name="footer" /></section>',
        },
      },
    },
  })
  wrappers.push(w)
  await flushPromises()
  return w
}

beforeEach(() => {
  vi.resetAllMocks()
  setLocale('zh-CN')
  appI18n.global.mergeLocaleMessage('zh-CN', { ...agentZh, ...toolZh })
  appI18n.global.mergeLocaleMessage('en-US', { ...agentEn, ...toolEn })
  usePermissionStore(pinia).reset()
  vi.mocked(agentApi.getAgents).mockResolvedValue({
    list: [agent],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(agentApi.getAgent).mockResolvedValue({ agent, bindings, tools: [] })
  vi.mocked(agentApi.getAgentCandidates).mockResolvedValue({
    list: [],
    total: 0,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(agentApi.getAgentOptions).mockResolvedValue({
    statuses: [
      { value: 0, label: 'ͣ��' },
      { value: 1, label: '����' },
    ],
    capabilities: [{ value: 'text', label: '����' }],
    textProtocols: [
      { value: 1, label: 'Responses' },
      { value: 2, label: 'Chat Completions' },
    ],
    defaults: { status: 0, multiplierPpm: 1000000, textProtocol: 1 },
    constraints: {
      maxPageSize: 100,
      maxPromptBytes: 65536,
      maxBoundTools: 16,
      maxToolCalls: 16,
      minMultiplierPpm: 1,
      maxMultiplierPpm: 100000000,
    },
  })
  vi.mocked(toolApi.getTools).mockResolvedValue({ list: [tool], total: 1, page: 1, pageSize: 20 })
  vi.mocked(toolApi.getTool).mockResolvedValue({ tool, version })
  vi.mocked(toolApi.getToolOptions).mockResolvedValue({
    statuses: [
      { value: 0, label: 'ͣ��' },
      { value: 1, label: '����' },
    ],
    types: [
      { value: 1, label: 'HTTP' },
      { value: 2, label: '����' },
    ],
    builtins: [
      { value: 'calculator', label: '������' },
      { value: 'currentTime', label: '��ǰʱ��' },
    ],
    idempotencyModes: [
      { value: 0, label: '�޸�����' },
      { value: 1, label: '�ݵȼ�' },
    ],
    defaults: { status: 0, timeoutMs: 10000, outputLimitBytes: 65536 },
    constraints: {
      maxPageSize: 100,
      minTimeoutMs: 100,
      maxTimeoutMs: 30000,
      maxOutputLimitBytes: 65536,
      maxSchemaBytes: 65536,
      maxSchemaDepth: 16,
    },
  })
})

afterEach(() => {
  for (const w of wrappers) w.unmount()
  wrappers.length = 0
  document.body.innerHTML = ''
})

it('keeps page access independent from list and write actions', async () => {
  permissions(['ai:agent:view', 'ai:tool:view'])
  const a = await agentPage()
  const b = await toolPage()
  expect(agentApi.getAgents).not.toHaveBeenCalled()
  expect(toolApi.getTools).not.toHaveBeenCalled()
  expect(a.find('[data-testid="ai-agent-create"]').exists()).toBe(false)
  expect(b.find('[data-testid="ai-tool-create"]').exists()).toBe(false)
})

it('preserves unknown capabilities and status rather than relabeling them', async () => {
  permissions(['ai:agent:list'])
  vi.mocked(agentApi.getAgents).mockResolvedValue({
    list: [{ ...agent, status: 72, capabilities: ['future'] }],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  const w = await agentPage()
  expect(w.text()).toContain('72')
  expect(w.text()).toContain('future')
  expect(w.find('[data-testid="ai-agent-edit"]').exists()).toBe(false)
})
