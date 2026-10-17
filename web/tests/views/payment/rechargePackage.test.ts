import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import * as api from '@/api/payment/rechargePackage'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import Page from '@/views/payment/rechargePackage/index.vue'

vi.mock('@/api/payment/rechargePackage', () => ({
  getRechargePackages: vi.fn(),
  getRechargePackage: vi.fn(),
  getRechargePackageOptions: vi.fn(),
  createRechargePackage: vi.fn(),
  updateRechargePackage: vi.fn(),
  updateRechargePackageStatus: vi.fn(),
  deleteRechargePackage: vi.fn(),
}))

const options: api.RechargePackageOptions = {
  statuses: [{ value: 0, label: '停用' }, { value: 1, label: '启用' }],
  defaults: { sortOrder: 0, isEnabled: 0 },
  limits: {
    name: 128,
    badge: 32,
    amountScale: 2,
    minimumAmount: '0.01',
    maximumAmount: '100000000000.00',
    sortMinimum: -2147483648,
    sortMaximum: 2147483647,
  },
}
const row: api.RechargePackage = {
  id: 12,
  name: '基础充值',
  amount: '10.00',
  badge: null,
  sortOrder: 0,
  isEnabled: 0,
  actions: { update: true, status: true, delete: true },
  presentation: { status: '停用' },
  createdAt: '2026-10-17T00:00:00Z',
  updatedAt: '2026-10-17T00:00:00Z',
}
let wrapper: VueWrapper | undefined

beforeEach(() => {
  vi.resetAllMocks()
  setLocale('zh-CN')
  usePermissionStore(pinia).reset()
  usePermissionStore(pinia).applySnapshot({
    roleCodes: [],
    menuTree: [],
    permissionCodes: ['payment:rechargePackage:list', 'payment:rechargePackage:create'],
  })
  vi.mocked(api.getRechargePackageOptions).mockResolvedValue(options)
  vi.mocked(api.getRechargePackages).mockResolvedValue({ list: [row], total: 1, page: 1, pageSize: 20 })
  vi.mocked(api.createRechargePackage).mockResolvedValue({ id: 13 })
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
})

async function mountPage(): Promise<VueWrapper> {
  wrapper = mount(Page, { global: { plugins: [ElementPlus, pinia, appI18n] } })
  await flushPromises()
  return wrapper
}

it('removes package code and searches by package name', async () => {
  const page = await mountPage()
  expect(page.text()).not.toContain('编码')
  expect(page.find('[data-testid="payment-recharge-package-code"]').exists()).toBe(false)
  expect(page.get('[data-testid="payment-recharge-package-name"]').attributes('placeholder')).toContain('基础充值')
})

it('marks required name and amount fields and shows their validation errors', async () => {
  const page = await mountPage()
  await page.get('[data-testid="payment-recharge-package-create"]').trigger('click')
  await flushPromises()
  await page.get('[data-testid="payment-recharge-package-save"]').trigger('click')
  await flushPromises()

  expect(page.text()).toContain('请填写套餐名称')
  expect(page.text()).toContain('请填写充值金额')
  expect(api.createRechargePackage).not.toHaveBeenCalled()
})
