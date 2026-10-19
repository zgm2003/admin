// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'

import * as api from '@/api/payment/redeemCode'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))

beforeEach(() => vi.resetAllMocks())

it('keeps admin redeem-code endpoints and numeric filters', async () => {
  await api.getRedeemCodeOptions()
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/redeemcode/options')
  await api.getRedeemCodes({ page: 1, pageSize: 20, batchId: 7, status: 1 })
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/redeemcode', {
    params: { page: 1, pageSize: 20, batchId: 7, status: 1 },
  })
  await api.getRedeemCode(9)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/redeemcode/9')
})

it('does not reshape secret-code requests or responses', async () => {
  await api.lookupRedeemCode('ABCD2345EFGHJKLMNPQR')
  expect(request.post).toHaveBeenCalledWith('/api/admin/v1/payment/redeemcode/lookup', {
    code: 'ABCD2345EFGHJKLMNPQR',
  })
  const input = {
    requestId: '00000000-0000-4000-8000-000000000001',
    amount: '10.00',
    quantity: 2,
    expiresAt: null,
    remark: '',
  }
  await api.generateRedeemCodeBatch(input)
  expect(request.post).toHaveBeenCalledWith('/api/admin/v1/payment/redeemcode/batch', input)
  await api.exportRedeemCodes([1, 2])
  expect(request.post).toHaveBeenCalledWith('/api/admin/v1/payment/redeemcode/export', {
    ids: [1, 2],
  })
  await api.voidRedeemCodes([1])
  expect(request.patch).toHaveBeenCalledWith('/api/admin/v1/payment/redeemcode/void', { ids: [1] })
})

it('uses the personal wallet redemption endpoint', async () => {
  await api.redeemWalletCode('ABCD2345EFGHJKLMNPQR')
  expect(request.post).toHaveBeenCalledWith('/api/admin/v1/payment/wallet/redemption', {
    code: 'ABCD2345EFGHJKLMNPQR',
  })
})
