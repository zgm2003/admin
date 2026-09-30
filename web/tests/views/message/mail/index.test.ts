import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as mailApi from '@/api/message/mail'
import { getDictionaryOptions } from '@/api/system/dictionary'
import { requestObjectURL } from '@/api/storage/upload'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import MailPage from '@/views/message/mail/index.vue'

const wrappers: VueWrapper[] = []

vi.mock('@/api/message/mail', () => ({
  MailStatus: { Pending: 1, Sent: 2, Failed: 3 },
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
  mailRuleCSVMaxBytes: 1024 * 1024,
  getMailRuleImportTemplate: vi.fn(),
  previewMailRuleImport: vi.fn(),
  importMailRules: vi.fn(),
  exportMailRules: vi.fn(),
  listMailRateLimitPolicies: vi.fn(),
  updateMailRateLimitPolicy: vi.fn(),
}))
vi.mock('@/api/system/dictionary', () => ({ getDictionaryOptions: vi.fn() }))
vi.mock('@/api/storage/upload', () => ({ requestObjectURL: vi.fn() }))

describe('mail service page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(mailApi.getMailRuleImportTemplate).mockReset().mockResolvedValue({ objectKey: '' })
    vi.mocked(requestObjectURL)
      .mockReset()
      .mockResolvedValue({ url: 'https://example.com/template.csv', expiresAt: null })
    vi.mocked(mailApi.previewMailRuleImport).mockReset()
    vi.mocked(mailApi.importMailRules).mockReset().mockResolvedValue({ imported: 1 })
    setLocale('zh-CN')
    vi.mocked(getDictionaryOptions)
      .mockReset()
      .mockResolvedValue({
        'message.mail.region': [
          { value: 'ap-guangzhou', label: '广州（ap-guangzhou）' },
          { value: 'ap-hongkong', label: '中国香港（ap-hongkong）' },
        ],
      })
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
        content: '<!DOCTYPE html><html><head></head><body>{{code}} {{ttl_minutes}}</body></html>',
        variableKeys: ['code', 'ttl_minutes'],
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
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
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

  it.each(['import', 'export'])(
    'exposes the independent CSV %s action without list access',
    async (action) => {
      const wrapper = mountPage(['message:mail:view', `message:mail:rule:${action}`])
      await flushPromises()
      expect(wrapper.findAll('[role="tab"]')).toHaveLength(1)
      expect(wrapper.get('[role="tab"]').attributes('aria-selected')).toBe('true')
      expect(wrapper.find(`[data-testid="mail-rule-${action}"]`).exists()).toBe(true)
      expect(wrapper.text()).toContain('未授予收件规则读取权限')
      expect(wrapper.findAllComponents({ name: 'AppTable' })).toHaveLength(0)
      expect(mailApi.getMailConfig).not.toHaveBeenCalled()
      expect(mailApi.listMailRules).not.toHaveBeenCalled()
    },
  )

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
    expect(getDictionaryOptions).toHaveBeenCalledWith(['message.mail.region'])

    const configForm = wrapper.findComponent({ name: 'ElForm' })
    expect(configForm.props('labelWidth')).toBe('120px')
    expect(wrapper.text()).toContain('地域')
    expect(wrapper.text()).toContain('发信地址')
    expect(wrapper.text()).toContain('发件人别名')
  })

  it('does not substitute hardcoded mail regions when dictionary loading fails', async () => {
    vi.mocked(getDictionaryOptions).mockRejectedValueOnce(new Error('dictionary unavailable'))
    const wrapper = mountPage(['message:mail:list'])
    await flushPromises()

    const regionSelect = wrapper.findComponent({ name: 'ElSelectV2' })
    expect(regionSelect.props('options')).toEqual([])
    expect(regionSelect.props('disabled')).toBe(true)
    expect(wrapper.text()).toContain('邮件地域选项加载失败')
  })

  it('reloads mail region labels when the active language changes', async () => {
    vi.mocked(getDictionaryOptions)
      .mockReset()
      .mockResolvedValueOnce({
        'message.mail.region': [{ value: 'ap-guangzhou', label: '广州（ap-guangzhou）' }],
      })
      .mockResolvedValueOnce({
        'message.mail.region': [{ value: 'ap-guangzhou', label: 'Guangzhou (ap-guangzhou)' }],
      })
    const wrapper = mountPage(['message:mail:list'])
    await flushPromises()
    const regionSelect = wrapper.findComponent({ name: 'ElSelectV2' })
    expect(regionSelect.props('options')).toEqual([
      { value: 'ap-guangzhou', label: '广州（ap-guangzhou）' },
    ])

    setLocale('en-US')
    await flushPromises()

    expect(getDictionaryOptions).toHaveBeenCalledTimes(2)
    expect(regionSelect.props('options')).toEqual([
      { value: 'ap-guangzhou', label: 'Guangzhou (ap-guangzhou)' },
    ])
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
          status: mailApi.MailStatus.Sent,
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
        status: mailApi.MailStatus.Sent,
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
          status: mailApi.MailStatus.Pending,
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

  it('does not let an older config response overwrite a newer tab reload', async () => {
    const older = deferred<mailApi.MailConfig>()
    vi.mocked(mailApi.getMailConfig).mockReturnValueOnce(older.promise).mockResolvedValueOnce({
      configured: true,
      region: 'ap-guangzhou',
      endpoint: '',
      fromEmail: 'new@example.com',
      fromName: 'New sender',
      replyTo: '',
      ttlMinutes: 10,
      isEnabled: YesNo.Yes,
      lastTestAt: null,
      lastTestError: '',
    })
    const wrapper = mountPage(['message:mail:list'])
    await vi.waitFor(() => expect(mailApi.getMailConfig).toHaveBeenCalledOnce())

    await selectTab(wrapper, '邮件模板')
    await selectTab(wrapper, '邮件配置')
    expect(mailConfigInputValues(wrapper)).toContain('new@example.com')

    older.resolve({
      configured: true,
      region: 'ap-guangzhou',
      endpoint: '',
      fromEmail: 'old@example.com',
      fromName: 'Old sender',
      replyTo: '',
      ttlMinutes: 10,
      isEnabled: YesNo.Yes,
      lastTestAt: null,
      lastTestError: '',
    })
    await flushPromises()

    expect(mailConfigInputValues(wrapper)).toContain('new@example.com')
    expect(mailConfigInputValues(wrapper)).not.toContain('old@example.com')
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

  it('gates CSV import and export with their own independent actions', async () => {
    const readonly = mountPage(['message:mail:list', 'message:mail:rule:create'])
    await flushPromises()
    await selectTab(readonly, '收件规则')
    expect(readonly.find('[data-testid="mail-rule-import"]').exists()).toBe(false)
    expect(readonly.find('[data-testid="mail-rule-export"]').exists()).toBe(false)
    readonly.unmount()
    const editable = mountPage([
      'message:mail:list',
      'message:mail:rule:import',
      'message:mail:rule:export',
    ])
    await flushPromises()
    await selectTab(editable, '收件规则')
    expect(editable.find('[data-testid="mail-rule-import"]').exists()).toBe(true)
    expect(editable.find('[data-testid="mail-rule-export"]').exists()).toBe(true)
    expect(editable.find('[data-testid="mail-rule-create"]').exists()).toBe(false)
  })

  it('exports fresh server data with a native Blob download and revokes the object URL', async () => {
    vi.mocked(mailApi.exportMailRules).mockResolvedValue({
      fileName: 'mail-recipient-rule.csv',
      content: '\ufeff类型,邮箱/域名,动作,名称,备注,启用状态\n',
    })
    const createURL = vi.fn(() => 'blob:mail-rules')
    const revokeURL = vi.fn()
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL = createURL
        static revokeObjectURL = revokeURL
      },
    )
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:export'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-export"]').trigger('click')
    await flushPromises()
    expect(createURL).toHaveBeenCalledWith(expect.any(Blob))
    expect(click).toHaveBeenCalledOnce()
    expect(revokeURL).toHaveBeenCalledWith('blob:mail-rules')
    expect(document.querySelector('a[download="mail-recipient-rule.csv"]')).toBeNull()
    click.mockRestore()
    vi.unstubAllGlobals()
  })

  it('previews an uploaded CSV before importing the unchanged content', async () => {
    const content = '类型,邮箱/域名,动作,名称,备注,启用状态\nemail,a@example.com,deny,规则,,1\n'
    vi.mocked(mailApi.getMailRuleImportTemplate).mockResolvedValue({
      objectKey:
        'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv',
    })
    vi.mocked(mailApi.previewMailRuleImport).mockResolvedValue({
      rows: [{ line: 2, values: ['email', 'a@example.com', 'deny', '规则', '', '1'], errors: [] }],
      errors: [],
    })
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:import'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
    await flushPromises()
    expect(
      document.querySelector('[data-testid="mail-rule-template-download"]')?.getAttribute('href'),
    ).toBe('https://example.com/template.csv')
    expect(requestObjectURL).toHaveBeenCalledWith(
      'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv',
    )
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
    await uploadRuleCSV(new File([content], 'rules.csv', { type: 'text/csv' }))
    await vi.waitFor(() => expect(mailApi.previewMailRuleImport).toHaveBeenCalledWith(content))
    await flushPromises()
    expect(document.body.textContent).toContain('a@example.com')
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(false)
    bodyButton('mail-rule-import-confirm').click()
    await flushPromises()
    expect(mailApi.importMailRules).toHaveBeenCalledWith(content)
    expect(mailApi.listMailRules).toHaveBeenCalledTimes(2)
  })

  it('shows row errors and does not allow a partial import', async () => {
    vi.mocked(mailApi.previewMailRuleImport).mockResolvedValue({
      rows: [
        {
          line: 2,
          values: ['domain', '@qq.com', 'deny', '错误', '', '1'],
          errors: ['invalid_pattern'],
        },
      ],
      errors: [],
    })
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:import'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('模板未配置')
    await uploadRuleCSV(new File(['bad'], 'rules.csv', { type: 'text/csv' }))
    await vi.waitFor(() => expect(mailApi.previewMailRuleImport).toHaveBeenCalledOnce())
    await flushPromises()
    expect(document.body.textContent).toContain('邮箱或域名格式无效')
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
    expect(mailApi.importMailRules).not.toHaveBeenCalled()
    expect(requestObjectURL).not.toHaveBeenCalled()
  })

  it('reports template object resolution failure without guessing a download URL', async () => {
    vi.mocked(mailApi.getMailRuleImportTemplate).mockResolvedValue({
      objectKey:
        'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv',
    })
    vi.mocked(requestObjectURL).mockRejectedValue(new Error('storage unavailable'))
    const wrapper = mountPage(['message:mail:view', 'message:mail:rule:import'])
    await flushPromises()
    await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
    await flushPromises()
    expect(requestObjectURL).toHaveBeenCalledOnce()
    expect(document.querySelector('[data-testid="mail-rule-template-download"]')).toBeNull()
    expect(document.body.textContent).toContain('模板下载地址加载失败')
  })

  it('discards an old template object URL after import permission is revoked', async () => {
    vi.mocked(mailApi.getMailRuleImportTemplate).mockResolvedValue({
      objectKey:
        'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.csv',
    })
    const pending = deferred<{ url: string; expiresAt: null }>()
    vi.mocked(requestObjectURL).mockReturnValue(pending.promise)
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:import'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
    await flushPromises()
    expect(requestObjectURL).toHaveBeenCalledOnce()
    usePermissionStore().applySnapshot({
      roleCodes: [],
      menuTree: [],
      permissionCodes: ['message:mail:list'],
    })
    pending.resolve({ url: 'https://example.com/stale.csv', expiresAt: null })
    await flushPromises()
    expect(document.querySelector('[data-testid="mail-rule-template-download"]')).toBeNull()
  })

  it('discards a stale preview after a new file is selected', async () => {
    const older = deferred<mailApi.MailRuleCSVPreview>()
    vi.mocked(mailApi.previewMailRuleImport)
      .mockReturnValueOnce(older.promise)
      .mockResolvedValueOnce({
        rows: [
          { line: 2, values: ['email', 'new@example.com', 'deny', 'new', '', '1'], errors: [] },
        ],
        errors: [],
      })
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:import'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
    await flushPromises()
    await uploadRuleCSV(new File(['old'], 'old.csv', { type: 'text/csv' }))
    await vi.waitFor(() => expect(mailApi.previewMailRuleImport).toHaveBeenCalledOnce())
    await uploadRuleCSV(new File(['new'], 'new.csv', { type: 'text/csv' }))
    await vi.waitFor(() => expect(mailApi.previewMailRuleImport).toHaveBeenCalledTimes(2))
    await flushPromises()
    older.resolve({
      rows: [{ line: 2, values: ['email', 'old@example.com', 'deny', 'old', '', '1'], errors: [] }],
      errors: [],
    })
    await flushPromises()
    expect(document.body.textContent).toContain('new@example.com')
    expect(document.body.textContent).not.toContain('old@example.com')
  })

  it('clears pending file reading when a replacement has an invalid extension', async () => {
    const reader = vi
      .spyOn(FileReader.prototype, 'readAsArrayBuffer')
      .mockImplementation(() => undefined)
    try {
      const wrapper = mountPage(['message:mail:list', 'message:mail:rule:import'])
      await flushPromises()
      await selectTab(wrapper, '收件规则')
      await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
      await flushPromises()
      await uploadRuleCSV(new File(['pending'], 'pending.csv', { type: 'text/csv' }))
      expect(wrapper.findAllComponents({ name: 'AppTable' })).toHaveLength(2)
      await uploadRuleCSV(new File(['not csv'], 'replacement.txt', { type: 'text/plain' }))
      expect(document.body.textContent).toContain('请选择 .csv 文件')
      expect(wrapper.findAllComponents({ name: 'AppTable' })).toHaveLength(1)
      expect(mailApi.previewMailRuleImport).not.toHaveBeenCalled()
    } finally {
      reader.mockRestore()
    }
  })
})

function bodyButton(testId: string): HTMLButtonElement {
  const button = document.querySelector(`[data-testid="${testId}"]`)
  if (!(button instanceof HTMLButtonElement)) throw new Error(`button missing: ${testId}`)
  return button
}

async function uploadRuleCSV(file: File): Promise<void> {
  const input = document.querySelector('[data-testid="mail-rule-import-file"]')
  if (!(input instanceof HTMLInputElement)) throw new Error('CSV input missing')
  Object.defineProperty(input, 'files', { configurable: true, value: [file] })
  input.dispatchEvent(new Event('change', { bubbles: true }))
  await flushPromises()
}

function mountPage(permissionCodes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  const wrapper = mount(MailPage, {
    attachTo: document.body,
    global: { plugins: [pinia, appI18n, ElementPlus] },
  })
  wrappers.push(wrapper)
  return wrapper
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
    status: mailApi.MailStatus.Sent,
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

function mailConfigInputValues(wrapper: VueWrapper): string[] {
  return wrapper
    .get('.mail-form')
    .findAll('input')
    .map((item) => (item.element as HTMLInputElement).value)
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
