import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import Chat from '@/views/ai/chat/index.vue'
import Run from '@/views/ai/run/index.vue'
import FileSelectDialog from '@/components/FileSelectDialog/index.vue'
import * as chat from '@/api/ai/chat'
import * as run from '@/api/ai/run'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import { appI18n } from '@/i18n'
vi.mock('@/api/ai/chat', () => ({
  listAgentOptions: vi.fn(),
  listConversations: vi.fn(),
  listMessages: vi.fn(),
  getConversation: vi.fn(),
  createConversation: vi.fn(),
  renameConversation: vi.fn(),
  deleteConversation: vi.fn(),
}))
vi.mock('@/api/ai/run', () => ({
  getRunOptions: vi.fn(),
  listRuns: vi.fn(),
  getRunSummary: vi.fn(),
  getRun: vi.fn(),
  listRunEvents: vi.fn(),
  listRunMedia: vi.fn(),
  listRunRecords: vi.fn(),
  createRun: vi.fn(),
  cancelRun: vi.fn(),
  retryRunTransfer: vi.fn(),
  previewRunMedia: vi.fn(),
}))
const wrappers: VueWrapper[] = []
const page = { list: [], hasMore: false, nextBeforeId: '' }
async function render(component: typeof Chat | typeof Run) {
  const w = mount(component, {
    global: {
      plugins: [pinia, appI18n, ElementPlus],
      stubs: {
        AppDialog: {
          props: ['modelValue'],
          template: '<section v-if="modelValue"><slot /></section>',
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
  usePermissionStore(pinia).reset()
  vi.mocked(chat.listConversations).mockResolvedValue(page)
  vi.mocked(chat.listAgentOptions).mockResolvedValue(page)
  vi.mocked(run.listRuns).mockResolvedValue(page)
  vi.mocked(run.getRunOptions).mockResolvedValue({
    operations: [{ value: 'image.generate', label: 'Image' }],
    executionStatuses: [],
    usageStatuses: [],
    billingStatuses: [],
    transferStatuses: [],
    mediaTransferStatuses: [],
    constraints: {
      defaultLimit: 20,
      maxLimit: 100,
      maxHistoryDays: 31,
      maxSummaryHours: 24,
      maxFiles: 4,
    },
  })
  vi.mocked(run.getRunSummary).mockResolvedValue({
    runCount: 0,
    estimatedUnits: '0',
    settledUnits: '0',
  })
})
afterEach(() => {
  wrappers.forEach((w) => w.unmount())
  wrappers.length = 0
  vi.useRealTimers()
  window.history.replaceState(null, '', '/')
})
it('does not fetch protected records or show generation for page-only access', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:chat:view', 'ai:run:view']
  const c = await render(Chat)
  await render(Run)
  expect(chat.listConversations).not.toHaveBeenCalled()
  expect(chat.listAgentOptions).not.toHaveBeenCalled()
  expect(run.listRuns).not.toHaveBeenCalled()
  expect(c.find('[data-testid="ai-chat-send"]').exists()).toBe(false)
})
it('shows real monitor records and opens authorized details', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:run:list', 'ai:run:detail']
  vi.mocked(run.listRuns).mockResolvedValue({
    ...page,
    list: [
      {
        id: '18',
        agentId: '1',
        conversationId: null,
        operation: 'image.generate',
        executionStatus: 3,
        usageStatus: 1,
        billingStatus: 1,
        transferStatus: 3,
        estimatedUnits: 20,
        acceptedAt: '2026-10-29',
        finishedAt: null,
        errorCode: null,
        errorMessage: null,
        stateVersion: 1,
        nextEventSequence: 0,
        canCancel: false,
      },
    ],
  })
  vi.mocked(run.getRun).mockResolvedValue({
    ...(await run.listRuns()).list[0]!,
    input: { text: 'draw' },
    result: 'done',
  })
  vi.mocked(run.listRunRecords).mockResolvedValue(page)
  vi.mocked(run.listRunMedia).mockResolvedValue(page)
  const w = await render(Run)
  expect(w.text()).toContain('18')
  await w.find('[data-testid="ai-run-detail"]').trigger('click')
  await flushPromises()
  expect(run.getRun).toHaveBeenCalledWith('18', true)
  expect(w.text()).toContain('draw')
})

it('generates an image with the selected agent and preserves request identity after a failed submit', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:chat:generate']
  vi.mocked(chat.listAgentOptions).mockResolvedValue({
    ...page,
    list: [
      {
        id: '7',
        name: 'Image agent',
        capabilities: ['image'],
        operations: ['image.generate'],
        parameters: {},
        generationParams: {},
        limits: {},
        parameterOptions: [],
      },
    ],
  })
  vi.mocked(run.createRun).mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({
    id: '22',
    agentId: '7',
    conversationId: null,
    operation: 'image.generate',
    executionStatus: 0,
    usageStatus: 0,
    billingStatus: 0,
    transferStatus: 0,
    estimatedUnits: 1,
    acceptedAt: 'now',
    finishedAt: null,
    errorCode: null,
    errorMessage: null,
    stateVersion: 1,
    nextEventSequence: 0,
    canCancel: true,
  })
  const w = await render(Chat)
  w.findAllComponents({ name: 'ElSelectV2' })[0]!.vm.$emit('update:modelValue', '7')
  await flushPromises()
  w.findAllComponents({ name: 'ElSelectV2' })[1]!.vm.$emit('update:modelValue', 'image.generate')
  await w.find('textarea').setValue('A mountain')
  await flushPromises()
  await w.find('[data-testid="ai-chat-send"]').trigger('click')
  await flushPromises()
  await w.find('[data-testid="ai-chat-send"]').trigger('click')
  await flushPromises()
  expect(run.createRun).toHaveBeenCalledTimes(2)
  const first = vi.mocked(run.createRun).mock.calls[0]![0]
  expect(first).toMatchObject({
    agentId: '7',
    operation: 'image.generate',
    input: { text: 'A mountain' },
  })
  expect(first.requestKey).toBeTruthy()
  expect(vi.mocked(run.createRun).mock.calls[1]![0].requestKey).toBe(first.requestKey)
  expect(run.getRun).not.toHaveBeenCalled()
})
it('uses the next cursor for monitor pagination without restarting generation', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:run:list']
  vi.mocked(run.listRuns)
    .mockResolvedValueOnce({ ...page, hasMore: true, nextBeforeId: '99' })
    .mockResolvedValueOnce(page)
  const w = await render(Run)
  const more = w.findAll('button').find((button) => button.text().includes('加载更多'))
  expect(more).toBeDefined()
  await more!.trigger('click')
  await flushPromises()
  expect(run.listRuns).toHaveBeenLastCalledWith(
    expect.objectContaining({ beforeId: '99', limit: 20 }),
    true,
  )
  expect(run.createRun).not.toHaveBeenCalled()
})

