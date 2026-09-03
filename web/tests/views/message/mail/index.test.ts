import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as mailApi from '@/api/message/mail'
import { YesNo } from '@/enums/yes-no'
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
  deleteMailLog: vi.fn(),
  deleteMailLogs: vi.fn(),
  listMailRules: vi.fn(),
  createMailRule: vi.fn(),
  updateMailRule: vi.fn(),
  updateMailRuleStatus: vi.fn(),
  deleteMailRule: vi.fn(),
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
        platformId: 1,
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

  it('renders controls only for granted action permissions', async () => {
    const wrapper = mountPage([
      'message:mail:list',
      'message:mail:detail',
      'message:mail:template:update',
      'message:mail:log:delete',
      'message:mail:rule:create',
    ])
    await flushPromises()
    expect(wrapper.find('[data-testid="mail-config-save"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('发送日志')

    await selectTab(wrapper, '邮件模板')
    expect(wrapper.find('[data-testid="mail-template-edit"]').exists()).toBe(true)
    await selectTab(wrapper, '发送日志')
    expect(wrapper.find('[data-testid="mail-log-batch-delete"]').exists()).toBe(true)
    await selectTab(wrapper, '黑白名单')
    expect(wrapper.find('[data-testid="mail-rule-create"]').exists()).toBe(true)
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
