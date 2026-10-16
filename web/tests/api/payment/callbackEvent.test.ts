// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/payment/callbackEvent'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))

beforeEach(() => vi.resetAllMocks())

it('keeps callback audit on its independent permission surface', async () => {
  await api.getOrderCallbackEvents(9, { page: 1, pageSize: 20 })
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/order/9/callback', {
    params: { page: 1, pageSize: 20 },
  })
  await api.getOrderCallbackEvent(9, 12)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/order/9/callback/12')
})
