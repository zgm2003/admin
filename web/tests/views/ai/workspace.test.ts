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
    requestStatuses: [{ value: 2, label: '请求完成' }],
    toolCallStatuses: [{ value: 2, label: '工具完成' }],
    requestUsageStatuses: [{ value: 1, label: '请求用量完整' }],
    usageCompleteness: [
      { value: 0, label: '未完整' },
      { value: 1, label: '完整' },
    ],
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
  vi.mocked(run.getRunOptions).mockResolvedValue({
    operations: [],
    executionStatuses: [{ value: 3, label: '完成' }],
    usageStatuses: [{ value: 1, label: '完整' }],
    billingStatuses: [{ value: 1, label: '待结算' }],
    transferStatuses: [{ value: 3, label: '全部就绪' }],
    mediaTransferStatuses: [],
    requestStatuses: [{ value: 2, label: '请求完成' }],
    toolCallStatuses: [{ value: 2, label: '工具完成' }],
    requestUsageStatuses: [{ value: 1, label: '请求用量完整' }],
    usageCompleteness: [
      { value: 0, label: '未完整' },
      { value: 1, label: '完整' },
    ],
    constraints: {
      defaultLimit: 20,
      maxLimit: 100,
      maxHistoryDays: 31,
      maxSummaryHours: 24,
      maxFiles: 4,
    },
  })
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
        estimatedUnits: '20',
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
  expect(w.text()).toContain('完整')
  expect(w.text()).toContain('待结算')
  expect(w.text()).toContain('全部就绪')
  await w.find('[data-testid="ai-run-detail"]').trigger('click')
  await flushPromises()
  expect(run.getRun).toHaveBeenCalledWith('18', true)
  expect(w.text()).toContain('draw')
})

it('keeps separate request, tool, charge, and charge-item detail tabs', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:run:list', 'ai:run:detail']
  const item = {
    id: '19',
    agentId: '1',
    conversationId: null,
    operation: 'text.generate',
    executionStatus: 3,
    usageStatus: 1,
    billingStatus: 1,
    transferStatus: 0,
    estimatedUnits: '1',
    acceptedAt: '2026-10-29',
    finishedAt: '2026-10-29',
    errorCode: null,
    errorMessage: null,
    stateVersion: 1,
    nextEventSequence: 0,
    canCancel: false,
  }
  vi.mocked(run.listRuns).mockResolvedValue({ ...page, list: [item] })
  vi.mocked(run.getRun).mockResolvedValue({ ...item, input: { text: 'hello' }, result: 'done' })
  vi.mocked(run.listRunRecords).mockResolvedValue(page)
  vi.mocked(run.listRunMedia).mockResolvedValue(page)

  const w = await render(Run)
  await w.find('[data-testid="ai-run-detail"]').trigger('click')
  await flushPromises()

  const tabs = w.findAll('.el-radio-button')
  expect(tabs.filter((tab) => tab.text() === appI18n.global.t('aiRun.requests'))).toHaveLength(1)
  expect(tabs.filter((tab) => tab.text() === appI18n.global.t('aiRun.tools'))).toHaveLength(1)
  expect(tabs.filter((tab) => tab.text() === appI18n.global.t('aiRun.charges'))).toHaveLength(1)
  expect(tabs.filter((tab) => tab.text() === appI18n.global.t('aiRun.chargeItems'))).toHaveLength(1)
})

