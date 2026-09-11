import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as smsApi from '@/api/message/sms'
import { getDictionaryOptions } from '@/api/system/dictionary'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import SmsPage from '@/views/message/sms/index.vue'

vi.mock('@/api/message/sms', () => ({
  getSmsPageInit: vi.fn(),
  getSmsConfig: vi.fn(),
  saveSmsConfig: vi.fn(),
  deleteSmsConfig: vi.fn(),
  sendSmsTest: vi.fn(),
  listSmsTemplates: vi.fn(),
  updateSmsTemplate: vi.fn(),
  updateSmsTemplateStatus: vi.fn(),
  listSmsRules: vi.fn(),
  createSmsRule: vi.fn(),
  updateSmsRule: vi.fn(),
  updateSmsRuleStatus: vi.fn(),
  deleteSmsRule: vi.fn(),
  listSmsLogs: vi.fn(),
  getSmsLogDetail: vi.fn(),
  listSmsRateLimitPolicies: vi.fn(),
  updateSmsRateLimitPolicy: vi.fn(),
}))
vi.mock('@/api/system/dictionary', () => ({ getDictionaryOptions: vi.fn() }))

const timestamp = '2026-09-11T08:00:00Z'
const scenes: smsApi.SmsSceneOption[] = [
  { scene: 'login', name: '登录验证码', parameterKeys: ['code', 'ttl_minutes'] },
  { scene: 'forget', name: '找回密码', parameterKeys: ['code', 'ttl_minutes'] },
  { scene: 'bind_phone', name: '绑定/换绑手机', parameterKeys: ['code', 'ttl_minutes'] },
  { scene: 'change_password', name: '修改密码', parameterKeys: ['code', 'ttl_minutes'] },
]
const config: smsApi.SmsConfig = {
  configured: true,
  smsSdkAppId: '1400000000',
  signName: 'Admin',
  region: 'ap-guangzhou',
  endpoint: '',
  ttlMinutes: 5,
  isEnabled: YesNo.Yes,
  lastTestAt: null,
  lastTestError: '',
}
const template: smsApi.SmsTemplate = {
  id: 1,
  scene: 'login',
  name: '登录验证码',
  tencentTemplateId: '100001',
  parameterKeys: ['code', 'ttl_minutes'],
  exampleVariables: { code: '123456', ttl_minutes: '5' },
  isEnabled: YesNo.Yes,
  createdAt: timestamp,
  updatedAt: timestamp,
}
const rule: smsApi.SmsRule = {
  id: 2,
  scope: 'phone',
  patternHint: '156****8271',
  action: 'deny',
  name: '阻止号码',
  remark: '测试',
  isEnabled: YesNo.Yes,
  createdAt: timestamp,
  updatedAt: timestamp,
}
const log: smsApi.SmsLog = {
  id: 3,
  platformId: 1,
  platform: 'admin',
  userId: 7,
  username: 'alice',
  scene: 'login',
  templateId: 1,
  toPhoneHint: '156****8271',
  status: 'sent',
  requestId: 'request-id',
  serialNo: 'serial-no',
  fee: 1,
  errorCode: '',
  errorSummary: '',
  latencyMs: 30,
  sentAt: timestamp,
  createdAt: timestamp,
  updatedAt: timestamp,
}
const ratePlatform: smsApi.SmsRateLimitPlatform = {
  platformId: 1,
  platformCode: 'admin',
  platformName: 'Admin',
  policies: [
    {
      key: 'business_phone_minute',
      mode: 'business',
      dimension: 'platform_phone',
      limit: 1,
      windowSeconds: 60,
      revision: 1,
      updatedAt: timestamp,
    },
    {
      key: 'business_phone_10m',
      mode: 'business',
      dimension: 'platform_phone',
      limit: 5,
      windowSeconds: 600,
      revision: 1,
      updatedAt: timestamp,
    },
  ],
}
const wrappers: VueWrapper[] = []

