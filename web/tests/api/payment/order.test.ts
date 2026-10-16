// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/payment/order'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))

beforeEach(() => vi.resetAllMocks())

it('sends only the fixed recharge create contract', async () => {
  const input = {
    requestId: '00000000-0000-4000-8000-000000000001',
    packageId: 8,
    payMethod: 1,
    returnUrl: 'https://app.example.test/paid',
  }
  await api.createRecharge(input)
  expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/v1/payment/recharge', input)
  await api.payRecharge(9)
  expect(request.post).toHaveBeenCalledWith('/api/v1/payment/recharge/9/pay')
})
