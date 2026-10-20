import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import * as api from '@/api/payment/rechargePackage'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import Page from '@/views/payment/rechargePackage/index.vue'
import { ApiError } from '@/utils/request'

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
  statuses: [
    { value: 0, label: '停用' },
    { value: 1, label: '启用' },
  ],
  defaults: { sortOrder: 0, isEnabled: 0 },
  limits: {
    name: 128,
    badge: 32,
    amountScale: 2,
    minimumAmount: '0.01',
    maximumAmount: '92233720368.54',
    sortMinimum: 0,
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
  vi.mocked(api.getRechargePackages).mockResolvedValue({
    list: [row],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(api.createRechargePackage).mockResolvedValue({ id: 13 })
  vi.mocked(api.updateRechargePackage).mockResolvedValue({})
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

async function openCreate(): Promise<VueWrapper> {
  const page = await mountPage()
  await page.get('[data-testid="payment-recharge-package-create"]').trigger('click')
  await flushPromises()
  return page
}

async function fillInput(testId: string, value: string): Promise<void> {
  const host = document.body.querySelector(`[data-testid="${testId}"]`)
  const input = host instanceof HTMLInputElement ? host : host?.querySelector('input')
  expect(input).toBeInstanceOf(HTMLInputElement)
  if (!(input instanceof HTMLInputElement)) throw new Error(`Missing input: ${testId}`)
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
  input.dispatchEvent(new Event('change', { bubbles: true }))
  input.dispatchEvent(new Event('blur', { bubbles: true }))
  await flushPromises()
}

async function save(): Promise<void> {
  document.body
    .querySelector<HTMLButtonElement>('[data-testid="payment-recharge-package-save"]')
    ?.click()
  await flushPromises()
}

it('removes package code and searches by package name', async () => {
  const page = await mountPage()
  expect(page.text()).not.toContain('编码')
  expect(page.find('[data-testid="payment-recharge-package-code"]').exists()).toBe(false)
  await page.get('[data-testid="payment-recharge-package-create"]').trigger('click')
  await flushPromises()
  expect(document.body.querySelector('[data-testid="payment-recharge-package-code"]')).toBeNull()
  expect(
    document.body
      .querySelector('[data-testid="payment-recharge-package-name"]')
      ?.getAttribute('placeholder'),
  ).toContain('基础充值')
})

it('marks required name and amount fields and shows their validation errors', async () => {
  const page = await mountPage()
  await page.get('[data-testid="payment-recharge-package-create"]').trigger('click')
  await flushPromises()
  const save = document.body.querySelector<HTMLButtonElement>(
    '[data-testid="payment-recharge-package-save"]',
  )
  expect(save).not.toBeNull()
  save?.click()
  await flushPromises()

  expect(api.createRechargePackage).not.toHaveBeenCalled()
  await vi.waitFor(() => {
    expect(document.body.textContent).toContain('请填写套餐名称')
    expect(document.body.textContent).toContain('请填写充值金额')
  })
})

it('uses a bounded input-number and submits a precise money string on create', async () => {
  const page = await openCreate()
  const amount = page
    .findAllComponents({ name: 'ElInputNumber' })
    .find((component) => component.attributes('data-testid') === 'payment-recharge-package-amount')
  expect(amount).toBeDefined()
  expect(amount?.props()).toMatchObject({
    min: 0.01,
    max: 92233720368.54,
    precision: 2,
    step: 0.01,
  })
  await fillInput('payment-recharge-package-name', ' 基础充值 ')
  await fillInput('payment-recharge-package-amount', '10.01')
  await save()
  expect(api.createRechargePackage).toHaveBeenCalledExactlyOnceWith({
    name: '基础充值',
    amount: '10.01',
    badge: null,
    sortOrder: 0,
    isEnabled: 0,
  })
  expect(api.getRechargePackages).toHaveBeenCalledTimes(2)
  expect(document.body.textContent).toContain('已保存')
})

it('retains entered values after create fails and allows a retry', async () => {
  vi.mocked(api.createRechargePackage).mockRejectedValueOnce(new ApiError(503, 'unavailable', 503))
  await openCreate()
  await fillInput('payment-recharge-package-name', '基础充值')
  await fillInput('payment-recharge-package-amount', '10.00')
  await save()
  expect(
    document.body.querySelector('[data-testid="payment-recharge-package-save"]'),
  ).not.toBeNull()
  await save()
  expect(api.createRechargePackage).toHaveBeenCalledTimes(2)
  expect(api.createRechargePackage).toHaveBeenLastCalledWith(
    expect.objectContaining({ name: '基础充值', amount: '10.00' }),
  )
})

it.each([
  ['0.01', '0.01'],
  ['10', '10.00'],
  ['92233720368.54', '92233720368.54'],
])('preserves the amount %s as an exact API string', async (input, amount) => {
  await openCreate()
  await fillInput('payment-recharge-package-name', '基础充值')
  await fillInput('payment-recharge-package-amount', input)
  await save()
  expect(api.createRechargePackage).toHaveBeenCalledExactlyOnceWith(
    expect.objectContaining({ amount }),
  )
})

it('does not send an amount that the user cleared', async () => {
  await openCreate()
  await fillInput('payment-recharge-package-name', '基础充值')
  await fillInput('payment-recharge-package-amount', '10')
  await fillInput('payment-recharge-package-amount', '')
  await save()
  expect(api.createRechargePackage).not.toHaveBeenCalled()
  await vi.waitFor(() => expect(document.body.textContent).toContain('请填写充值金额'))
})

it('edits a money string through the numeric control without changing the API contract', async () => {
  usePermissionStore(pinia).applySnapshot({
    roleCodes: [],
    menuTree: [],
    permissionCodes: ['payment:rechargePackage:list', 'payment:rechargePackage:update'],
  })
  const page = await mountPage()
  const edit = page.findAll('button').find((button) => button.text() === '编辑')
  expect(edit).toBeDefined()
  await edit?.trigger('click')
  await flushPromises()
  await fillInput('payment-recharge-package-amount', '20.01')
  await save()
  expect(api.createRechargePackage).not.toHaveBeenCalled()
  expect(api.updateRechargePackage).toHaveBeenCalledExactlyOnceWith(12, {
    name: '基础充值',
    amount: '20.01',
    badge: null,
    sortOrder: 0,
  })
})

it('sends only one create request when save is clicked twice during validation', async () => {
  let resolveCreate: ((result: { id: number }) => void) | undefined
  vi.mocked(api.createRechargePackage).mockReturnValueOnce(
    new Promise((resolve) => {
      resolveCreate = resolve
    }),
  )
  await openCreate()
  await fillInput('payment-recharge-package-name', '基础充值')
  await fillInput('payment-recharge-package-amount', '10')
  const button = document.body.querySelector<HTMLButtonElement>(
    '[data-testid="payment-recharge-package-save"]',
  )
  button?.click()
  button?.click()
  await flushPromises()
  expect(api.createRechargePackage).toHaveBeenCalledTimes(1)
  resolveCreate?.({ id: 13 })
  await flushPromises()
})