it('waits for recovery requests, keeps polling cancellation in progress, and stops on unmount', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:chat:list', 'ai:chat:detail']
  const item = {
    id: '31',
    agentId: '1',
    conversationId: null,
    operation: 'image.generate',
    executionStatus: 1,
    usageStatus: 0,
    billingStatus: 0,
    transferStatus: 0,
    estimatedUnits: 1,
    acceptedAt: 'now',
    finishedAt: null,
    errorCode: null,
    errorMessage: null,
    stateVersion: 1,
    nextEventSequence: 0,
    canCancel: true,
  }
  vi.mocked(run.listRuns).mockResolvedValue({ ...page, list: [item] })
  let resolveRequest: ((value: run.RunDetail) => void) | undefined
  vi.mocked(run.getRun)
    .mockResolvedValueOnce({ ...item, input: { text: 'x' }, result: null })
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveRequest = resolve
        }),
    )
  vi.mocked(run.getRun).mockResolvedValue({
    ...item,
    canCancel: false,
    input: { text: 'x' },
    result: null,
  })
  vi.mocked(run.listRunEvents).mockResolvedValue({ list: [], hasMore: false, nextSequence: 0 })
  vi.mocked(run.listRunRecords).mockResolvedValue(page)
  vi.mocked(run.listRunMedia).mockResolvedValue(page)
  const w = await render(Chat)
  vi.useFakeTimers()
  const historyButton = w.findAll('button').find((button) => button.text().includes('31 ·'))!
  await historyButton.trigger('click')
  await flushPromises()
  await vi.advanceTimersByTimeAsync(10000)
  expect(run.getRun).toHaveBeenCalledTimes(2)
  resolveRequest!({ ...item, canCancel: false, input: { text: 'x' }, result: null })
  await flushPromises()
  await vi.advanceTimersByTimeAsync(1500)
  expect(run.getRun).toHaveBeenCalledTimes(3)
  w.unmount()
  await vi.advanceTimersByTimeAsync(10000)
  expect(run.getRun).toHaveBeenCalledTimes(3)
})

