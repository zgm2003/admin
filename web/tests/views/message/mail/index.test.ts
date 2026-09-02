import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as mailApi from '@src/api/system/mail'
import { YesNo } from '@src/enums/yes-no'
import { appI18n, setLocale } from '@src/i18n'
import { usePermissionStore } from '@src/store/permission'
import MailPage from '@src/views/message/mail/index.vue'

vi.mock('@src/api/system/mail', () => ({
  getMailConfig: vi.fn(), saveMailConfig: vi.fn(), deleteMailConfig: vi.fn(), sendMailTest: vi.fn(),
  listMailTemplates: vi.fn(), updateMailTemplate: vi.fn(), updateMailTemplateStatus: vi.fn(),
  listMailLogs: vi.fn(), getMailLogDetail: vi.fn(), deleteMailLog: vi.fn(), deleteMailLogs: vi.fn(),
  listMailRules: vi.fn(), createMailRule: vi.fn(), updateMailRule: vi.fn(), updateMailRuleStatus: vi.fn(), deleteMailRule: vi.fn(),
}))

describe('mail service page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(mailApi.getMailConfig).mockResolvedValue({ configured: true, region: 'ap-guangzhou', endpoint: '', fromEmail: 'sender@example.com', fromName: 'Admin', replyTo: '', ttlMinutes: 10, isEnabled: YesNo.Yes, lastTestAt: null, lastTestError: '' })
    vi.mocked(mailApi.listMailTemplates).mockResolvedValue([{
      id: 1, platformId: 1, scene: 'login', name: '登录验证码', subject: '登录验证码', tencentTemplateId: 47941,
      variables: { code: '123456', ttl_minutes: '10' }, exampleVariables: { code: '123456', ttl_minutes: '10' },
      isEnabled: YesNo.Yes, createdAt: '2026-09-01T00:00:00Z', updatedAt: '2026-09-01T00:00:00Z'
    }])
    vi.mocked(mailApi.listMailLogs).mockResolvedValue({ list: [], total: 0, page: 1, pageSize: 20 })
    vi.mocked(mailApi.listMailRules).mockResolvedValue([])
  })
  afterEach(() => { document.body.innerHTML = '' })

  it('hides the log tab without detail permission and never restores secrets from config', async () => {
    const wrapper = mountPage(['system:mail:list'])
    await flushPromises()
    expect(wrapper.text()).not.toContain('发送日志')
    const passwords = wrapper.findAll('input[type="password"]')
    expect(passwords).toHaveLength(2)
    expect(passwords.every((input) => (input.element as HTMLInputElement).value === '')).toBe(true)
  })

  it('renders controls only for granted action permissions', async () => {
    const wrapper = mountPage(['system:mail:list', 'system:mail:detail', 'system:mail:template:update', 'system:mail:log:delete', 'system:mail:rule:create'])
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
  return mount(MailPage, { attachTo: document.body, global: { plugins: [pinia, appI18n, ElementPlus] } })
}

async function selectTab(wrapper: VueWrapper, label: string) {
  const tab = wrapper.findAll('[role="tab"]').find((item) => item.text() === label)
  if (!tab) throw new Error(`tab not found: ${label}`)
  await tab.trigger('click')
  await flushPromises()
}