it('renders monitor records as readable tables and reveals request payloads on demand', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:run:list', 'ai:run:detail']
  const item = {
    id: '61',
    agentId: '1',
    conversationId: null,
    operation: 'text.generate',
    executionStatus: 3,
    usageStatus: 1,
    billingStatus: 2,
    transferStatus: 0,
    estimatedUnits: '9007199254740993',
    acceptedAt: '2026-11-03T01:00:00Z',
    finishedAt: '2026-11-03T01:00:01Z',
    errorCode: null,
    errorMessage: null,
    stateVersion: 1,
    nextEventSequence: 0,
    canCancel: false,
  }
  vi.mocked(run.listRuns).mockResolvedValue({ ...page, list: [item] })
  vi.mocked(run.getRun).mockResolvedValue({ ...item, input: { text: 'hello' }, result: 'done' })
  vi.mocked(run.listRunMedia).mockResolvedValue(page)
  vi.mocked(run.listRunRecords).mockResolvedValueOnce({
    ...page,
    list: [
      {
        id: 'request-61',
        runId: '61',
        sequence: 1,
        status: 44,
        responseItems: [{ text: 'Detailed model response' }],
        usage: { inputTokens: 7 },
        usageStatus: 1,
        startedAt: '2026-11-03T01:00:00Z',
        finishedAt: '2026-11-03T01:00:01Z',
      },
    ],
  })
  const w = await render(Run)
  await w.find('[data-testid="ai-run-detail"]').trigger('click')
  await flushPromises()
  let table = w.get('[data-testid="ai-run-records"]')
  expect(table.findAll('th').map((cell) => cell.text())).toContain(
    appI18n.global.t('aiRun.requestId'),
  )
  expect(table.text()).toContain('request-61')
  expect(table.text()).toContain('44')
  expect(table.text()).toContain('请求用量完整')
  expect(table.text()).not.toContain('Detailed model response')
  await table.get('.el-table__expand-icon').trigger('click')
  await flushPromises()
  expect(table.text()).toContain('Detailed model response')

  vi.mocked(run.listRunRecords).mockResolvedValueOnce({
    ...page,
    list: [
      {
        id: 'tool-61',
        runId: '61',
        providerRequestId: 'request-61',
        exposedName: 'currentTime',
        status: 2,
        startedAt: null,
        finishedAt: null,
      },
    ],
  })
  await w
    .findAll('.el-radio-button')
    .find((tab) => tab.text() === appI18n.global.t('aiRun.tools'))!
    .get('input')
    .setValue(true)
  await flushPromises()
  table = w.get('[data-testid="ai-run-records"]')
  expect(table.text()).toContain('currentTime')
  expect(table.text()).toContain('工具完成')
  expect(table.findAll('th').map((cell) => cell.text())).toContain(
    appI18n.global.t('aiRun.toolName'),
  )
  expect(table.find('pre').exists()).toBe(false)

  vi.mocked(run.listRunRecords).mockResolvedValueOnce({
    ...page,
    list: [
      {
        id: 'charge-61',
        runId: '61',
        status: 2,
        currency: 'CNY',
        priceVersionId: 'price-2',
        multiplierPpm: 1500000,
        actualUnits: '9007199254740993',
        usageComplete: 1,
        settledAt: '2026-11-03T01:00:01Z',
        transactionId: 'transaction-3',
      },
    ],
  })
  await w
    .findAll('.el-radio-button')
    .find((tab) => tab.text() === appI18n.global.t('aiRun.charges'))!
    .get('input')
    .setValue(true)
  await flushPromises()
  table = w.get('[data-testid="ai-run-records"]')
  expect(table.text()).toContain('9007199254740993')
  expect(table.text()).toContain('transaction-3')
  expect(table.findAll('th').map((cell) => cell.text())).toContain(
    appI18n.global.t('aiRun.actualUnits'),
  )
  expect(table.find('pre').exists()).toBe(false)
})