it('requires an owned attachment for transcription and sends file IDs through the AI input purpose', async () => {
  usePermissionStore(pinia).permissionCodes = [
    'ai:chat:generate',
    'storage:file:list',
    'storage:file:detail',
  ]
  vi.mocked(chat.listAgentOptions).mockResolvedValue({
    ...page,
    list: [
      {
        id: '9',
        name: 'Transcribe',
        capabilities: ['audio'],
        operations: ['audio.transcribe'],
        parameters: {},
        generationParams: {},
        limits: {},
        parameterOptions: [],
      },
    ],
  })
  const opts = await run.getRunOptions()
  vi.mocked(run.getRunOptions).mockResolvedValue({
    ...opts,
    operations: [{ value: 'audio.transcribe', label: 'Transcribe' }],
  })
  vi.mocked(run.createRun).mockResolvedValue({
    id: '23',
    agentId: '9',
    conversationId: null,
    operation: 'audio.transcribe',
    executionStatus: 0,
    usageStatus: 0,
    billingStatus: 0,
    transferStatus: 0,
    estimatedUnits: 1,
    acceptedAt: 'now',
    finishedAt: null,
    errorCode: null,
    errorMessage: null,
    stateVersion: 1,
    nextEventSequence: 0,
    canCancel: true,
  })
  const w = await render(Chat)
  w.findAllComponents({ name: 'ElSelectV2' })[0]!.vm.$emit('update:modelValue', '9')
  await flushPromises()
  w.findAllComponents({ name: 'ElSelectV2' })[1]!.vm.$emit('update:modelValue', 'audio.transcribe')
  await flushPromises()
  expect(w.find('[data-testid="ai-chat-send"]').attributes('disabled')).toBeDefined()
  const selector = w.findComponent(FileSelectDialog)
  expect(selector.props('purpose')).toBe('ai/input')
  selector.vm.$emit('select', {
    id: 42,
    fileName: 'speech.wav',
    mimeType: 'audio/wav',
    sizeBytes: 100,
    category: 'audio',
    createdAt: 'now',
    objectKey: 'audio/key',
  })
  await flushPromises()
  await w.find('[data-testid="ai-chat-send"]').trigger('click')
  await flushPromises()
  expect(run.createRun).toHaveBeenCalledWith(
    expect.objectContaining({
      agentId: '9',
      operation: 'audio.transcribe',
      input: { text: '', fileIds: ['42'] },
    }),
  )
})

