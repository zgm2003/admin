import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/payment/config'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import Page from '@/views/payment/config/index.vue'
vi.mock('element-plus/es/components/message-box/index', () => ({
  ElMessageBox: { confirm: vi.fn(), install: vi.fn() },
}))
vi.mock('@/api/payment/config', () => ({
  getPaymentConfigs: vi.fn(),
  getPaymentConfig: vi.fn(),
  getPaymentConfigOptions: vi.fn(),
  createPaymentConfig: vi.fn(),
  updatePaymentConfig: vi.fn(),
  publishPaymentConfigVersion: vi.fn(),
  updatePaymentConfigStatus: vi.fn(),
  deletePaymentConfig: vi.fn(),
  testPaymentConfig: vi.fn(),
}))
const row: api.PaymentConfig = {
  id: 7,
  code: 'alipay_main',
  name: 'Main',
  provider: 1,
  environment: 1,
  appId: '20260001',
  sellerId: '20880001',
  currentVersion: 1,
  sortOrder: 0,
  isEnabled: 0,
  notifyUrl: 'https://pay.example.test/api/v1/payment/callback/alipay/7/1',
  notifyUrlTemplate: 'https://pay.example.test/api/v1/payment/callback/alipay/{configId}/{version}',
  payMethods: [1, 2],
  credentialConfigured: true,
  certificates: {
    app: {
      subject: 'app',
      issuer: 'root',
      serialNumber: '2',
      notBefore: '2026-01-01T00:00:00Z',
      notAfter: '2027-01-01T00:00:00Z',
    },
    alipay: {
      subject: 'alipay',
      issuer: 'root',
      serialNumber: '3',
      notBefore: '2026-01-01T00:00:00Z',
      notAfter: '2027-01-01T00:00:00Z',
    },
    roots: [],
  },
  actions: { update: true, status: true, delete: true, test: true },
  presentation: {
    provider: '服务端通道',
    environment: '服务端环境',
    payMethods: [{ value: 1, label: '服务端方式' }],
    status: '服务端状态',
  },
  createdAt: '2026-10-13T00:00:00Z',
  updatedAt: '2026-10-13T00:00:00Z',
}
const options: api.PaymentConfigOptions = {
  providers: [{ value: 1, label: '支付宝' }],
  environments: [
    { value: 1, label: '沙箱' },
    { value: 2, label: '生产' },
  ],
  payMethods: [
    { value: 1, label: 'PC' },
    { value: 2, label: 'H5' },
  ],
  statuses: [
    { value: 0, label: '停用' },
    { value: 1, label: '启用' },
  ],
  defaults: { provider: 1, environment: 1, payMethods: [1, 2], sortOrder: 0, isEnabled: 0 },
  limits: {
    code: 64,
    name: 128,
    identity: 64,
    notifyUrl: 2048,
    certificateBytes: 65536,
    privateKeyBytes: 16384,
    sortMinimum: -2147483648,
    sortMaximum: 2147483647,
  },
  notifyPathTemplate: '/api/v1/payment/callback/alipay/{configId}/{version}',
}
let wrapper: VueWrapper | undefined
beforeEach(() => {
  vi.resetAllMocks()
  setLocale('zh-CN')
  usePermissionStore(pinia).reset()
  vi.mocked(api.getPaymentConfigs).mockResolvedValue({
    list: [row],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  vi.mocked(api.getPaymentConfig).mockResolvedValue(row)
  vi.mocked(api.getPaymentConfigOptions).mockResolvedValue(options)
  vi.mocked(api.updatePaymentConfig).mockResolvedValue({})
})
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.body.innerHTML = ''
})
function permissions(codes: string[]) {
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: codes })
}
async function page() {
  wrapper = mount(Page, { global: { plugins: [ElementPlus, pinia, appI18n] } })
  await flushPromises()
  return wrapper
}
it('page view alone grants no list or mutation', async () => {
  permissions(['payment:config:view'])
  const w = await page()
  expect(api.getPaymentConfigs).not.toHaveBeenCalled()
  expect(w.find('[data-testid="payment-config-create"]').exists()).toBe(false)
})
it('uses backend presentation and exact action permissions', async () => {
  permissions(['payment:config:list'])
  const w = await page()
  expect(w.text()).toContain('服务端通道')
  expect(w.text()).toContain('服务端环境')
  expect(w.find('[data-testid="payment-config-edit"]').exists()).toBe(false)
})
it('metadata edit saves only name and sort order, without reuploading credentials', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  const w = await page()
  await w.get('[data-testid="payment-config-edit"]').trigger('click')
  await flushPromises()
  await w.get('[data-testid="payment-config-name"]').setValue('Renamed')
  expect(w.find('[data-testid="payment-config-private-key"]').exists()).toBe(false)
  await w.get('[data-testid="payment-config-save"]').trigger('click')
  await flushPromises()
  expect(api.updatePaymentConfig).toHaveBeenCalledWith(7, { name: 'Renamed', sortOrder: 0 })
})
it('clears stale rows on failed refresh', async () => {
  permissions(['payment:config:list'])
  const w = await page()
  vi.mocked(api.getPaymentConfigs).mockRejectedValue(new Error('offline'))
  await w.getComponent({ name: 'AppTable' }).vm.$emit('refresh')
  await flushPromises()
  expect(w.text()).toContain('offline')
  expect(w.text()).not.toContain('服务端通道')
})
it('blocks writes after permission revocation while a confirmation is open', async () => {
  permissions(['payment:config:list', 'payment:config:delete'])
  let confirm: (() => void) | undefined
  vi.mocked(ElMessageBox.confirm).mockImplementation(
    () =>
      new Promise<Awaited<ReturnType<typeof ElMessageBox.confirm>>>((resolve) => {
        confirm = () =>
          resolve('confirm' as unknown as Awaited<ReturnType<typeof ElMessageBox.confirm>>)
      }),
  )
  const w = await page()
  await w.get('[data-testid="payment-config-delete"]').trigger('click')
  permissions(['payment:config:list'])
  confirm?.()
  await flushPromises()
  expect(api.deletePaymentConfig).not.toHaveBeenCalled()
})
it('admits only one mutation while confirmation and API are pending', async () => {
  permissions(['payment:config:list', 'payment:config:delete', 'payment:config:status'])
  let confirm: (() => void) | undefined
  vi.mocked(ElMessageBox.confirm).mockImplementation(
    () =>
      new Promise<Awaited<ReturnType<typeof ElMessageBox.confirm>>>((resolve) => {
        confirm = () =>
          resolve('confirm' as unknown as Awaited<ReturnType<typeof ElMessageBox.confirm>>)
      }),
  )
  const w = await page()
  await w.get('[data-testid="payment-config-delete"]').trigger('click')
  await w.get('[data-testid="payment-config-status"]').trigger('click')
  expect(ElMessageBox.confirm).toHaveBeenCalledOnce()
  confirm?.()
  await flushPromises()
})
it('refreshes committed-pending data without claiming success or dropping credential drafts', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  const w = await page()
  await w.get('[data-testid="payment-config-version"]').trigger('click')
  await flushPromises()
  await w.get('textarea[data-testid="payment-config-private-key"]').setValue('local-secret-draft')
  vi.mocked(api.publishPaymentConfigVersion).mockRejectedValue(
    new Error('配置已提交，缓存同步待恢复'),
  )
  vi.mocked(api.getPaymentConfigs).mockResolvedValue({
    list: [{ ...row, currentVersion: 2 }],
    total: 1,
    page: 1,
    pageSize: 20,
  })
  await w.get('[data-testid="payment-config-save"]').trigger('click')
  await flushPromises()
  expect(w.text()).toContain('配置已提交，缓存同步待恢复')
  expect(w.text()).toContain('v2')
  expect(w.text()).not.toContain('保存成功')
  expect(
    (w.get('textarea[data-testid="payment-config-private-key"]').element as HTMLTextAreaElement)
      .value,
  ).toBe('local-secret-draft')
})
async function certificateFile(
  w: VueWrapper,
  field: string,
  name: string,
  content: string,
  size?: number,
) {
  const file = new File([content], name, { type: 'application/x-pem-file' })
  Object.defineProperty(file, 'text', { value: async () => content })
  if (size !== undefined) Object.defineProperty(file, 'size', { value: size })
  const input = w.get(`[data-testid="payment-config-import-${field}"]`)
  Object.defineProperty(input.element, 'files', { configurable: true, value: [file] })
  await input.trigger('change')
  await flushPromises()
}
it('imports the three reference certificates locally, preserves numeric methods and retains blank credentials', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  vi.mocked(api.publishPaymentConfigVersion).mockResolvedValue({})
  const w = await page()
  await w.get('[data-testid="payment-config-version"]').trigger('click')
  await flushPromises()
  const pem = '-----BEGIN CERTIFICATE-----\nZml4dHVyZQ==\n-----END CERTIFICATE-----'
  for (const field of ['appCertPEM', 'alipayCertPEM', 'rootCertPEM']) {
    await certificateFile(w, field, `${field}.crt`, pem)
    expect(
      (w.get(`textarea[data-testid="payment-config-${field}"]`).element as HTMLTextAreaElement)
        .value,
    ).toBe(pem)
  }
  await w.get('[data-testid="payment-config-save"]').trigger('click')
  await flushPromises()
  expect(api.publishPaymentConfigVersion).toHaveBeenCalledWith(7, {
    privateKey: null,
    appCertPEM: pem,
    alipayCertPEM: pem,
    rootCertPEM: pem,
    notifyUrl: row.notifyUrlTemplate,
    payMethods: [1, 2],
  })
})
it('rejects oversized or malformed imported files without losing an existing draft', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  const w = await page()
  await w.get('[data-testid="payment-config-version"]').trigger('click')
  await flushPromises()
  const input = w.get('textarea[data-testid="payment-config-appCertPEM"]')
  await input.setValue('existing draft')
  await certificateFile(w, 'appCertPEM', 'bad.crt', 'not a certificate')
  expect((input.element as HTMLTextAreaElement).value).toBe('existing draft')
  expect(w.text()).toContain('证书文件')
  await certificateFile(w, 'appCertPEM', 'large.crt', 'x', 65537)
  expect((input.element as HTMLTextAreaElement).value).toBe('existing draft')
  await w.get('[data-testid="payment-config-save"]').trigger('click')
  await flushPromises()
  expect(api.publishPaymentConfigVersion).not.toHaveBeenCalled()
})
