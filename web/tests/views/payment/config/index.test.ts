import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import * as api from '@/api/payment/config'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { usePermissionStore } from '@/store/permission'
import { ApiError } from '@/utils/request'
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
  name: 'Main',
  provider: 1,
  environment: 1,
  appId: '20260001',
  sellerId: '20880001',
  currentVersion: 1,
  sortOrder: 0,
  isEnabled: 0,
  notifyUrl: 'https://pay.example.test/api/admin/v1/payment/callback/alipay/7/1',
  notifyUrlTemplate: 'https://pay.example.test/api/admin/v1/payment/callback/alipay/{configId}/{version}',
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
  defaults: {
    provider: 1,
    environment: 1,
    payMethods: [1, 2],
    sortOrder: 0,
    isEnabled: 0,
    notifyUrl: 'https://HOST/api/admin/v1/payment/callback/alipay/{configId}/{version}',
  },
  limits: {
    name: 128,
    identity: 64,
    notifyUrl: 2048,
    certificateBytes: 65536,
    privateKeyBytes: 16384,
    sortMinimum: -2147483648,
    sortMaximum: 2147483647,
  },
  notifyPathTemplate: '/api/admin/v1/payment/callback/alipay/{configId}/{version}',
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
it('renders localized placeholders for payment identifiers and credential inputs', async () => {
  permissions(['payment:config:list', 'payment:config:create'])
  const w = await page()
  await w.get('[data-testid="payment-config-create"]').trigger('click')
  await flushPromises()
  const notifyInput = w.get<HTMLInputElement>('input[data-testid="payment-config-notify-url"]')
  expect(notifyInput.element.value).toBe(
    'https://HOST/api/admin/v1/payment/callback/alipay/{configId}/{version}',
  )
  await notifyInput.setValue(row.notifyUrlTemplate)
  setLocale('en-US')
  await flushPromises()
  expect(notifyInput.element.value).toBe(row.notifyUrlTemplate)
  setLocale('zh-CN')
  await flushPromises()
  expect(w.find('[data-testid="payment-config-code"]').exists()).toBe(false)
  expect(w.text()).not.toContain('编码')
  expect(w.text()).toContain('APPID')
  expect(w.text()).toContain('绑定的商家账号（PID）')
  expect(w.text()).toContain('配置名称')
  expect(w.text()).not.toContain('应用名称')
  expect(w.get('input[placeholder="例如：支付宝生产主配置"]').attributes('placeholder')).toBe(
    '例如：支付宝生产主配置',
  )
  expect(w.get('input[placeholder="请输入支付宝 APPID（纯数字）"]').attributes('placeholder')).toBe(
    '请输入支付宝 APPID（纯数字）',
  )
  expect(
    w.get('input[placeholder="请输入绑定的商家账号（PID，纯数字）"]').attributes('placeholder'),
  ).toBe('请输入绑定的商家账号（PID，纯数字）')
  expect(
    w.get('textarea[data-testid="payment-config-private-key"]').attributes('placeholder'),
  ).toContain('RSA2')
  expect(
    w.get('textarea[data-testid="payment-config-appCertPEM"]').attributes('placeholder'),
  ).toContain('PEM')
  expect(
    w.get('textarea[data-testid="payment-config-alipayCertPEM"]').attributes('placeholder'),
  ).toContain('PEM')
  expect(
    w.get('textarea[data-testid="payment-config-rootCertPEM"]').attributes('placeholder'),
  ).toContain('PEM')
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
  expect(w.find('.el-alert--success').exists()).toBe(false)
  const notifications = document.body.querySelectorAll('.el-notification')
  expect(notifications).toHaveLength(1)
  expect(notifications[0]?.textContent).toContain(appI18n.global.t('payment.config.saved'))
})
it('keeps a failed save editable without repeating the request error inside the page or dialog', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  vi.mocked(api.updatePaymentConfig).mockRejectedValueOnce(new ApiError(10001, '保存被拒绝'))
  const w = await page()
  await w.get('[data-testid="payment-config-edit"]').trigger('click')
  await flushPromises()
  await w.get('[data-testid="payment-config-name"]').setValue('My draft')
  await w.get('[data-testid="payment-config-save"]').trigger('click')
  await flushPromises()
  expect(w.text()).not.toContain('保存被拒绝')
  expect(w.find('.el-alert--error').exists()).toBe(false)
  expect(w.get<HTMLInputElement>('[data-testid="payment-config-name"]').element.value).toBe('My draft')
  expect(w.get('[data-testid="payment-config-save"]').attributes('disabled')).toBeUndefined()
  await w.get('[data-testid="payment-config-save"]').trigger('click')
  await flushPromises()
  expect(api.updatePaymentConfig).toHaveBeenCalledTimes(2)
})
it('validates the server-provided HTTPS callback template before submitting credentials', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  const w = await page()
  await w.get('[data-testid="payment-config-version"]').trigger('click')
  await flushPromises()
  const input = w.get('input[data-testid="payment-config-notify-url"]')
  expect((input.element as HTMLInputElement).value).toBe(row.notifyUrlTemplate)
  for (const invalid of [
    'http://abc',
    'https://pay.example.test',
    'https://HOST/api/admin/v1/payment/callback/alipay/{configId}/{version}',
    row.notifyUrlTemplate + '?token=secret',
  ]) {
    await input.setValue(invalid)
    await w.get('[data-testid="payment-config-save"]').trigger('click')
    await flushPromises()
    expect(api.publishPaymentConfigVersion).not.toHaveBeenCalled()
    await vi.waitFor(() => {
      const message = w.get('.el-form-item__error').text()
      expect(message).toContain('HTTPS')
      expect(message).toContain('{configId}/{version}')
    })
  }
  await input.setValue(row.notifyUrlTemplate)
  vi.mocked(api.publishPaymentConfigVersion).mockResolvedValue({})
  await w.get('[data-testid="payment-config-save"]').trigger('click')
  await flushPromises()
  expect(api.publishPaymentConfigVersion).toHaveBeenCalledOnce()
})
it('clears stale rows on failed refresh', async () => {
  permissions(['payment:config:list'])
  const w = await page()
  vi.mocked(api.getPaymentConfigs).mockRejectedValue(new Error('offline'))
  await w.getComponent({ name: 'AppTable' }).vm.$emit('refresh')
  await flushPromises()
  expect(w.text()).not.toContain('offline')
  expect(w.getComponent({ name: 'AppTable' }).props('resultState')).toBe('error')
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
  expect(w.text()).not.toContain('配置已提交，缓存同步待恢复')
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
  const input = w.get(`[data-testid="payment-config-import-${field}"] input[type="file"]`)
  Object.defineProperty(input.element, 'files', { configurable: true, value: [file] })
  await input.trigger('change')
  await flushPromises()
}
it.each(['create', 'version'] as const)(
  '%s submits distinct imported certificates in their own labeled fields without adding certificate blocks',
  async (mode) => {
    permissions(['payment:config:list', 'payment:config:create', 'payment:config:update'])
    vi.mocked(api.createPaymentConfig).mockResolvedValue({ id: 8 })
    vi.mocked(api.publishPaymentConfigVersion).mockResolvedValue({})
    const w = await page()
    await w.get(`[data-testid="payment-config-${mode}"]`).trigger('click')
    await flushPromises()
    if (mode === 'create') {
      await w.get('[data-testid="payment-config-name"]').setValue('Local fixture')
      await w.get('[data-testid="payment-config-app-id"]').setValue('20260001')
      await w.get('[data-testid="payment-config-seller-id"]').setValue('20880001')
      await w.get('[data-testid="payment-config-private-key"]').setValue('local-fixture-key')
      await w.get('[data-testid="payment-config-notify-url"]').setValue(row.notifyUrlTemplate)
    }
    const certificates = {
      appCertPEM: '-----BEGIN CERTIFICATE-----\nYXBw\n-----END CERTIFICATE-----',
      alipayCertPEM:
        '-----BEGIN CERTIFICATE-----\nYWxpcGF5\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\naW50ZXJtZWRpYXRl\n-----END CERTIFICATE-----',
      rootCertPEM: '-----BEGIN CERTIFICATE-----\ncm9vdA==\n-----END CERTIFICATE-----',
    }
    const fields = [
      ['appCertPEM', '应用公钥证书'],
      ['alipayCertPEM', '支付宝公钥证书'],
      ['rootCertPEM', '支付宝根证书'],
    ] as const
    for (const [field, label] of fields) {
      const input = w.get(`textarea[data-testid="payment-config-${field}"]`)
      expect(
        input.element
          .closest('.el-form-item')
          ?.querySelector('.payment-config-form__credential-label-text')
          ?.textContent?.trim(),
      ).toBe(label)
      await certificateFile(w, field, `${field}.crt`, certificates[field])
    }
    for (const [field] of fields) {
      expect(
        w.get<HTMLTextAreaElement>(`textarea[data-testid="payment-config-${field}"]`).element.value,
      ).toBe(certificates[field])
    }
    await w.get('[data-testid="payment-config-save"]').trigger('click')
    await flushPromises()
    const credentials = {
      ...certificates,
      privateKey: mode === 'create' ? 'local-fixture-key' : null,
      notifyUrl: row.notifyUrlTemplate,
      payMethods: [1, 2],
    }
    if (mode === 'create') {
      expect(api.createPaymentConfig).toHaveBeenCalledExactlyOnceWith({
        ...credentials,
        name: 'Local fixture',
        provider: 1,
        environment: 1,
        appId: '20260001',
        sellerId: '20880001',
        sortOrder: 0,
        isEnabled: 0,
      })
    } else {
      expect(api.publishPaymentConfigVersion).toHaveBeenCalledExactlyOnceWith(7, credentials)
    }
  },
)
it('reimporting a single certificate replaces the previous bundle instead of retaining or appending it', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  vi.mocked(api.publishPaymentConfigVersion).mockResolvedValue({})
  const w = await page()
  await w.get('[data-testid="payment-config-version"]').trigger('click')
  await flushPromises()
  const oldBundle =
    '-----BEGIN CERTIFICATE-----\nb2xk\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\naW50ZXJtZWRpYXRl\n-----END CERTIFICATE-----'
  const replacement = '-----BEGIN CERTIFICATE-----\nbmV3\n-----END CERTIFICATE-----'
  await certificateFile(w, 'alipayCertPEM', 'alipay.crt', oldBundle)
  await certificateFile(w, 'alipayCertPEM', 'alipay.crt', replacement)
  expect(
    w.get<HTMLTextAreaElement>('textarea[data-testid="payment-config-alipayCertPEM"]').element
      .value,
  ).toBe(replacement)
  await w.get('[data-testid="payment-config-save"]').trigger('click')
  await flushPromises()
  expect(api.publishPaymentConfigVersion).toHaveBeenCalledExactlyOnceWith(7, {
    privateKey: null,
    appCertPEM: null,
    alipayCertPEM: replacement,
    rootCertPEM: null,
    notifyUrl: row.notifyUrlTemplate,
    payMethods: [1, 2],
  })
})
it('uses Element Plus textareas and import buttons for certificate entry', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  const w = await page()
  await w.get('[data-testid="payment-config-version"]').trigger('click')
  await flushPromises()
  for (const field of ['appCertPEM', 'alipayCertPEM', 'rootCertPEM']) {
    const input = w.get(`textarea[data-testid="payment-config-${field}"]`)
    await input.setValue('certificate draft')
    expect((input.element as HTMLTextAreaElement).value).toBe('certificate draft')
    const importer = w.get(`[data-testid="payment-config-import-${field}"]`)
    expect(importer.get('button').text()).toBe('导入证书文件')
  }
})
it('places certificate import controls beside each certificate label', async () => {
  permissions(['payment:config:list', 'payment:config:update'])
  const w = await page()
  await w.get('[data-testid="payment-config-version"]').trigger('click')
  await flushPromises()
  for (const field of ['appCertPEM', 'alipayCertPEM', 'rootCertPEM']) {
    const input = w.get(`textarea[data-testid="payment-config-${field}"]`)
    const item = input.element.closest('.el-form-item')
    expect(
      item
        ?.querySelector('.el-form-item__label')
        ?.querySelector(`[data-testid="payment-config-import-${field}"]`),
    ).not.toBeNull()
    expect(item?.querySelector('.payment-config-form__credential-entry--with-import')).toBeNull()
  }
})
it.each(['create', 'version'] as const)(
  '%s places each imported filename immediately after its import button with the full name available',
  async (mode) => {
    permissions(['payment:config:list', 'payment:config:create', 'payment:config:update'])
    const w = await page()
    await w.get(`[data-testid="payment-config-${mode}"]`).trigger('click')
    await flushPromises()
    for (const field of ['appCertPEM', 'alipayCertPEM', 'rootCertPEM']) {
      const filename = `${field}-${'long-certificate-name-'.repeat(12)}.crt`
      await certificateFile(
        w,
        field,
        filename,
        '-----BEGIN CERTIFICATE-----\nY2VydA==\n-----END CERTIFICATE-----',
      )
      const importer = w.get(`[data-testid="payment-config-import-${field}"]`)
      const item = importer.element.closest('.el-form-item')
      const name = item
        ?.querySelector('.el-form-item__label')
        ?.querySelector('.payment-config-form__import-name')
      expect(name).not.toBeNull()
      expect(name?.textContent?.trim()).toBe(filename)
      expect(name?.getAttribute('title')).toBe(filename)
      expect(importer.element.nextElementSibling).toBe(name)
      expect(
        item?.querySelector('.payment-config-form__credential-entry')?.textContent,
      ).not.toContain(filename)
    }
  },
)
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
