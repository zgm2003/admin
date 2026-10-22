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
    packages: [{ id: 1, name: 'Starter', amount: '10.00', badge: '热门' }],
    limits: { minimumAmount: '10.00', maximumAmount: '999999.99', amountScale: 2 },
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
  expect(wrapper.findAll('button').some((button) => button.text() === '查询')).toBe(true)
  expect(wrapper.findAll('button').some((button) => button.text() === '资金流水')).toBe(false)
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
    packages: [{ id: 1, name: 'Starter', amount: '10.00', badge: '热门' }],
    limits: { minimumAmount: '10.00', maximumAmount: '999999.99', amountScale: 2 },
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

function rechargeOptions(packages = [{ id: 1, name: 'Starter', amount: '5.00', badge: '热门' }]) {
  return {
    packages,
    payMethods: [{ value: 1, label: 'PC' }],
    available: true,
    limits: { minimumAmount: '10.00', maximumAmount: '999999.99', amountScale: 2 },
  }
}

it('shows selectable package cards including badges and does not apply the custom minimum to packages', async () => {
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue(rechargeOptions())
  vi.mocked(orderApi.createRecharge).mockResolvedValue({ order, checkout: '' })
  permissions(['payment:recharge:create'])
  const wrapper = await page(RechargePage)
  const card = wrapper.get('[data-testid="payment-recharge-package-1"]')
  expect(card.attributes('role')).toBe('radio')
  expect(card.attributes('aria-checked')).toBe('true')
  expect(card.text()).toContain('热门')
  expect(card.text()).toContain('5.00')
  await wrapper.get('[data-testid="payment-recharge-create"]').trigger('click')
  expect(orderApi.createRecharge).toHaveBeenCalledWith(expect.objectContaining({ packageId: 1 }))
})

it('selects custom recharge with the keyboard and submits the scaled amount without packageId', async () => {
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue(rechargeOptions())
  vi.mocked(orderApi.createRecharge).mockResolvedValue({ order, checkout: '' })
  permissions(['payment:recharge:create'])
  const wrapper = await page(RechargePage)
  await wrapper
    .get('[data-testid="payment-recharge-package-1"]')
    .trigger('keydown', { key: 'ArrowRight' })
  expect(wrapper.get('[data-testid="payment-recharge-custom"]').attributes('aria-checked')).toBe(
    'true',
  )
  const amount = wrapper.getComponent({ name: 'ElInputNumber' })
  expect(amount.props('controlsPosition')).toBe('right')
  expect(amount.props('min')).toBe(10)
  amount.vm.$emit('update:modelValue', 12.3)
  await flushPromises()
  expect(wrapper.get('[data-testid="payment-recharge-total"]').text()).toContain('12.30')
  await wrapper.get('[data-testid="payment-recharge-create"]').trigger('click')
  expect(orderApi.createRecharge).toHaveBeenCalledWith({
    requestId: expect.any(String),
    amount: '12.30',
    payMethod: 1,
    returnUrl: window.location.href,
  })
})

it('allows custom recharge without packages, blocks empty amounts and clamps bounds', async () => {
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue(rechargeOptions([]))
  permissions(['payment:recharge:create'])
  const wrapper = await page(RechargePage)
  expect(wrapper.text()).toContain('暂无充值套餐')
  const amount = wrapper.getComponent({ name: 'ElInputNumber' })
  for (const value of [undefined, null]) {
    amount.vm.$emit('update:modelValue', value)
    await flushPromises()
    expect(
      wrapper.get('[data-testid="payment-recharge-create"]').attributes('disabled'),
    ).toBeDefined()
    await wrapper.get('form').trigger('submit')
  }
  for (const [value, expected] of [
    [9, 10],
    [1000000, 999999.99],
  ]) {
    amount.vm.$emit('update:modelValue', value)
    await flushPromises()
    expect(amount.props('modelValue')).toBe(expected)
  }
  expect(orderApi.createRecharge).not.toHaveBeenCalled()
})

it('prevents duplicate recharge submissions and leaves a failed request retryable', async () => {
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue(rechargeOptions())
  let rejectRequest: (reason: Error) => void = () => undefined
  vi.mocked(orderApi.createRecharge).mockImplementationOnce(
    () =>
      new Promise((_resolve, reject) => {
        rejectRequest = reject
      }),
  )
  permissions(['payment:recharge:create'])
  const wrapper = await page(RechargePage)
  await wrapper.get('form').trigger('submit')
  await wrapper.get('form').trigger('submit')
  expect(orderApi.createRecharge).toHaveBeenCalledOnce()
  rejectRequest(new Error('request layer notified'))
  await flushPromises()
  expect(
    wrapper.get('[data-testid="payment-recharge-create"]').attributes('disabled'),
  ).toBeUndefined()
})

it('ignores old recharge options after locale changes or create permission is revoked', async () => {
  let finishOld: (value: orderApi.RechargeOptions) => void = () => undefined
  vi.mocked(orderApi.getRechargeOptions).mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finishOld = resolve
      }),
  )
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue(
    rechargeOptions([{ id: 2, name: 'Current', amount: '20.00', badge: 'New' }]),
  )
  permissions(['payment:recharge:create'])
  const wrapper = await page(RechargePage)
  setLocale('en-US')
  await flushPromises()
  finishOld(rechargeOptions())
  await flushPromises()
  expect(wrapper.find('[data-testid="payment-recharge-package-1"]').exists()).toBe(false)
  expect(wrapper.find('[data-testid="payment-recharge-package-2"]').exists()).toBe(true)
  permissions([])
  await flushPromises()
  expect(wrapper.find('[data-testid="payment-recharge-create"]').exists()).toBe(false)
})

it('does not load options or create a recharge without its action permission', async () => {
  permissions(['payment:recharge:view', 'payment:recharge:list'])
  const wrapper = await page(RechargePage)
  expect(orderApi.getRechargeOptions).not.toHaveBeenCalled()
  expect(wrapper.find('[data-testid="payment-recharge-create"]').exists()).toBe(false)
})

it('keeps an uncertain request id on retry and changes it when the recharge intent changes', async () => {
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue(rechargeOptions())
  vi.mocked(orderApi.createRecharge).mockRejectedValue(new Error('request layer notified'))
  permissions(['payment:recharge:create'])
  const wrapper = await page(RechargePage)
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  const first = vi.mocked(orderApi.createRecharge).mock.calls[0]?.[0]
  expect(first?.requestId).toEqual(expect.any(String))
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  expect(vi.mocked(orderApi.createRecharge).mock.calls[1]?.[0]).toEqual(first)
  await wrapper.get('[data-testid="payment-recharge-custom"]').trigger('click')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  const changed = vi.mocked(orderApi.createRecharge).mock.calls[2]?.[0]
  expect(changed?.requestId).not.toBe(first?.requestId)
  expect(changed).toMatchObject({ amount: '10.00' })
})

it('shows a retryable options error without exposing a usable payment action', async () => {
  vi.mocked(orderApi.getRechargeOptions).mockRejectedValueOnce(new Error('request layer notified'))
  vi.mocked(orderApi.getRechargeOptions).mockResolvedValue(rechargeOptions([]))
  permissions(['payment:recharge:create'])
  const wrapper = await page(RechargePage)
  expect(wrapper.text()).toContain('充值选项暂不可用')
  expect(wrapper.find('[data-testid="payment-recharge-create"]').exists()).toBe(false)
  await wrapper.get('button').trigger('click')
  await flushPromises()
  expect(wrapper.find('[data-testid="payment-recharge-amount"]').exists()).toBe(true)
})
