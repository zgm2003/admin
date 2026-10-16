// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/payment/transaction'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))

beforeEach(() => vi.resetAllMocks())

it('keeps the personal transaction projection free of admin fields', async () => {
  await api.getMyTransactions({ page: 1, pageSize: 20 })
  expect(request.get).toHaveBeenCalledWith('/api/v1/payment/wallet/transaction', {
    params: { page: 1, pageSize: 20 },
  })
  await api.getTransactions({ page: 1, pageSize: 20, userId: 8 })
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/transaction', {
    params: { page: 1, pageSize: 20, userId: 8 },
  })
})
