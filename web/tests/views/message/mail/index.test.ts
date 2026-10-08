import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as mailApi from '@/api/message/mail'
import { getDictionaryOptions } from '@/api/system/dictionary'
import { requestObjectURL } from '@/api/storage/upload'
import { YesNo } from '@/enums/yesNo'
import { MailRuleAction, MailRuleScope } from '@/enums/mailRecipientRule'
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
  mailRuleXlsxMaxBytes: 2 * 1024 * 1024,
  mailRuleXlsxMime: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  getMailRuleImportTemplate: vi.fn(),
  previewMailRuleXlsx: vi.fn(),
  importMailRuleXlsx: vi.fn(),
  exportMailRuleXlsx: vi.fn(),
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
      .mockResolvedValue({ url: 'https://example.com/template.xlsx', expiresAt: null })
    vi.mocked(mailApi.previewMailRuleXlsx).mockReset()
    vi.mocked(mailApi.importMailRuleXlsx).mockReset().mockResolvedValue({ imported: 1 })
    vi.mocked(mailApi.exportMailRuleXlsx).mockReset()
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
    'exposes the independent XLSX %s action without list access',
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
      { value: 0, label: '邮箱' },
      { value: 1, label: '域名' },
    ])
    expect(action?.props('options')).toEqual([
      { value: 1, label: '允许' },
      { value: 0, label: '拒绝' },
    ])
    expect(scope?.props('modelValue')).toBe(0)
    expect(action?.props('modelValue')).toBe(0)
    expect(scope?.props('clearable')).toBe(false)
    expect(action?.props('clearable')).toBe(false)

    setLocale('en-US')
    await flushPromises()
    expect(scope?.props('options')).toEqual([
      { value: 0, label: 'Email' },
      { value: 1, label: 'Domain' },
    ])
    expect(action?.props('options')).toEqual([
      { value: 1, label: 'Allowlist' },
      { value: 0, label: 'Denylist' },
    ])
    expect(getDictionaryOptions).toHaveBeenCalledTimes(2)
    expect(
      vi
        .mocked(getDictionaryOptions)
        .mock.calls.every(([codes]) => codes.length === 1 && codes[0] === 'message.mail.region'),
    ).toBe(true)
  })

  it('creates an email deny rule with both zero enum values', async () => {
    vi.mocked(mailApi.createMailRule).mockResolvedValueOnce({ id: 7 })
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:create'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-create"]').trigger('click')
    await flushPromises()
    const dialog = wrapper
      .findAllComponents({ name: 'ElDialog' })
      .find((item) => item.props('modelValue') === true)
    expect(dialog).toBeDefined()
    if (!dialog) throw new Error('recipient rule dialog missing')
    await dialog.get('[data-testid="mail-rule-pattern"]').setValue('a@example.com')
    expect(dialog.get('[data-testid="mail-rule-pattern"]').attributes('placeholder')).toContain(
      '完整邮箱',
    )
    await dialog.get('.el-dialog__footer .el-button--primary').trigger('click')
    await flushPromises()
    expect(mailApi.createMailRule).toHaveBeenCalledWith({
      scope: 0,
      pattern: 'a@example.com',
      action: 0,
      name: '',
      remark: '',
      isEnabled: 1,
    })
    expect(mailApi.listMailRules).toHaveBeenCalledTimes(2)
  })

  it('renders both numeric enum choices and edits domain allow into email deny', async () => {
    const timestamp = '2026-09-01T00:00:00Z'
    vi.mocked(mailApi.listMailRules).mockResolvedValue([
      {
        id: 7,
        scope: 1,
        pattern: 'example.com',
        action: 1,
        name: 'Domain allow',
        remark: '',
        isEnabled: 1,
        createdAt: timestamp,
        updatedAt: timestamp,
      },
      {
        id: 8,
        scope: 0,
        pattern: 'a@example.com',
        action: 0,
        name: 'Email deny',
        remark: '',
        isEnabled: 1,
        createdAt: timestamp,
        updatedAt: timestamp,
      },
    ])
    vi.mocked(mailApi.updateMailRule).mockResolvedValueOnce({})
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:update'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    const rows = wrapper.findAll('.el-table__body-wrapper tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('域名')
    expect(rows[0].text()).toContain('允许')
    expect(rows[0].get('.el-tag').classes()).toContain('el-tag--success')
    expect(rows[1].text()).toContain('邮箱')
    expect(rows[1].text()).toContain('拒绝')
    expect(rows[1].get('.el-tag').classes()).toContain('el-tag--danger')
    await rows[0].get('.el-button').trigger('click')
    await flushPromises()
    const dialog = wrapper
      .findAllComponents({ name: 'ElDialog' })
      .find((item) => item.props('modelValue') === true)
    expect(dialog).toBeDefined()
    if (!dialog) throw new Error('recipient rule dialog missing')
    const selects = dialog.findAllComponents({ name: 'ElSelectV2' })
    const scope = selects.find((select) => select.attributes('data-testid') === 'mail-rule-scope')
    const action = selects.find((select) => select.attributes('data-testid') === 'mail-rule-action')
    expect(scope?.props('modelValue')).toBe(1)
    expect(action?.props('modelValue')).toBe(1)
    expect(dialog.get('[data-testid="mail-rule-pattern"]').attributes('placeholder')).toContain(
      '域名',
    )
    scope?.vm.$emit('update:modelValue', 0)
    action?.vm.$emit('update:modelValue', 0)
    await dialog.get('[data-testid="mail-rule-pattern"]').setValue('b@example.com')
    await dialog.get('.el-dialog__footer .el-button--primary').trigger('click')
    await flushPromises()
    expect(mailApi.updateMailRule).toHaveBeenCalledWith(7, {
      scope: 0,
      pattern: 'b@example.com',
      action: 0,
      name: 'Domain allow',
      remark: '',
      isEnabled: 1,
    })
    expect(mailApi.listMailRules).toHaveBeenCalledTimes(2)
  })

  it('passes the recipient rule id when toggling its status', async () => {
    vi.mocked(mailApi.listMailRules).mockResolvedValue([
      {
        id: 7,
        scope: 1,
        pattern: 'example.com',
        action: 0,
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

  it('gates XLSX import and export with their own independent actions', async () => {
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
    vi.mocked(mailApi.exportMailRuleXlsx).mockResolvedValue({
      fileName: 'mail-recipient-rule.xlsx',
      content: new Uint8Array([80, 75, 3, 4, 0]).buffer,
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
    expect(document.querySelector('a[download="mail-recipient-rule.xlsx"]')).toBeNull()
    click.mockRestore()
    vi.unstubAllGlobals()
  })

  it('locks duplicate exports and offers a retry after failure', async () => {
    const pending = deferred<mailApi.MailRuleXlsxExportFile>()
    vi.mocked(mailApi.exportMailRuleXlsx).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage(['message:mail:view', 'message:mail:rule:export'])
    await flushPromises()
    const button = wrapper.get('[data-testid="mail-rule-export"]')
    await button.trigger('click')
    await button.trigger('click')
    expect(mailApi.exportMailRuleXlsx).toHaveBeenCalledOnce()
    pending.reject(new Error('offline'))
    await flushPromises()
    expect(wrapper.text()).toContain('导出失败，请重试')
    const retry = deferred<mailApi.MailRuleXlsxExportFile>()
    vi.mocked(mailApi.exportMailRuleXlsx).mockReturnValueOnce(retry.promise)
    await button.trigger('click')
    expect(mailApi.exportMailRuleXlsx).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('导出失败，请重试')
    retry.reject(new Error('still offline'))
    await flushPromises()
  })

  it('cancels exports and prevents stale downloads when permission is revoked', async () => {
    const pending = deferred<mailApi.MailRuleXlsxExportFile>()
    vi.mocked(mailApi.exportMailRuleXlsx).mockReturnValueOnce(pending.promise)
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    try {
      const wrapper = mountPage(['message:mail:view', 'message:mail:rule:export'])
      await flushPromises()
      await wrapper.get('[data-testid="mail-rule-export"]').trigger('click')
      const signal = vi.mocked(mailApi.exportMailRuleXlsx).mock.calls[0]?.[0]
      expect(signal).toBeInstanceOf(AbortSignal)
      usePermissionStore().applySnapshot({
        roleCodes: [],
        menuTree: [],
        permissionCodes: ['message:mail:list'],
      })
      await flushPromises()
      expect(signal?.aborted).toBe(true)
      pending.resolve({
        fileName: 'mail-recipient-rule.xlsx',
        content: new Uint8Array([80, 75, 3, 4, 0]).buffer,
      })
      await flushPromises()
      expect(click).not.toHaveBeenCalled()
    } finally {
      click.mockRestore()
    }
  })

  it('previews an uploaded XLSX before importing the unchanged content', async () => {
    const content = new Uint8Array([80, 75, 3, 4, 0])
    vi.mocked(mailApi.getMailRuleImportTemplate).mockResolvedValue({
      objectKey:
        'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.xlsx',
    })
    vi.mocked(mailApi.previewMailRuleXlsx).mockResolvedValue({
      rows: [
        {
          line: 2,
          rawValues: ['邮箱', 'a@example.com', '拒绝', '规则', '', '启用'],
          data: {
            scope: MailRuleScope.Email,
            pattern: 'a@example.com',
            action: MailRuleAction.Deny,
            name: '规则',
            remark: '',
            isEnabled: YesNo.Yes,
          },
          errors: [],
        },
      ],
      errors: [],
    })
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:import'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
    await flushPromises()
    expect(
      document.querySelector('[data-testid="mail-rule-template-download"]')?.getAttribute('href'),
    ).toBe('https://example.com/template.xlsx')
    expect(requestObjectURL).toHaveBeenCalledWith(
      'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.xlsx',
      expect.any(AbortSignal),
    )
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
    await uploadRuleXLSX(
      new File([content], 'rules.xlsx', {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    )
    await vi.waitFor(() =>
      expect(mailApi.previewMailRuleXlsx).toHaveBeenCalledWith(
        { fileName: 'rules.xlsx', contentBase64: 'UEsDBAA=' },
        expect.any(AbortSignal),
      ),
    )
    await flushPromises()
    expect(document.body.textContent).toContain('a@example.com')
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(false)
    bodyButton('mail-rule-import-confirm').click()
    await flushPromises()
    expect(mailApi.importMailRuleXlsx).toHaveBeenCalledWith(
      { fileName: 'rules.xlsx', contentBase64: 'UEsDBAA=' },
      expect.any(AbortSignal),
    )
    expect(mailApi.listMailRules).toHaveBeenCalledTimes(2)
  })

  it('displays parsed numeric enums in the active language and raw text only for invalid rows', async () => {
    vi.mocked(mailApi.previewMailRuleXlsx).mockResolvedValue({
      rows: [
        {
          line: 2,
          rawValues: ['RAW-TYPE', 'RAW-PATTERN', 'RAW-ACTION', 'RAW-NAME', '', 'RAW-STATUS'],
          data: {
            scope: MailRuleScope.Email,
            pattern: 'parsed@example.com',
            action: MailRuleAction.Deny,
            name: 'parsed-name',
            remark: '',
            isEnabled: YesNo.Yes,
          },
          errors: [],
        },
        {
          line: 3,
          rawValues: ['RAW-TYPE', 'RAW-PATTERN', 'RAW-ACTION', 'RAW-NAME', '', 'RAW-STATUS'],
          data: {
            scope: MailRuleScope.Domain,
            pattern: 'example.com',
            action: MailRuleAction.Allow,
            name: 'duplicate-name',
            remark: '',
            isEnabled: YesNo.No,
          },
          errors: ['duplicate_existing'],
        },
        {
          line: 4,
          rawValues: ['原始错误类型', '@raw-domain', '原始错误动作', '原始名称', '', '原始状态'],
          data: null,
          errors: ['invalid_scope', 'invalid_action'],
        },
      ],
      errors: [],
    })
    await openImport()
    await uploadRuleXLSX(xlsxFile())
    await vi.waitFor(() => expect(document.querySelectorAll('.xlsx-values')).toHaveLength(3))
    const displayedRows = () =>
      [...document.querySelectorAll('.xlsx-values')].map((row) => row.textContent)
    expect(displayedRows()).toEqual([
      '邮箱 | parsed@example.com | 拒绝 | parsed-name |  | 启用',
      '域名 | example.com | 允许 | duplicate-name |  | 停用',
      '原始错误类型 | @raw-domain | 原始错误动作 | 原始名称 |  | 原始状态',
    ])
    setLocale('en-US')
    await flushPromises()
    expect(displayedRows()).toEqual([
      'Email | parsed@example.com | Denylist | parsed-name |  | Enabled',
      'Domain | example.com | Allowlist | duplicate-name |  | Disabled',
      '原始错误类型 | @raw-domain | 原始错误动作 | 原始名称 |  | 原始状态',
    ])
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
  })

  it('shows row errors and does not allow a partial import', async () => {
    vi.mocked(mailApi.previewMailRuleXlsx).mockResolvedValue({
      rows: [
        {
          line: 2,
          rawValues: ['域名', '@qq.com', '拒绝', '错误', '', '启用'],
          data: null,
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
    await uploadRuleXLSX(
      new File(['bad'], 'rules.xlsx', {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    )
    await vi.waitFor(() => expect(mailApi.previewMailRuleXlsx).toHaveBeenCalledOnce())
    await flushPromises()
    expect(document.body.textContent).toContain('邮箱或域名格式无效')
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
    expect(mailApi.importMailRuleXlsx).not.toHaveBeenCalled()
    expect(requestObjectURL).not.toHaveBeenCalled()
  })

  it('reports template object resolution failure without guessing a download URL', async () => {
    vi.mocked(mailApi.getMailRuleImportTemplate).mockResolvedValue({
      objectKey:
        'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.xlsx',
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
        'file/.admin-storage/v2/p1/r1/c1/v1/2026/09/30/0123456789abcdef0123456789abcdef.xlsx',
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
    pending.resolve({ url: 'https://example.com/stale.xlsx', expiresAt: null })
    await flushPromises()
    expect(document.querySelector('[data-testid="mail-rule-template-download"]')).toBeNull()
  })

  it('discards a stale preview after a new file is selected', async () => {
    const older = deferred<mailApi.MailRuleXlsxPreview>()
    vi.mocked(mailApi.previewMailRuleXlsx)
      .mockReturnValueOnce(older.promise)
      .mockResolvedValueOnce({
        rows: [
          {
            line: 2,
            rawValues: ['邮箱', 'new@example.com', '拒绝', 'new', '', '启用'],
            data: {
              scope: MailRuleScope.Email,
              pattern: 'new@example.com',
              action: MailRuleAction.Deny,
              name: 'new',
              remark: '',
              isEnabled: YesNo.Yes,
            },
            errors: [],
          },
        ],
        errors: [],
      })
    const wrapper = mountPage(['message:mail:list', 'message:mail:rule:import'])
    await flushPromises()
    await selectTab(wrapper, '收件规则')
    await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
    await flushPromises()
    await uploadRuleXLSX(
      new File(['old'], 'old.xlsx', {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    )
    await vi.waitFor(() => expect(mailApi.previewMailRuleXlsx).toHaveBeenCalledOnce())
    await uploadRuleXLSX(
      new File(['new'], 'new.xlsx', {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      }),
    )
    await vi.waitFor(() => expect(mailApi.previewMailRuleXlsx).toHaveBeenCalledTimes(2))
    await flushPromises()
    older.resolve({
      rows: [
        {
          line: 2,
          rawValues: ['邮箱', 'old@example.com', '拒绝', 'old', '', '启用'],
          data: {
            scope: MailRuleScope.Email,
            pattern: 'old@example.com',
            action: MailRuleAction.Deny,
            name: 'old',
            remark: '',
            isEnabled: YesNo.Yes,
          },
          errors: [],
        },
      ],
      errors: [],
    })
    await flushPromises()
    expect(document.body.textContent).toContain('new@example.com')
    expect(document.body.textContent).not.toContain('old@example.com')
  })

  it('shows a preview failure and retries the unchanged XLSX before confirming', async () => {
    vi.mocked(mailApi.previewMailRuleXlsx)
      .mockRejectedValueOnce(new Error('unavailable'))
      .mockResolvedValue({
        rows: [
          {
            line: 2,
            rawValues: ['邮箱', 'retry@example.com', '拒绝', '重试', '', '启用'],
            data: {
              scope: MailRuleScope.Email,
              pattern: 'retry@example.com',
              action: MailRuleAction.Deny,
              name: '重试',
              remark: '',
              isEnabled: YesNo.Yes,
            },
            errors: [],
          },
        ],
        errors: [],
      })
    const wrapper = await openImport()
    await uploadRuleXLSX(xlsxFile())
    await vi.waitFor(() => expect(document.body.textContent).toContain('预览失败'))
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
    bodyButton('mail-rule-import-preview').click()
    await flushPromises()
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(false)
    vi.mocked(mailApi.importMailRuleXlsx).mockRejectedValueOnce(new Error('conflict'))
    bodyButton('mail-rule-import-confirm').click()
    await flushPromises()
    expect(document.body.textContent).toContain('导入未确认成功')
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
    expect(wrapper.findAllComponents({ name: 'AppTable' })).toHaveLength(1)
    bodyButton('mail-rule-import-preview').click()
    await flushPromises()
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(false)
  })

  it.each(['rules.csv', 'rules.xls', 'rules.xlsm'])(
    'rejects %s before reading or previewing',
    async (fileName) => {
      await openImport()
      await uploadRuleXLSX(xlsxFile(fileName))
      expect(document.body.textContent).toContain('请选择 .xlsx 文件')
      expect(mailApi.previewMailRuleXlsx).not.toHaveBeenCalled()
    },
  )

  it('rejects empty and oversized files before previewing', async () => {
    await openImport()
    await uploadRuleXLSX(new File([], 'empty.xlsx'))
    expect(document.body.textContent).toContain('没有可导入的规则')
    await uploadRuleXLSX(new File([new Uint8Array(2 * 1024 * 1024 + 1)], 'large.xlsx'))
    expect(document.body.textContent).toContain('不能超过 2 MiB')
    expect(mailApi.previewMailRuleXlsx).not.toHaveBeenCalled()
  })

  it('renders empty validation results without allowing confirmation', async () => {
    vi.mocked(mailApi.previewMailRuleXlsx).mockResolvedValue({ rows: [], errors: ['empty'] })
    await openImport()
    await uploadRuleXLSX(xlsxFile())
    await vi.waitFor(() => expect(document.body.textContent).toContain('没有可导入的规则'))
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
  })

  it('cancels a pending preview on close and ignores its eventual response', async () => {
    const pending = deferred<mailApi.MailRuleXlsxPreview>()
    vi.mocked(mailApi.previewMailRuleXlsx).mockReturnValue(pending.promise)
    const wrapper = await openImport()
    await uploadRuleXLSX(xlsxFile())
    await vi.waitFor(() => expect(mailApi.previewMailRuleXlsx).toHaveBeenCalledOnce())
    const signal = vi.mocked(mailApi.previewMailRuleXlsx).mock.calls[0]?.[1]
    expect(signal).toBeInstanceOf(AbortSignal)
    bodyButton('mail-rule-import-cancel').click()
    await flushPromises()
    expect(signal?.aborted).toBe(true)
    pending.resolve({
      rows: [
        {
          line: 2,
          rawValues: ['邮箱', 'stale@example.com', '拒绝', '过期', '', '启用'],
          data: {
            scope: MailRuleScope.Email,
            pattern: 'stale@example.com',
            action: MailRuleAction.Deny,
            name: '过期',
            remark: '',
            isEnabled: YesNo.Yes,
          },
          errors: [],
        },
      ],
      errors: [],
    })
    await flushPromises()
    await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
    await flushPromises()
    expect(document.body.textContent).not.toContain('stale@example.com')
    expect(bodyButton('mail-rule-import-confirm').disabled).toBe(true)
  })

  it('does not duplicate preview or confirmation requests while pending', async () => {
    const pending = deferred<mailApi.MailRuleXlsxPreview>()
    vi.mocked(mailApi.previewMailRuleXlsx).mockReturnValue(pending.promise)
    const wrapper = await openImport()
    await uploadRuleXLSX(xlsxFile())
    await vi.waitFor(() => expect(mailApi.previewMailRuleXlsx).toHaveBeenCalledOnce())
    const tables = wrapper.findAllComponents({ name: 'AppTable' })
    tables.at(-1)?.vm.$emit('refresh')
    tables.at(-1)?.vm.$emit('refresh')
    await flushPromises()
    expect(mailApi.previewMailRuleXlsx).toHaveBeenCalledOnce()
    pending.resolve({
      rows: [
        {
          line: 2,
          rawValues: ['邮箱', 'a@example.com', '拒绝', '名称', '', '启用'],
          data: {
            scope: MailRuleScope.Email,
            pattern: 'a@example.com',
            action: MailRuleAction.Deny,
            name: '名称',
            remark: '',
            isEnabled: YesNo.Yes,
          },
          errors: [],
        },
      ],
      errors: [],
    })
    await flushPromises()
    const imported = deferred<{ imported: number }>()
    vi.mocked(mailApi.importMailRuleXlsx).mockReturnValueOnce(imported.promise)
    bodyButton('mail-rule-import-confirm').click()
    bodyButton('mail-rule-import-confirm').click()
    await flushPromises()
    expect(mailApi.importMailRuleXlsx).toHaveBeenCalledOnce()
    usePermissionStore().applySnapshot({
      roleCodes: [],
      menuTree: [],
      permissionCodes: ['message:mail:list'],
    })
    imported.resolve({ imported: 1 })
    await flushPromises()
    expect(mailApi.listMailRules).toHaveBeenCalledOnce()
  })

  it('retries failed template loading inside the dialog', async () => {
    vi.mocked(mailApi.getMailRuleImportTemplate)
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({
        objectKey:
          'setting/.admin-storage/v2/p1/r1/c1/v1/2026/10/08/0123456789abcdef0123456789abcdef.xlsx',
      })
    await openImport()
    expect(document.body.textContent).toContain('模板下载地址加载失败')
    bodyButton('mail-rule-template-retry').click()
    await flushPromises()
    expect(
      document.querySelector('[data-testid="mail-rule-template-download"]')?.textContent,
    ).toContain('下载 Excel 模板')
  })

  it('uses Excel instructions in both languages and preserves Chinese enum guidance', async () => {
    const wrapper = await openImport()
    expect(document.body.textContent).toContain('Excel 模板（.xlsx）')
    expect(document.body.textContent).toContain('邮箱/域名')
    expect(document.body.textContent).not.toMatch(/CSV|Workbook|ruleXlsx/u)
    setLocale('en-US')
    await flushPromises()
    expect(wrapper.text()).toContain('Import Excel')
    expect(document.body.textContent).toContain('Excel template (.xlsx)')
    expect(document.body.textContent).toContain('启用/停用')
    expect(document.body.textContent).not.toMatch(/CSV|Workbook|ruleXlsx/u)
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
      await uploadRuleXLSX(
        new File(['pending'], 'pending.xlsx', {
          type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        }),
      )
      expect(wrapper.findAllComponents({ name: 'AppTable' })).toHaveLength(2)
      await uploadRuleXLSX(new File(['not Excel'], 'replacement.txt', { type: 'text/plain' }))
      expect(document.body.textContent).toContain('请选择 .xlsx 文件')
      expect(wrapper.findAllComponents({ name: 'AppTable' })).toHaveLength(1)
      expect(mailApi.previewMailRuleXlsx).not.toHaveBeenCalled()
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

async function openImport(): Promise<VueWrapper> {
  const wrapper = mountPage(['message:mail:list', 'message:mail:rule:import'])
  await flushPromises()
  await selectTab(wrapper, '收件规则')
  await wrapper.get('[data-testid="mail-rule-import"]').trigger('click')
  await flushPromises()
  return wrapper
}

function xlsxFile(fileName = 'rules.xlsx'): File {
  return new File([new Uint8Array([80, 75, 3, 4, 0])], fileName, {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  })
}

async function uploadRuleXLSX(file: File): Promise<void> {
  const input = document.querySelector('[data-testid="mail-rule-import-file"]')
  if (!(input instanceof HTMLInputElement)) throw new Error('XLSX input missing')
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
