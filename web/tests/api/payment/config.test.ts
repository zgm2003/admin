// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/payment/config'
import { request } from '@/utils/request'
vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))
beforeEach(() => vi.resetAllMocks())
it('preserves numeric enums and explicit null credential replacement', async () => {
  const input = {
    privateKey: null,
    appCertPEM: null,
    alipayCertPEM: null,
    rootCertPEM: null,
    notifyUrl: 'https://pay.example.test/api/admin/v1/payment/callback/alipay/{configId}/{version}',
    payMethods: [2, 1],
  }
  const returned = { newField: 99 }
  vi.mocked(request.post).mockResolvedValue(returned)
  expect(await api.publishPaymentConfigVersion(7, input)).toBe(returned)
  expect(request.post).toHaveBeenCalledExactlyOnceWith(
    '/api/admin/v1/payment/config/7/version',
    input,
  )
})
it('metadata editing does not submit immutable identity or credentials', async () => {
  await api.updatePaymentConfig(7, { name: 'Renamed', sortOrder: 0 })
  expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/payment/config/7', {
    name: 'Renamed',
    sortOrder: 0,
  })
})
it('forwards query and failures without swallowing errors', async () => {
  const error = new Error('unavailable')
  vi.mocked(request.get).mockRejectedValue(error)
  await expect(api.getPaymentConfigs({ page: 1, pageSize: 20, isEnabled: 0 })).rejects.toBe(error)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/config', {
    params: { page: 1, pageSize: 20, isEnabled: 0 },
  })
})
