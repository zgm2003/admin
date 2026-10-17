import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { Component } from 'vue'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import * as redeemApi from '@/api/payment/redeemCode'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import RedeemCodePage from '@/views/payment/redeemCode/index.vue'
import MyWalletPage from '@/views/payment/myWallet/index.vue'
import * as walletApi from '@/api/payment/wallet'
import * as transactionApi from '@/api/payment/transaction'

vi.mock('@/api/payment/redeemCode', () => ({
  getRedeemCodeOptions: vi.fn(),
  getRedeemCodes: vi.fn(),
  getRedeemCode: vi.fn(),
  lookupRedeemCode: vi.fn(),
  generateRedeemCodeBatch: vi.fn(),
  voidRedeemCodes: vi.fn(),
  exportRedeemCodes: vi.fn(),
  redeemWalletCode: vi.fn(),
}))
vi.mock('@/api/payment/wallet', () => ({ getMyWallet: vi.fn() }))
vi.mock('@/api/payment/transaction', () => ({ getMyTransactions: vi.fn() }))

const row = {
  id: 1,
  batchId: 2,
  batchNo: 'RBABC',
  codeMask: 'ABCD************NPQR',
  status: 1,
  amount: '10.00',
  expiresAt: null,
  usedBy: null,
  usedAt: null,
  createdAt: '2026-10-17T00:00:00Z',
  actions: { void: true, export: true },
}
const options = {
  limits: {
    maxBatchQuantity: 1000,
    maxVoidQuantity: 1000,
    maxExportQuantity: 10000,
    maxCodeLength: 128,
    codeLength: 20,
    remark: 255,
    minimumAmount: '0.01',
    maximumAmount: '92233720368.54',
    amountScale: 2,
  },
  defaults: { quantity: 1 },
  statuses: [
    { value: 1, label: '未使用' },
    { value: 2, label: '已使用' },
    { value: 3, label: '已作废' },
    { value: 4, label: '已过期' },
  ],
}

let wrappers: VueWrapper[] = []
beforeEach(() => {
  vi.resetAllMocks()
  setLocale('zh-CN')
  usePermissionStore(pinia).reset()
  vi.mocked(redeemApi.getRedeemCodeOptions).mockResolvedValue(options)
  vi.mocked(redeemApi.getRedeemCodes).mockResolvedValue({
    list: [row],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(redeemApi.generateRedeemCodeBatch).mockResolvedValue({
    batchId: 2,
    batchNo: 'RBABC',
    codes: ['ABCD2345EFGHJKLMNPQR'],
  })
  vi.mocked(redeemApi.redeemWalletCode).mockResolvedValue({
    transactionId: 11,
    transactionNo: 'PT-REDEEM-11',
    amount: '10.00',
    balance: '3.00',
    createdAt: '2026-10-17T00:00:00Z',
  })
  vi.mocked(walletApi.getMyWallet).mockResolvedValue({
    hasWallet: true,
    currency: 'CNY',
    balance: '-7',
  })
  vi.mocked(transactionApi.getMyTransactions).mockResolvedValue({
    list: [],
    total: 0,
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

it('loads masked codes and only exposes generation with its action permission', async () => {
  permissions(['payment:redeemCode:view', 'payment:redeemCode:list', 'payment:redeemCode:generate'])
  const wrapper = await page(RedeemCodePage)
  expect(redeemApi.getRedeemCodes).toHaveBeenCalledOnce()
  expect(wrapper.text()).toContain('ABCD************NPQR')
  expect(wrapper.find('[data-testid="payment-redeem-code-generate"]').exists()).toBe(true)
  permissions(['payment:redeemCode:view', 'payment:redeemCode:list'])
  await flushPromises()
  expect(wrapper.find('[data-testid="payment-redeem-code-generate"]').exists()).toBe(false)
})

it('offers personal redemption only with the dedicated action', async () => {
  permissions(['payment:myWallet:detail', 'payment:myWallet:list', 'payment:myWallet:redeem'])
  const wrapper = await page(MyWalletPage)
  expect(wrapper.find('[data-testid="payment-wallet-redeem"]').exists()).toBe(true)
})

it('redeems through the personal endpoint and refreshes wallet projections', async () => {
  permissions(['payment:myWallet:detail', 'payment:myWallet:list', 'payment:myWallet:redeem'])
  const wrapper = await page(MyWalletPage)
  await wrapper.get('[data-testid="payment-wallet-redeem"]').trigger('click')
  const code = document.querySelector<HTMLInputElement>('.el-dialog input')
  expect(code).not.toBeNull()
  if (code === null) return
  code.value = 'ABCD2345EFGHJKLMNPQR'
  code.dispatchEvent(new Event('input', { bubbles: true }))
  const submit = document.querySelector<HTMLButtonElement>(
    '[data-testid="payment-wallet-redeem-submit"]',
  )
  expect(submit).not.toBeNull()
  submit?.click()
  await flushPromises()
  expect(redeemApi.redeemWalletCode).toHaveBeenCalledWith('ABCD2345EFGHJKLMNPQR')
  expect(vi.mocked(walletApi.getMyWallet).mock.calls.length).toBeGreaterThanOrEqual(2)
  expect(vi.mocked(transactionApi.getMyTransactions).mock.calls.length).toBeGreaterThanOrEqual(2)
})
