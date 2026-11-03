// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as chat from '@/api/ai/chat'
import * as run from '@/api/ai/run'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}))
beforeEach(() => vi.resetAllMocks())
it('uses the owned conversation and candidate surfaces with cursor pagination', async () => {
  await chat.listAgentOptions()
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/chat/agent', { params: {} })
  await chat.listConversations({ beforeId: '9', limit: 20 })
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/chat/conversation', {
    params: { beforeId: '9', limit: 20 },
  })
})
it('keeps generation on the owned permission surface', async () => {
  const input = {
    agentId: '3',
    requestKey: 'key',
    operation: 'image.generate',
    input: { text: 'draw' },
  }
  await run.createRun(input)
  expect(request.post).toHaveBeenCalledWith('/api/admin/v1/ai/chat/run', input)
})

it('uses owned event recovery and protected manager detail surfaces', async () => {
  await run.listRunEvents('3', { afterSequence: 19, limit: 20 })
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/chat/run/3/event', {
    params: { afterSequence: 19, limit: 20 },
  })
  await run.getRun('3', true)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/run/3')
  await run.getRunOptions()
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/chat/options')
  await run.previewRunMedia('3', '4', 'attachment', true)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/run/3/media/4/preview', {
    params: { disposition: 'attachment' },
  })
  await chat.renameConversation('8', 'Renamed')
  expect(request.put).toHaveBeenCalledWith('/api/admin/v1/ai/chat/conversation/8', {
    title: 'Renamed',
  })
  await chat.deleteConversation('8')
  expect(request.delete).toHaveBeenCalledWith('/api/admin/v1/ai/chat/conversation/8')
})

it('reads own charges with detail access rather than manager access', async () => {
  await run.listRunRecords('3', 'usage-charge', { limit: 20 }, false)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/chat/run/3/usage-charge', {
    params: { limit: 20 },
  })
})