it('rebuilds streamed text by event sequence and replaces it with the terminal result', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:chat:list', 'ai:chat:detail']
  const item = {
    id: '51',
    agentId: '1',
    conversationId: null,
    operation: 'text.generate',
    executionStatus: 1,
    usageStatus: 0,
    billingStatus: 0,
    transferStatus: 0,
    estimatedUnits: 1,
    acceptedAt: 'now',
    finishedAt: null,
    errorCode: null,
    errorMessage: null,
    stateVersion: 1,
    nextEventSequence: 2,
    canCancel: true,
  }
  vi.mocked(run.listRuns).mockResolvedValue({ ...page, list: [item] })
  vi.mocked(run.getRun).mockResolvedValue({ ...item, input: { text: 'x' }, result: null })
  const event = {
    id: '1',
    runId: '51',
    sequence: 1,
    eventType: 'textDelta',
    payload: { text: 'Hello' },
    createdAt: 'now',
  }
  vi.mocked(run.listRunEvents)
    .mockResolvedValueOnce({ list: [event], hasMore: false, nextSequence: 1 })
    .mockResolvedValueOnce({
      list: [event, { ...event, id: '2', sequence: 2, payload: { text: ' world' } }],
      hasMore: false,
      nextSequence: 2,
    })
    .mockResolvedValue({ list: [], hasMore: false, nextSequence: 2 })
  vi.mocked(run.listRunMedia).mockResolvedValue(page)
  vi.mocked(run.listRunRecords).mockResolvedValue(page)
  const w = await render(Chat)
  vi.useFakeTimers()
  await w
    .findAll('button')
    .find((button) => button.text().includes('51 ·'))!
    .trigger('click')
  await flushPromises()
  expect(w.find('[data-testid="ai-chat-result"]').exists()).toBe(true)
  expect(w.find('[data-testid="ai-chat-result"]').text()).toBe('Hello')
  await vi.advanceTimersByTimeAsync(1500)
  await flushPromises()
  expect(w.find('[data-testid="ai-chat-result"]').text()).toBe('Hello world')
  vi.mocked(run.getRun).mockResolvedValue({
    ...item,
    executionStatus: 3,
    finishedAt: 'now',
    canCancel: false,
    input: { text: 'x' },
    result: [{ text: 'Final answer' }],
  })
  await vi.advanceTimersByTimeAsync(1500)
  await flushPromises()
  expect(w.find('[data-testid="ai-chat-result"]').text()).toBe('Final answer')
})

it('clears expired event state and recovers the durable run result', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:chat:list', 'ai:chat:detail']
  const item = {
    id: '51',
    agentId: '1',
    conversationId: null,
    operation: 'text.generate',
    executionStatus: 1,
    usageStatus: 0,
    billingStatus: 0,
    transferStatus: 0,
    estimatedUnits: 1,
    acceptedAt: 'now',
    finishedAt: null,
    errorCode: null,
    errorMessage: null,
    stateVersion: 1,
    nextEventSequence: 2,
    canCancel: true,
  }
  vi.mocked(run.listRuns).mockResolvedValue({ ...page, list: [item] })
  vi.mocked(run.getRun).mockResolvedValue({ ...item, input: { text: 'x' }, result: null })
  const event = {
    id: '1',
    runId: '51',
    sequence: 1,
    eventType: 'textDelta',
    payload: { text: 'Hello' },
    createdAt: 'now',
  }
  vi.mocked(run.listRunEvents)
    .mockResolvedValueOnce({ list: [event], hasMore: false, nextSequence: 1 })
    .mockResolvedValueOnce({
      list: [event, { ...event, id: '2', sequence: 2, payload: { text: ' world' } }],
      hasMore: false,
      nextSequence: 2,
    })
    .mockResolvedValue({ list: [], hasMore: false, nextSequence: 2 })
  vi.mocked(run.listRunMedia).mockResolvedValue(page)
  vi.mocked(run.listRunRecords).mockResolvedValue(page)
  const w = await render(Chat)
  vi.useFakeTimers()
  await w
    .findAll('button')
    .find((button) => button.text().includes('51 ·'))!
    .trigger('click')
  await flushPromises()
  expect(w.find('[data-testid="ai-chat-result"]').exists()).toBe(true)
  expect(w.find('[data-testid="ai-chat-result"]').text()).toBe('Hello')
  vi.mocked(run.getRun).mockResolvedValue({
    ...item,
    executionStatus: 3,
    finishedAt: 'now',
    canCancel: false,
    input: { text: 'x' },
    result: [{ text: 'Final answer' }],
  })
  vi.mocked(run.listRunEvents)
    .mockReset()
    .mockResolvedValue({
      list: [],
      hasMore: false,
      nextSequence: 9,
      resetReason: 'EVENTS_EXPIRED',
    } as Awaited<ReturnType<typeof run.listRunEvents>>)
  await vi.advanceTimersByTimeAsync(1500)
  await flushPromises()
  expect(w.find('[data-testid="ai-chat-result"]').text()).toBe('Final answer')
  expect(w.text()).not.toContain('Hello')
})
