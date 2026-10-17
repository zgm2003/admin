import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import * as orderApi from '@/api/payment/order'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import { getMyTransactions } from '@/api/payment/transaction'
import { getMyWallet } from '@/api/payment/wallet'
import WalletManagement from '@/views/payment/walletPortal/index.vue'

vi.mock('@/api/payment/order', () => ({
  getRechargeOptions: vi.fn(),
  getMyRechargeOrders: vi.fn(),
  createRecharge: vi.fn(),
  payRecharge: vi.fn(),
}))
vi.mock('@/api/payment/wallet', () => ({ getMyWallet: vi.fn() }))
vi.mock('@/api/payment/transaction', () => ({ getMyTransactions: vi.fn() }))
vi.mock('@/api/payment/redeemCode', () => ({ redeemWalletCode: vi.fn() }))

let wrapper: VueWrapper | undefined

beforeEach(() => {
  setLocale('zh-CN')
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: [] })
  vi.mocked(getMyWallet).mockResolvedValue({ hasWallet: true, balance: '12.00', currency: 'CNY' })
  vi.mocked(getMyTransactions).mockResolvedValue({ list: [], total: 0, page: 1, pageSize: 20 })
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue({
    available: false,
    packages: [],
    payMethods: [],
  } as never)
  vi.mocked(orderApi.getMyRechargeOrders).mockResolvedValue({
    list: [],
    total: 0,
    page: 1,
    pageSize: 20,
  })
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
})

it('renders no payment tab without view permission', async () => {
  wrapper = mount(WalletManagement, { global: { plugins: [ElementPlus, appI18n] } })
  await flushPromises()
  expect(wrapper.find('[data-testid="wallet-tabs"]').exists()).toBe(false)
  expect(wrapper.text()).toContain('暂无可用的钱包功能')
})

it('exposes only tabs whose page view permission is granted', async () => {
  usePermissionStore().applySnapshot({
    roleCodes: [],
    menuTree: [],
    permissionCodes: ['payment:myWallet:view'],
  })
  wrapper = mount(WalletManagement, { global: { plugins: [ElementPlus, appI18n] } })
  await flushPromises()
  expect(wrapper.find('[data-testid="wallet-tab-my-wallet"]').exists()).toBe(true)
  expect(wrapper.find('[data-testid="wallet-tab-recharge"]').exists()).toBe(false)
})

it('keeps both authorized tabs inside the wallet page', async () => {
  usePermissionStore().applySnapshot({
    roleCodes: [],
    menuTree: [],
    permissionCodes: ['payment:myWallet:view', 'payment:recharge:view'],
  })
  wrapper = mount(WalletManagement, { global: { plugins: [ElementPlus, appI18n] } })
  await flushPromises()
  expect(wrapper.find('[data-testid="wallet-tab-my-wallet"]').exists()).toBe(true)
  expect(wrapper.find('[data-testid="wallet-tab-recharge"]').exists()).toBe(true)
  expect(wrapper.find('[data-testid="wallet-tab-panel-my-wallet"]').exists()).toBe(true)
})
