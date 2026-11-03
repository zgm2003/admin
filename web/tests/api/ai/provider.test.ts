// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/ai/provider'
import * as official from '@/api/ai/officialModel'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))
beforeEach(() => vi.resetAllMocks())

it('projects ordinary provider updates without status, identity or pricing fields', async () => {
  const input = {
    code: 'protected',
    status: 1,
    name: 'OpenAI',
    description: '',
    baseUrl: 'https://api.openai.com/v1',
    apiKey: '',
    headers: null,
    mediaHostAllowlist: [],
    requestTimeoutMs: 30000,
  }
  await api.updateProvider('1', input)
  expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/ai/provider/1', {
    name: 'OpenAI',
    description: '',
    baseUrl: 'https://api.openai.com/v1',
    apiKey: '',
    mediaHostAllowlist: [],
    requestTimeoutMs: 30000,
  })
})

it('preserves decimal prices and publishes the exact preview digest', async () => {
  await api.previewProviderPrice('1', '2', {
    versionId: '3',
    effectiveAt: '2026-10-28T00:00:00Z',
    rates: [{ rateId: '4', billingUnitPrice: '0.00000001' }],
  })
  expect(request.put).toHaveBeenCalledWith('/api/admin/v1/ai/provider/1/model/2/price', {
    versionId: '3',
    effectiveAt: '2026-10-28T00:00:00Z',
    rates: [{ rateId: '4', billingUnitPrice: '0.00000001' }],
  })
  await api.publishProviderPrice('1', '2', '5', 'exact-digest')
  expect(request.post).toHaveBeenCalledWith('/api/admin/v1/ai/provider/1/model/2/price/5/publish', {
    snapshotDigest: 'exact-digest',
  })
})

it('confirms only selected models and never sends inferred prices or status', async () => {
  const model = {
    upstreamModelId: 'gpt-5',
    name: 'GPT-5',
    officialModelId: '8',
    operations: ['text.generate'],
    limits: {},
    status: 1,
  }
  await api.confirmProviderCatalog('1', 'digest', [model])
  expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/ai/provider/1/model/sync', {
    digest: 'digest',
    models: [
      {
        upstreamModelId: 'gpt-5',
        name: 'GPT-5',
        officialModelId: '8',
        operations: ['text.generate'],
        limits: {},
      },
    ],
  })
})

it('keeps official model identity and status out of metadata updates', async () => {
  const input = {
    vendorCode: 'openai',
    modelKey: 'gpt-5',
    status: 1,
    name: 'GPT-5',
    operations: ['text.generate'],
    sourceUrls: ['https://developers.openai.com/api/docs/pricing'],
  }
  await official.updateOfficialModel('8', input)
  expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/ai/officialmodel/8', {
    name: 'GPT-5',
    operations: ['text.generate'],
    sourceUrls: ['https://developers.openai.com/api/docs/pricing'],
  })
})

it('forwards real pagination and errors unchanged', async () => {
  const error = new Error('unavailable')
  vi.mocked(request.get).mockRejectedValue(error)
  await expect(api.getProviders({ page: 2, pageSize: 20 })).rejects.toBe(error)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/ai/provider', {
    params: { page: 2, pageSize: 20 },
  })
})