it('shows owned charge and item amounts without rounding and keeps unknown amounts distinct', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:chat:list', 'ai:chat:detail']
  const item = {
    id: '62',
    agentId: '1',
    conversationId: null,
    operation: 'text.generate',
    executionStatus: 3,
    usageStatus: 1,
    billingStatus: 2,
    transferStatus: 0,
    estimatedUnits: '9007199254740993',
    acceptedAt: '2026-11-03T01:00:00Z',
    finishedAt: '2026-11-03T01:00:01Z',
    errorCode: null,
    errorMessage: null,
    stateVersion: 1,
    nextEventSequence: 0,
    canCancel: false,
  }
  vi.mocked(run.listRuns).mockResolvedValue({ ...page, list: [item] })
  vi.mocked(run.getRun).mockResolvedValue({ ...item, input: { text: 'hello' }, result: 'done' })
  vi.mocked(run.listRunEvents).mockResolvedValue({ list: [], hasMore: false, nextSequence: 0 })
  vi.mocked(run.listRunMedia).mockResolvedValue(page)
  vi.mocked(run.listRunRecords)
    .mockResolvedValueOnce({
      ...page,
      list: [
        {
          id: 'charge-62',
          runId: '62',
          status: 4,
          currency: 'CNY',
          priceVersionId: 'price-2',
          multiplierPpm: 1000000,
          actualUnits: null,
          usageComplete: 0,
          settledAt: null,
          transactionId: null,
        },
      ],
    })
    .mockResolvedValueOnce({
      ...page,
      list: [
        {
          id: 'item-62',
          chargeId: 'charge-62',
          providerRequestId: 'request-62',
          priceRateId: 'rate-1',
          operation: 'text.generate',
          usageKey: 'inputTokens',
          selector: { quality: 'high' },
          quantity: '10',
          unitQuantity: '1000000',
          unit: 'token',
          billingUnitPriceUnits: '9007199254740993',
          multiplierPpm: 1000000,
          amountUnits: '90071992547409931',
        },
      ],
    })
  const w = await render(Chat)
  await w
    .findAll('button')
    .find((button) => button.text().includes('62 ·'))!
    .trigger('click')
  await flushPromises()
  const tables = w.findAll('[data-testid="ai-run-records"]')
  expect(tables).toHaveLength(2)
  expect(tables[0]!.text()).toContain(appI18n.global.t('aiRun.amountUnknown'))
  expect(tables[0]!.text()).toContain('未完整')
  expect(tables[0]!.find('pre').exists()).toBe(false)
  expect(tables[1]!.text()).toContain('9007199254740993')
  expect(tables[1]!.text()).toContain('90071992547409931')
  expect(tables[1]!.text()).toContain('inputTokens')
  expect(tables[1]!.text()).not.toContain('"quality"')
  await tables[1]!.find('.el-table__expand-icon').trigger('click')
  await flushPromises()
  expect(tables[1]!.text()).toContain('"quality"')
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
    estimatedUnits: '1',
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
    estimatedUnits: '1',
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
    estimatedUnits: '1',
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
    estimatedUnits: '1',
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
    estimatedUnits: '1',
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

it('only shows the video notice when the backend exposes video generation', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:chat:generate']
  vi.mocked(chat.listAgentOptions).mockResolvedValue({
    ...page,
    list: [
      {
        id: '7',
        name: 'Text agent',
        capabilities: ['text'],
        operations: ['text.generate'],
        parameters: {},
        generationParams: {},
        limits: {},
        parameterOptions: [],
      },
    ],
  })
  vi.mocked(run.getRunOptions).mockResolvedValue({
    operations: [{ value: 'text.generate', label: 'Text' }],
    executionStatuses: [],
    usageStatuses: [],
    billingStatuses: [],
    transferStatuses: [],
    mediaTransferStatuses: [],
    requestStatuses: [{ value: 2, label: '请求完成' }],
    toolCallStatuses: [{ value: 2, label: '工具完成' }],
    requestUsageStatuses: [{ value: 1, label: '请求用量完整' }],
    usageCompleteness: [
      { value: 0, label: '未完整' },
      { value: 1, label: '完整' },
    ],
    constraints: {
      defaultLimit: 20,
      maxLimit: 100,
      maxHistoryDays: 31,
      maxSummaryHours: 24,
      maxFiles: 4,
    },
  })
  const w = await render(Chat)
  expect(w.text()).not.toContain(appI18n.global.t('aiChat.videoDisabled'))
})

it('uses backend status labels in workspace run history', async () => {
  usePermissionStore(pinia).permissionCodes = ['ai:chat:list', 'ai:chat:detail']
  vi.mocked(run.getRunOptions).mockResolvedValue({
    operations: [],
    executionStatuses: [{ value: 3, label: '已完成' }],
    usageStatuses: [],
    billingStatuses: [],
    transferStatuses: [],
    mediaTransferStatuses: [],
    requestStatuses: [{ value: 2, label: '请求完成' }],
    toolCallStatuses: [{ value: 2, label: '工具完成' }],
    requestUsageStatuses: [{ value: 1, label: '请求用量完整' }],
    usageCompleteness: [
      { value: 0, label: '未完整' },
      { value: 1, label: '完整' },
    ],
    constraints: {
      defaultLimit: 20,
      maxLimit: 100,
      maxHistoryDays: 31,
      maxSummaryHours: 24,
      maxFiles: 4,
    },
  })
  vi.mocked(run.listRuns).mockResolvedValue({
    ...page,
    list: [
      {
        id: '51',
        agentId: '1',
        conversationId: null,
        operation: 'text.generate',
        executionStatus: 3,
        usageStatus: 1,
        billingStatus: 2,
        transferStatus: 0,
        estimatedUnits: '1',
        acceptedAt: 'now',
        finishedAt: 'now',
        errorCode: null,
        errorMessage: null,
        stateVersion: 1,
        nextEventSequence: 0,
        canCancel: false,
      },
    ],
  })
  const w = await render(Chat)
  const runButton = w.findAll('button').find((button) => button.text().includes('51 ·'))!
  expect(runButton.text()).toContain('已完成')
  expect(runButton.text()).not.toContain(' · 3')
})
