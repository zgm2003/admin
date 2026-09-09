import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as mailApi from '@/api/message/mail'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import MailPage from '@/views/message/mail/index.vue'

vi.mock('@/api/message/mail', () => ({
  getMailConfig: vi.fn(),
  saveMailConfig: vi.fn(),
  deleteMailConfig: vi.fn(),
  sendMailTest: vi.fn(),
  listMailTemplates: vi.fn(),
  updateMailTemplate: vi.fn(),
  updateMailTemplateStatus: vi.fn(),
  listMailLogs: vi.fn(),
  getMailLogDetail: vi.fn(),
  listMailRules: vi.fn(),
  createMailRule: vi.fn(),
  updateMailRule: vi.fn(),
  updateMailRuleStatus: vi.fn(),
  deleteMailRule: vi.fn(),
  listMailRateLimitPolicies: vi.fn(),
  updateMailRateLimitPolicy: vi.fn(),
}))

describe('mail service page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(mailApi.getMailConfig).mockResolvedValue({
      configured: true,
      region: 'ap-guangzhou',
      endpoint: '',
      fromEmail: 'sender@example.com',
      fromName: 'Admin',
      replyTo: '',
      ttlMinutes: 10,
      isEnabled: YesNo.Yes,
      lastTestAt: null,
      lastTestError: '',
    })
    vi.mocked(mailApi.listMailTemplates).mockResolvedValue([
      {
        id: 1,
        scene: 'login',
        name: '登录验证码',
        subject: '登录验证码',
        tencentTemplateId: 47941,
        variables: { code: '123456', ttl_minutes: '10' },
        exampleVariables: { code: '123456', ttl_minutes: '10' },
        isEnabled: YesNo.Yes,
        createdAt: '2026-09-01T00:00:00Z',
        updatedAt: '2026-09-01T00:00:00Z',
      },
    ])
    vi.mocked(mailApi.listMailLogs).mockResolvedValue({ list: [], total: 0, page: 1, pageSize: 20 })
    vi.mocked(mailApi.listMailRules).mockResolvedValue([])
    vi.mocked(mailApi.listMailRateLimitPolicies).mockResolvedValue({ platforms: [] })
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('hides the log tab without detail permission and never restores secrets from config', async () => {
    const wrapper = mountPage(['message:mail:list'])
    await flushPromises()
    expect(wrapper.text()).not.toContain('发送日志')
    const passwords = wrapper.findAll('input[type="password"]')
    expect(passwords).toHaveLength(2)
    expect(passwords.every((input) => (input.element as HTMLInputElement).value === '')).toBe(true)
  })

  it('uses one compact management surface without a nested card around mail content', async () => {
    const wrapper = mountPage(['message:mail:list'])
    await flushPromises()

    const page = wrapper.find('.mail-page')
    expect(page.classes()).toContain('management-page')
    expect(page.findAll('.el-card')).toHaveLength(0)
    expect(page.findAll('.mail-panel')).toHaveLength(0)
    expect(page.find('.mail-tabs').exists()).toBe(true)
  })

  it('hides all mail data tabs without list permission', async () => {
    const wrapper = mountPage(['message:mail:view'])
    await flushPromises()
    expect(wrapper.findAll('[role="tab"]')).toHaveLength(0)
    expect(mailApi.getMailConfig).not.toHaveBeenCalled()
  })

  it('uses Tencent SES region choices and field names in the configuration form', async () => {
    const wrapper = mountPage(['message:mail:list'])
    await flushPromises()

    const regionSelect = wrapper.findComponent({ name: 'ElSelectV2' })
    expect(regionSelect.exists()).toBe(true)
    expect(regionSelect.props('modelValue')).toBe('ap-guangzhou')
    expect(regionSelect.props('options')).toEqual([
      { value: 'ap-guangzhou', label: '广州（ap-guangzhou）' },
      { value: 'ap-hongkong', label: '中国香港（ap-hongkong）' },
    ])

    const configForm = wrapper.findComponent({ name: 'ElForm' })
    expect(configForm.props('labelWidth')).toBe('120px')
    expect(wrapper.text()).toContain('地域')
    expect(wrapper.text()).toContain('发信地址')
    expect(wrapper.text()).toContain('发件人别名')
  })

  it('disables test sending while the mail service is inactive', async () => {
    vi.mocked(mailApi.getMailConfig).mockResolvedValueOnce({
      configured: true,
      region: 'ap-guangzhou',
      endpoint: '',
      fromEmail: 'sender@example.com',
      fromName: 'Admin',
      replyTo: '',
      ttlMinutes: 10,
      isEnabled: YesNo.No,
      lastTestAt: null,
      lastTestError: '',
    })
    const wrapper = mountPage(['message:mail:list', 'message:mail:test'])
    await flushPromises()

    const testButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('发送测试'))
    expect(testButton).toBeDefined()
    expect(testButton?.attributes('disabled')).toBeDefined()
  })

  it('reloads the latest test error after a rejected management test', async () => {
    vi.mocked(mailApi.getMailConfig)
      .mockResolvedValueOnce({
        configured: true,
        region: 'ap-guangzhou',
        endpoint: '',
        fromEmail: 'sender@example.com',
        fromName: 'Admin',
        replyTo: '',
        ttlMinutes: 10,
        isEnabled: YesNo.Yes,
        lastTestAt: null,
        lastTestError: '',
      })
      .mockResolvedValueOnce({
        configured: true,
        region: 'ap-guangzhou',
        endpoint: '',
        fromEmail: 'sender@example.com',
        fromName: 'Admin',
        replyTo: '',
        ttlMinutes: 10,
        isEnabled: YesNo.Yes,
        lastTestAt: '2026-09-08T10:00:00Z',
        lastTestError: 'mail recipient denied',
      })
    vi.mocked(mailApi.sendMailTest).mockRejectedValueOnce(new Error('mail recipient denied'))
    const wrapper = mountPage(['message:mail:list', 'message:mail:test'])
    await flushPromises()

    await wrapper.get('[data-testid="mail-config-test"]').trigger('click')
    await flushPromises()

    expect(mailApi.getMailConfig).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('mail recipient denied')
  })

  it('renders controls only for granted action permissions', async () => {
    const wrapper = mountPage([
      'message:mail:list',
      'message:mail:detail',
      'message:mail:template:update',
      'message:mail:rule:create',
    ])
    await flushPromises()
    expect(wrapper.find('[data-testid="mail-config-save"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('发送日志')

    await selectTab(wrapper, '邮件模板')
    expect(wrapper.find('[data-testid="mail-template-edit"]').exists()).toBe(true)
    await selectTab(wrapper, '发送日志')
    expect(wrapper.find('[data-testid="mail-log-batch-delete"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('批量删除')
    await selectTab(wrapper, '收件规则')
    expect(wrapper.find('[data-testid="mail-rule-create"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('默认允许；精确邮箱优先于域名；拒绝优先于允许。')
  })

  it('uses virtualized selects for recipient rule choices', async () => {
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:create'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-create"]').trigger('click')
    await flushPromises()

    const selects = wrapper.findAllComponents({ name: 'ElSelectV2' })
    const scope = selects.find((select) => select.attributes('data-testid') === 'mail-rule-scope')
    const action = selects.find((select) => select.attributes('data-testid') === 'mail-rule-action')
    expect(scope?.props('options')).toEqual([
      { value: 'email', label: '邮箱' },
      { value: 'domain', label: '域名' },
    ])
    expect(action?.props('options')).toEqual([
      { value: 'allow', label: '允许' },
      { value: 'deny', label: '拒绝' },
    ])
  })

  it('passes the recipient rule id when toggling its status', async () => {
    vi.mocked(mailApi.listMailRules).mockResolvedValue([
      {
        id: 7,
        scope: 'domain',
        pattern: 'example.com',
        action: 'deny',
        name: 'Blocked domain',
        remark: '',
        isEnabled: YesNo.Yes,
        createdAt: '2026-09-01T00:00:00Z',
        updatedAt: '2026-09-01T00:00:00Z',
      },
    ])
    vi.mocked(mailApi.updateMailRuleStatus).mockResolvedValueOnce({ id: 7, isEnabled: YesNo.No })
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:status'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')

    const toggle = wrapper.findAll('.el-switch')[1]
    expect(toggle.exists()).toBe(true)
    await toggle.trigger('click')
    await flushPromises()

    expect(mailApi.updateMailRuleStatus).toHaveBeenCalledWith(7, YesNo.No)
  })

  it('does not pass a static success result state to mail tables', async () => {
    const wrapper = mountPage(['message:mail:list'])
    await flushPromises()
    await selectTab(wrapper, '邮件模板')

    expect(wrapper.findComponent({ name: 'AppTable' }).props('resultState')).toBe('idle')
  })
  it('renders the provider send time and verification expiration in the active locale', async () => {
    vi.mocked(mailApi.listMailLogs).mockResolvedValue({
      list: [
        {
          id: 9,
          platformId: 1,
          platform: 'admin',
          userId: 169,
          username: 'tester',
          scene: 'login',
          templateId: 1,
          toEmail: '2093146753@qq.com',
          subject: '登录验证码',
          status: 'sent',
          requestId: '',
          messageId: '',
          errorCode: '',
          errorSummary: '',
          latencyMs: 433,
          sentAt: '2026-09-03T14:33:44.2485Z',
          createdAt: '2026-09-02T14:33:44.2485Z',
          updatedAt: '2026-09-03T14:33:44.2485Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(mailApi.getMailLogDetail).mockResolvedValue({
      log: {
        id: 9,
        platformId: 1,
        platform: 'admin',
        userId: 169,
        username: 'tester',
        scene: 'login',
        templateId: 1,
        toEmail: '2093146753@qq.com',
        subject: '登录验证码',
        status: 'sent',
        requestId: '',
        messageId: '',
        errorCode: '',
        errorSummary: '',
        latencyMs: 433,
        sentAt: '2026-09-03T14:33:44.2485Z',
        createdAt: '2026-09-02T14:33:44.2485Z',
        updatedAt: '2026-09-03T14:33:44.2485Z',
      },
      verificationCode: '123456',
      verificationExpiresAt: '2026-09-04T14:33:44.2485Z',
    })
    const wrapper = mountPage(['message:mail:list', 'message:mail:detail'])
    await flushPromises()
    await selectTab(wrapper, '发送日志')

    expect(wrapper.text()).toContain('发送时间')
    expect(wrapper.text()).toContain('2026年9月3日')
    expect(wrapper.text()).not.toContain('2026年9月2日')
    await wrapper
      .findAll('button')
      .find((button) => button.text() === '详情')!
      .trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('2026年9月4日')
    expect(document.body.textContent).not.toContain('2026-09-04T14:33:44')
  })

  it('renders a placeholder when a delivery has no provider send time', async () => {
    vi.mocked(mailApi.listMailLogs).mockResolvedValue({
      list: [
        {
          id: 10,
          platformId: 2,
          platform: 'canvas',
          userId: null,
          username: '',
          scene: 'login',
          templateId: 1,
          toEmail: 'pending@example.com',
          subject: '登录验证码',
          status: 'pending',
          requestId: '',
          messageId: '',
          errorCode: '',
          errorSummary: '',
          latencyMs: 0,
          sentAt: null,
          createdAt: '2026-09-02T14:33:44.2485Z',
          updatedAt: '2026-09-02T14:33:44.2485Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage(['message:mail:list', 'message:mail:detail'])
    await flushPromises()
    await selectTab(wrapper, '发送日志')

    expect(wrapper.text()).toContain('发送时间操作')
    expect(wrapper.text()).toContain('-')
    expect(wrapper.text()).toContain('平台')
    expect(wrapper.text()).toContain('canvas')
  })

  it('sends delivery log filters through the log query', async () => {
    const wrapper = mountPage(['message:mail:list', 'message:mail:detail'])
    await flushPromises()
    await selectTab(wrapper, '发送日志')
    expect(mailApi.listMailLogs).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 })

    await wrapper.get('[data-testid="mail-log-platform"]').setValue(' canvas')
    await wrapper.get('[data-testid="mail-log-email"]').setValue('user@example.com')
    wrapper.findComponent({ name: 'AppSearch' }).vm.$emit('query', {
      platform: 'canvas',
      toEmail: 'user@example.com',
      scene: 'login',
      status: '',
      timeRange: [],
    })
    await flushPromises()
    expect(mailApi.listMailLogs).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      platform: 'canvas',
      toEmail: 'user@example.com',
      scene: 'login',
    })

    wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
    await flushPromises()
    expect(mailApi.listMailLogs).toHaveBeenLastCalledWith({
      page: 2,
      pageSize: 20,
      platform: 'canvas',
      toEmail: 'user@example.com',
      scene: 'login',
    })
  })

  it('loads delivery logs even when the optional template catalog fails', async () => {
    vi.mocked(mailApi.listMailTemplates).mockRejectedValueOnce(new Error('template unavailable'))
    vi.mocked(mailApi.listMailLogs).mockResolvedValueOnce({
      list: [mailLogRow(21, 'fallback@example.com', 'custom_scene')],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage(['message:mail:list', 'message:mail:detail'])
    await flushPromises()

    await selectTab(wrapper, '发送日志')

    expect(mailApi.listMailLogs).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('fallback@example.com')
    expect(wrapper.text()).toContain('custom_scene')
    expect(wrapper.find('.mail-error').exists()).toBe(false)
  })

  it('attempts an empty template catalog only once while paging logs', async () => {
    vi.mocked(mailApi.listMailTemplates).mockResolvedValueOnce([])
    const wrapper = mountPage(['message:mail:list', 'message:mail:detail'])
    await flushPromises()
    await selectTab(wrapper, '发送日志')

    wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('current-change', 2)
    await flushPromises()

    expect(mailApi.listMailTemplates).toHaveBeenCalledOnce()
    expect(mailApi.listMailLogs).toHaveBeenCalledTimes(2)
  })

  it('does not let an older log response overwrite a newer filter result', async () => {
    const older = deferred<Awaited<ReturnType<typeof mailApi.listMailLogs>>>()
    vi.mocked(mailApi.listMailLogs)
      .mockReturnValueOnce(older.promise)
      .mockResolvedValueOnce({
        list: [mailLogRow(23, 'new@example.com', 'login')],
        total: 1,
        page: 1,
        pageSize: 20,
      })
    const wrapper = mountPage(['message:mail:list', 'message:mail:detail'])
    await flushPromises()
    await selectTab(wrapper, '发送日志')

    wrapper.findComponent({ name: 'AppSearch' }).vm.$emit('query', {
      platform: '',
      toEmail: 'new@example.com',
      scene: '',
      status: '',
      timeRange: [],
    })
    await flushPromises()
    expect(wrapper.text()).toContain('new@example.com')

    older.resolve({
      list: [mailLogRow(22, 'old@example.com', 'login')],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('new@example.com')
    expect(wrapper.text()).not.toContain('old@example.com')
  })

  it('shows the rate limit tab only with list permission and does not fetch it eagerly', async () => {
    const wrapper = mountPage(['message:mail:list'])
    await flushPromises()
    expect(mailApi.listMailRateLimitPolicies).not.toHaveBeenCalled()

    await selectTab(wrapper, '限流策略')
    await flushPromises()
    expect(mailApi.listMailRateLimitPolicies).toHaveBeenCalledOnce()
  })

  it('never fetches rate limit policies without list permission', async () => {
    mountPage(['message:mail:view'])
    await flushPromises()
    expect(mailApi.listMailRateLimitPolicies).not.toHaveBeenCalled()
  })
})

function mountPage(permissionCodes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  return mount(MailPage, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
}

async function selectTab(wrapper: VueWrapper, label: string) {
  const tab = wrapper.findAll('[role="tab"]').find((item) => item.text() === label)
  if (!tab) throw new Error(`tab not found: ${label}`)
  await tab.trigger('click')
  await flushPromises()
}

function mailLogRow(id: number, toEmail: string, scene: string): mailApi.MailLog {
  return {
    id,
    platformId: 1,
    platform: 'admin',
    userId: null,
    username: '',
    scene,
    templateId: 1,
    toEmail,
    subject: 'subject',
    status: 'sent',
    requestId: '',
    messageId: '',
    errorCode: '',
    errorSummary: '',
    latencyMs: 1,
    sentAt: '2026-09-09T00:00:00Z',
    createdAt: '2026-09-09T00:00:00Z',
    updatedAt: '2026-09-09T00:00:00Z',
  }
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
