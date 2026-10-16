// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/payment/wallet'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))

beforeEach(() => vi.resetAllMocks())

it('keeps personal and admin wallet projections on separate endpoints', async () => {
  await api.getMyWallet()
  expect(request.get).toHaveBeenCalledWith('/api/v1/payment/wallet')
  await api.getWallets({ page: 1, pageSize: 20 })
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/wallet', {
    params: { page: 1, pageSize: 20 },
  })
  await api.getWallet(8)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/wallet/8')
})
