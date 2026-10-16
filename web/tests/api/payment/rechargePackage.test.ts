// @vitest-environment node
import { beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/payment/rechargePackage'
import { request } from '@/utils/request'

vi.mock('@/utils/request', () => ({
  request: { get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), delete: vi.fn() },
}))

beforeEach(() => vi.resetAllMocks())

it('uses the recharge package CRUD path and preserves decimal strings', async () => {
  const input = { code: 'starter', name: 'Starter', amount: '10.00', badge: null, sortOrder: 0, isEnabled: 1 as const }
  await api.createRechargePackage(input)
  expect(request.post).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/payment/rechargepackage', input)
  await api.updateRechargePackage(7, input)
  expect(request.put).toHaveBeenCalledExactlyOnceWith('/api/admin/v1/payment/rechargepackage/7', input)
})

it('forwards list failures without reshaping the response', async () => {
  const error = new Error('unavailable')
  vi.mocked(request.get).mockRejectedValue(error)
  await expect(api.getRechargePackages({ page: 1, pageSize: 20 })).rejects.toBe(error)
  expect(request.get).toHaveBeenCalledWith('/api/admin/v1/payment/rechargepackage', {
    params: { page: 1, pageSize: 20 },
  })
})
