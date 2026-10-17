import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { Component } from 'vue'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import * as orderApi from '@/api/payment/order'
import * as transactionApi from '@/api/payment/transaction'
import * as walletApi from '@/api/payment/wallet'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import WalletPage from '@/views/payment/wallet/index.vue'
import MyWalletPage from '@/views/payment/myWallet/index.vue'
import TransactionPage from '@/views/payment/transaction/index.vue'
import OrderPage from '@/views/payment/order/index.vue'
import RechargePage from '@/views/payment/recharge/index.vue'

vi.mock('@/api/payment/wallet', () => ({
  getWallets: vi.fn(),
  getWallet: vi.fn(),
  getMyWallet: vi.fn(),
}))
vi.mock('@/api/payment/transaction', () => ({
  getTransactions: vi.fn(),
  getTransaction: vi.fn(),
  getMyTransactions: vi.fn(),
}))
vi.mock('@/api/payment/order', () => ({
  getOrders: vi.fn(),
  getOrder: vi.fn(),
  getRechargeOptions: vi.fn(),
  getMyRechargeOrders: vi.fn(),
  createRecharge: vi.fn(),
  payRecharge: vi.fn(),
}))
vi.mock('@/api/payment/callbackEvent', () => ({
  getOrderCallbackEvents: vi.fn(),
  getOrderCallbackEvent: vi.fn(),
}))

const wallet = {
  userId: 7,
  username: 'fixture-user',
  currency: 'CNY',
  balance: '-7.00000001',
  createdAt: '2026-10-16T00:00:00Z',
  updatedAt: '2026-10-16T00:00:00Z',
}
const transaction = {
  id: 3,
  transactionNo: 'PT202610160000000000000001',
  sourceType: 1,
  amount: '10.00',
  balanceBefore: '-20.00',
  balanceAfter: '-10.00',
  createdAt: '2026-10-16T00:00:00Z',
}
const order = {
  id: 9,
  orderNo: 'PO202610160000000000000001',
  packageName: 'Starter',
  amount: '10.00',
  status: 1,
  statusLabel: '待支付',
  expiresAt: '2026-10-16T01:00:00Z',
  paidAt: null,
  creditedAt: null,
  closedAt: null,
  actions: { pay: true, sync: false, close: false },
}

let wrappers: VueWrapper[] = []
beforeEach(() => {
  window.history.replaceState({}, '', '/')
  vi.resetAllMocks()
  setLocale('zh-CN')
  usePermissionStore(pinia).reset()
  vi.mocked(walletApi.getWallets).mockResolvedValue({
    list: [wallet],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(walletApi.getWallet).mockResolvedValue(wallet)
  vi.mocked(walletApi.getMyWallet).mockResolvedValue({
    hasWallet: true,
    currency: 'CNY',
    balance: '-7.00000001',
  })
  vi.mocked(transactionApi.getTransactions).mockResolvedValue({
    list: [
      {
        ...transaction,
        userId: 7,
        username: 'fixture-user',
        platformId: 1,
        platformCode: 'admin',
        orderId: 9,
      },
    ],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(transactionApi.getMyTransactions).mockResolvedValue({
    list: [transaction],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(transactionApi.getTransaction).mockResolvedValue({
    ...transaction,
    userId: 7,
    username: 'fixture-user',
    platformId: 1,
    platformCode: 'admin',
    orderId: 9,
  })
  vi.mocked(orderApi.getOrders).mockResolvedValue({
    list: [order],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(orderApi.getOrder).mockResolvedValue(order)
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue({
    payMethods: [{ value: 1, label: 'PC' }],
    packages: [{ id: 1, name: 'Starter', amount: '10.00' }],
    available: false,
  })
  vi.mocked(orderApi.getMyRechargeOrders).mockResolvedValue({
    list: [order],
    total: 1,
    page: 1,
    pageSize: 20,
  })
})
afterEach(() => {
  wrappers.forEach((wrapper) => wrapper.unmount())
  wrappers = []
  document.body.innerHTML = ''
})
function permissions(codes: string[]): void {
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: codes })
}
async function page(component: Component): Promise<VueWrapper> {
  const wrapper = mount(component, { global: { plugins: [ElementPlus, pinia, appI18n] } })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

it('keeps management wallet fields separate and displays signed balances', async () => {
  permissions(['payment:wallet:list'])
  const wrapper = await page(WalletPage)
  expect(walletApi.getWallets).toHaveBeenCalledOnce()
  expect(wrapper.text()).toContain('-7.00000001')
  expect(wrapper.text()).toContain('fixture-user')
  expect(wrapper.find('[data-testid="payment-wallet-detail"]').exists()).toBe(false)
})

it('renders only personal wallet and transaction projections', async () => {
  permissions(['payment:myWallet:detail', 'payment:myWallet:list'])
  const wrapper = await page(MyWalletPage)
  expect(wrapper.text()).toContain('-7.00000001')
  expect(wrapper.text()).toContain('PT202610160000000000000001')
  expect(wrapper.text()).not.toContain('fixture-user')
  expect(wrapper.text()).not.toContain('用户 ID')
})

it('loads management transactions only with the list action', async () => {
  permissions(['payment:transaction:list'])
  const wrapper = await page(TransactionPage)
  expect(transactionApi.getTransactions).toHaveBeenCalledOnce()
  expect(wrapper.text()).toContain('fixture-user')
  expect(wrapper.findAll('button').some((button) => button.text() === '查看')).toBe(false)
})

it('renders admin orders and keeps callback audit behind its own permission', async () => {
  permissions(['payment:order:list', 'payment:order:detail'])
  const wrapper = await page(OrderPage)
  expect(orderApi.getOrders).toHaveBeenCalledOnce()
  expect(wrapper.text()).toContain('PO202610160000000000000001')
  expect(wrapper.text()).not.toContain('回调审计')
})

it('does not offer checkout when backend marks recharge unavailable', async () => {
  permissions(['payment:recharge:create', 'payment:recharge:list'])
  const wrapper = await page(RechargePage)
  expect(orderApi.getRechargeOptions).toHaveBeenCalledOnce()
  expect(
    wrapper.get('[data-testid="payment-recharge-create"]').attributes('disabled'),
  ).toBeDefined()
  expect(wrapper.text()).toContain('支付入口当前不可用')
})

it('submits the current route as returnUrl without asking the user to configure it', async () => {
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue({
    payMethods: [{ value: 1, label: 'PC' }],
    packages: [{ id: 1, name: 'Starter', amount: '10.00' }],
    available: true,
  })
  vi.mocked(orderApi.createRecharge).mockResolvedValue({
    order,
    checkout: 'https://pay.example.test/checkout',
  })
  permissions(['payment:recharge:create'])
  const wrapper = await page(RechargePage)
  window.history.replaceState({}, '', '/payment/recharge?from=test#private-fragment')
  await wrapper.get('[data-testid="payment-recharge-create"]').trigger('click')
  await flushPromises()
  expect(orderApi.createRecharge).toHaveBeenCalledWith({
    requestId: expect.any(String),
    packageId: 1,
    payMethod: 1,
    returnUrl: `${window.location.origin}/payment/recharge?from=test`,
  })
  expect(wrapper.find('[data-testid="payment-recharge-return-url"]').exists()).toBe(false)
})

it('uses a platform-specific placeholder in management filters', async () => {
  permissions(['payment:order:list'])
  const wrapper = await page(OrderPage)
  const placeholders = wrapper.findAll('input').map((input) => input.attributes('placeholder'))
  expect(placeholders).toContain('请输入平台 ID')
})