describe('SMS management page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(smsApi.getSmsPageInit).mockResolvedValue({ scenes })
    vi.mocked(smsApi.getSmsConfig).mockResolvedValue(config)
    vi.mocked(smsApi.saveSmsConfig).mockResolvedValue(config)
    vi.mocked(smsApi.deleteSmsConfig).mockResolvedValue(undefined)
    vi.mocked(smsApi.sendSmsTest).mockResolvedValue({
      logId: 3,
      status: 'sent',
      requestId: 'request-id',
      serialNo: 'serial-no',
    })
    vi.mocked(smsApi.listSmsTemplates).mockResolvedValue([template])
    vi.mocked(smsApi.updateSmsTemplate).mockResolvedValue(template)
    vi.mocked(smsApi.updateSmsTemplateStatus).mockResolvedValue(undefined)
    vi.mocked(smsApi.listSmsRules).mockResolvedValue([rule])
    vi.mocked(smsApi.createSmsRule).mockResolvedValue(rule)
    vi.mocked(smsApi.updateSmsRule).mockResolvedValue(rule)
    vi.mocked(smsApi.updateSmsRuleStatus).mockResolvedValue(undefined)
    vi.mocked(smsApi.deleteSmsRule).mockResolvedValue(undefined)
    vi.mocked(smsApi.listSmsLogs).mockResolvedValue({
      list: [log],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(smsApi.getSmsLogDetail).mockResolvedValue({
      log,
      toPhone: '+8615671628271',
      verificationCode: '123456',
      verificationExpiresAt: timestamp,
    })
    vi.mocked(smsApi.listSmsRateLimitPolicies).mockResolvedValue({
      platforms: [ratePlatform],
    })
    vi.mocked(smsApi.updateSmsRateLimitPolicy).mockResolvedValue(ratePlatform)
    vi.mocked(getDictionaryOptions).mockResolvedValue({
      'message.sms.region': [{ label: '广州', value: 'ap-guangzhou' }],
    })
  })

  afterEach(() => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
    document.body.innerHTML = ''
  })

  it('requires list permission before rendering tabs or loading control-plane data', async () => {
    const denied = mountPage(['message:sms:view'])
    await flushPromises()
    expect(denied.findAll('[role="tab"]')).toHaveLength(0)
    expect(smsApi.getSmsPageInit).not.toHaveBeenCalled()
    expect(smsApi.getSmsConfig).not.toHaveBeenCalled()
    denied.unmount()

    const allowed = mountPage(['message:sms:list'])
    await flushPromises()
    expect(allowed.findAll('[role="tab"]')).toHaveLength(5)
    expect(smsApi.getSmsPageInit).toHaveBeenCalledOnce()
    expect(smsApi.getSmsConfig).toHaveBeenCalledOnce()
    expect(smsApi.listSmsTemplates).not.toHaveBeenCalled()
  })

  it('loads each tab on demand and exposes a retry for an independent failure', async () => {
    vi.mocked(smsApi.listSmsTemplates)
      .mockRejectedValueOnce(new Error('template unavailable'))
      .mockResolvedValueOnce([template])
    const wrapper = mountPage(['message:sms:list'])
    await flushPromises()

    await selectTab(wrapper, '模板')
    expect(wrapper.text()).toContain('template unavailable')
    await wrapper.get('[data-testid="sms-tab-retry"]').trigger('click')
    await flushPromises()
    expect(smsApi.listSmsTemplates).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('登录验证码')

    await selectTab(wrapper, '收件规则')
    await selectTab(wrapper, '限流策略')
    await selectTab(wrapper, '发送日志')
    expect(smsApi.listSmsRules).toHaveBeenCalledOnce()
    expect(smsApi.listSmsRateLimitPolicies).toHaveBeenCalledOnce()
    expect(smsApi.listSmsLogs).toHaveBeenCalledOnce()
  })

  it('fails closed when region options cannot load and reloads labels with locale', async () => {
    vi.mocked(getDictionaryOptions)
      .mockRejectedValueOnce(new Error('dictionary unavailable'))
      .mockResolvedValueOnce({
        'message.sms.region': [{ label: 'Guangzhou', value: 'ap-guangzhou' }],
      })
    const wrapper = mountPage(['message:sms:list', 'message:sms:config:update'])
    await flushPromises()

    const region = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((item) => item.attributes('data-testid') === 'sms-config-region')
    expect(region?.props('options')).toEqual([])
    expect(region?.props('disabled')).toBe(true)
    expect(wrapper.get('[data-testid="sms-config-save"]').attributes('disabled')).toBeDefined()

    setLocale('en-US')
    await flushPromises()
    expect(getDictionaryOptions).toHaveBeenCalledTimes(2)
    expect(region?.props('options')).toEqual([{ label: 'Guangzhou', value: 'ap-guangzhou' }])
  })

  it('keeps configured secrets blank and submits the remaining exact config', async () => {
    const wrapper = mountPage(['message:sms:list', 'message:sms:config:update'])
    await flushPromises()

    expect(input(wrapper, 'sms-config-secret-id').value).toBe('')
    expect(input(wrapper, 'sms-config-secret-id').placeholder).toContain('留空')
    await wrapper.get('[data-testid="sms-config-save"]').trigger('click')
    await flushPromises()

    expect(smsApi.saveSmsConfig).toHaveBeenCalledWith({
      secretId: '',
      secretKey: '',
      smsSdkAppId: '1400000000',
      signName: 'Admin',
      region: 'ap-guangzhou',
      endpoint: '',
      ttlMinutes: 5,
      isEnabled: YesNo.Yes,
    })
  })

  it('sends an admin test through one of the fixed business scenes', async () => {
    const wrapper = mountPage(['message:sms:list', 'message:sms:test'])
    await flushPromises()

    await wrapper.get('[data-testid="sms-test-phone"]').setValue('15671628271')
    const sceneSelect = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((item) => item.attributes('data-testid') === 'sms-test-scene')
    sceneSelect?.vm.$emit('update:modelValue', 'forget')
    await wrapper.get('[data-testid="sms-test-send"]').trigger('click')
    await flushPromises()

    expect(smsApi.sendSmsTest).toHaveBeenCalledWith({
      toPhone: '15671628271',
      scene: 'forget',
    })
  })

  it('edits a fixed-scene template and controls status with separate permissions', async () => {
    const wrapper = mountPage([
      'message:sms:list',
      'message:sms:template:update',
      'message:sms:template:status',
    ])
    await flushPromises()
    await selectTab(wrapper, '模板')

    await wrapper.get('[data-testid="sms-template-edit"]').trigger('click')
    await flushPromises()
    const sceneInput = document.body.querySelector<HTMLInputElement>(
      '[data-testid="sms-template-scene"]',
    )
    expect(sceneInput?.disabled).toBe(true)
    expect(sceneInput?.value).toBe('login')
    await clickBody('sms-template-save')
    await flushPromises()
    expect(smsApi.updateSmsTemplate).toHaveBeenCalledWith(1, {
      scene: 'login',
      name: '登录验证码',
      tencentTemplateId: '100001',
      parameterKeys: ['code', 'ttl_minutes'],
      exampleVariables: { code: '123456', ttl_minutes: '5' },
    })

    const status = wrapper
      .findAllComponents({ name: 'ElSwitch' })
      .find((item) => item.attributes('data-testid') === 'sms-template-status')
    status?.vm.$emit('change', YesNo.No)
    await flushPromises()
    expect(smsApi.updateSmsTemplateStatus).toHaveBeenCalledWith(1, YesNo.No)
  })

  it('keeps rule patterns secret on edit and gates every rule action independently', async () => {
    const wrapper = mountPage([
      'message:sms:list',
      'message:sms:rule:update',
      'message:sms:rule:status',
    ])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    expect(wrapper.text()).toContain('156****8271')
    expect(wrapper.find('[data-testid="sms-rule-create"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="sms-rule-delete-2"]').exists()).toBe(false)

    await wrapper.get('[data-testid="sms-rule-edit-2"]').trigger('click')
    await flushPromises()
    const pattern = document.body.querySelector<HTMLInputElement>(
      '[data-testid="sms-rule-pattern"]',
    )
    expect(pattern?.value).toBe('')
    expect(pattern?.placeholder).toContain('留空')
    await clickBody('sms-rule-save')
    await flushPromises()
    expect(smsApi.updateSmsRule).toHaveBeenCalledWith(2, {
      scope: 'phone',
      action: 'deny',
      name: '阻止号码',
      remark: '测试',
      isEnabled: YesNo.Yes,
    })

    const status = wrapper
      .findAllComponents({ name: 'ElSwitch' })
      .find((item) => item.attributes('data-testid') === 'sms-rule-status-2')
    status?.vm.$emit('change', YesNo.No)
    await flushPromises()
    expect(smsApi.updateSmsRuleStatus).toHaveBeenCalledWith(2, YesNo.No)
  })

  it('prevents an older log response from overwriting a newer exact-phone result', async () => {
    const older = deferred<Awaited<ReturnType<typeof smsApi.listSmsLogs>>>()
    vi.mocked(smsApi.listSmsLogs)
      .mockReturnValueOnce(older.promise)
      .mockResolvedValueOnce({
        list: [{ ...log, id: 4, toPhoneHint: '138****0000' }],
        total: 1,
        page: 1,
        pageSize: 20,
      })
    const wrapper = mountPage(['message:sms:list'])
    await flushPromises()
    await selectTab(wrapper, '发送日志', false)
    const search = wrapper.getComponent({ name: 'AppSearch' })
    search.vm.$emit('query', {
      platform: '',
      toPhone: '13800000000',
      scene: '',
      status: '',
      timeRange: [],
    })
    await flushPromises()
    expect(wrapper.text()).toContain('138****0000')
    older.resolve({ list: [log], total: 1, page: 1, pageSize: 20 })
    await flushPromises()
    expect(wrapper.text()).toContain('138****0000')
    expect(wrapper.text()).not.toContain('156****8271')
  })

  it('shows sensitive log detail only with the exact detail permission', async () => {
    const denied = mountPage(['message:sms:list'])
    await flushPromises()
    await selectTab(denied, '发送日志')
    expect(denied.find('[data-testid="sms-log-detail-3"]').exists()).toBe(false)
    denied.unmount()

    const allowed = mountPage(['message:sms:list', 'message:sms:detail'])
    await flushPromises()
    await selectTab(allowed, '发送日志')
    await allowed.get('[data-testid="sms-log-detail-3"]').trigger('click')
    await flushPromises()
    expect(smsApi.getSmsLogDetail).toHaveBeenCalledWith(3)
    expect(document.body.textContent).toContain('+8615671628271')
    expect(document.body.textContent).toContain('123456')
  })

  it('updates a rate policy with its platform id and fixed key', async () => {
    const wrapper = mountPage(['message:sms:list', 'message:sms:rate-limit:update'])
    await flushPromises()
    await selectTab(wrapper, '限流策略')

    const limit = wrapper
      .findAllComponents({ name: 'ElInputNumber' })
      .find((item) => item.attributes('data-testid') === 'sms-rate-limit-1-business_phone_minute')
    limit?.vm.$emit('update:modelValue', 2)
    await wrapper.get('[data-testid="sms-rate-save-1-business_phone_minute"]').trigger('click')
    await flushPromises()

    expect(smsApi.updateSmsRateLimitPolicy).toHaveBeenCalledWith(1, 'business_phone_minute', {
      limit: 2,
      windowSeconds: 60,
    })
  })

  it('provides explicit placeholders for every text entry workflow', async () => {
    const wrapper = mountPage(['message:sms:list', 'message:sms:test'])
    await flushPromises()
    for (const testID of [
      'sms-config-secret-id',
      'sms-config-secret-key',
      'sms-config-sdk-app-id',
      'sms-config-sign-name',
      'sms-config-endpoint',
      'sms-test-phone',
    ]) {
      expect(input(wrapper, testID).placeholder, testID).not.toBe('')
    }
  })
})

function mountPage(permissionCodes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  const wrapper = mount(SmsPage, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
  wrappers.push(wrapper)
  return wrapper
}

async function selectTab(wrapper: VueWrapper, label: string, flush = true): Promise<void> {
  const tab = wrapper.findAll('[role="tab"]').find((item) => item.text() === label)
  if (tab === undefined) throw new Error(`missing tab: ${label}`)
  await tab.trigger('click')
  if (flush) await flushPromises()
}

function input(wrapper: VueWrapper, testID: string): HTMLInputElement {
  return wrapper.get(`[data-testid="${testID}"]`).element as HTMLInputElement
}

async function clickBody(testID: string): Promise<void> {
  const button = document.body.querySelector<HTMLButtonElement>(`[data-testid="${testID}"]`)
  if (button === null) throw new Error(`missing body button: ${testID}`)
  button.click()
  await Promise.resolve()
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}
