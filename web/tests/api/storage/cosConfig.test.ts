import { describe, expect, it, vi, beforeEach } from 'vitest'
import { request, ProtocolError } from '@/utils/request'
import {
  listCosConfigs,
  parseCosConfigResponse,
  updateCosConfig,
  type CosConfig,
} from '@/api/storage/cosConfig'

vi.mock('@/utils/request', async () => {
  const actual = await vi.importActual<typeof import('@/utils/request')>('@/utils/request')
  return { ...actual, request: vi.fn() }
})
const requestMock = vi.mocked(request)
const config: CosConfig = {
  id: 1,
  name: 'Main',
  appId: 'app',
  bucket: 'assets',
  region: 'ap-guangzhou',
  endpoint: null,
  bucketDomain: null,
  isEnabled: 1,
  hasCredentials: true,
  remark: '',
  createdAt: '2026-08-30T00:00:00Z',
  updatedAt: '2026-08-30T00:00:00Z',
}
describe('COS config API', () => {
  beforeEach(() => vi.clearAllMocks())
  it('parses safe config metadata only', () => {
    expect(parseCosConfigResponse(config)).toEqual(config)
    expect(() => parseCosConfigResponse({ ...config, secretId: 'leak' })).toThrow(ProtocolError)
    for (const field of ['currentVersion', 'generation', 'revision']) {
      expect(() => parseCosConfigResponse({ ...config, [field]: 1 })).toThrow(ProtocolError)
    }
  })
  it('uses the exact list endpoint and query', async () => {
    requestMock.mockResolvedValue({ list: [config], total: 1, page: 1, pageSize: 20 })
    await expect(listCosConfigs({ page: 1, pageSize: 20, keyword: 'main' })).resolves.toEqual({
      list: [config],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'GET',
      url: '/api/admin/v1/storage/cosconfig',
      params: { page: 1, pageSize: 20, keyword: 'main' },
    })
  })

  it('updates only mutable business fields without appId or versions', async () => {
    requestMock.mockResolvedValue({})
    await updateCosConfig(7, {
      name: 'Main v2',
      bucket: 'assets-v2',
      region: 'ap-shanghai',
      endpoint: null,
      bucketDomain: 'https://cdn.example.com',
      remark: '',
    })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'PUT',
      url: '/api/admin/v1/storage/cosconfig/7',
      data: {
        name: 'Main v2',
        bucket: 'assets-v2',
        region: 'ap-shanghai',
        endpoint: null,
        bucketDomain: 'https://cdn.example.com',
        remark: '',
      },
    })
  })
})
