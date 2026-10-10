import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as mailApi from '@/api/message/mail'
import { appI18n, setLocale } from '@/i18n'
import { mailOptions } from './fixtures'
import RateLimitTab from '@/views/message/mail/rateLimitPolicy/index.vue'

vi.mock('@/api/message/mail', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/message/mail')>()
  return { ...actual, updateMailRateLimitPolicy: vi.fn() }
})

const policies = [
  {
    platformId: 1,
    rowId: '1:business_email_minute',
    key: 'business_email_minute',
    mode: 'business' as const,
    dimension: 'platform_email',
    limit: 1,
    windowSeconds: 60,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    platformId: 1,
    rowId: '1:business_email_10m',
    key: 'business_email_10m',
    mode: 'business' as const,
    dimension: 'platform_email',
    limit: 5,
    windowSeconds: 600,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    platformId: 2,
    rowId: '2:business_email_minute',
    key: 'business_email_minute',
    mode: 'business' as const,
    dimension: 'platform_email',
    limit: 1,
    windowSeconds: 60,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    platformId: 2,
    rowId: '2:business_email_10m',
    key: 'business_email_10m',
    mode: 'business' as const,
    dimension: 'platform_email',
    limit: 5,
    windowSeconds: 600,
    updatedAt: '2026-09-04T12:00:00Z',
  },
]

function mountTab(canUpdate: boolean): VueWrapper {
  return mount(RateLimitTab, {
    props: { options: mailOptions(), policies, loading: false, canUpdate },
    global: { plugins: [ElementPlus, appI18n] },
  })
}

describe('mail rate limit tab', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    vi.clearAllMocks()
  })

  it('takes numeric bounds from backend constraints and fails closed when unavailable', async () => {
    const options = mailOptions()
    options.rateLimitConstraints = {
      minLimit: 2,
      maxLimit: 7,
      minWindowSeconds: 20,
      maxWindowSeconds: 90,
    }
    const wrapper = mountTab(true)
    await wrapper.setProps({ options })
    await flushPromises()
    const limitInput = wrapper.get('[data-testid="rate-limit-input"] input')
    const windowInput = wrapper.get('[data-testid="rate-limit-window-input"] input')
    expect(limitInput.attributes('min')).toBe('2')
    expect(limitInput.attributes('max')).toBe('7')
    expect(windowInput.attributes('max')).toBe('90')
    await wrapper.setProps({ options: null })
    expect(limitInput.attributes('disabled')).toBeDefined()
    expect(
      wrapper.get('[data-testid="rate-limit-save-1:business_email_minute"]').attributes('disabled'),
    ).toBeDefined()
    wrapper.unmount()
  })

  it('renders every platform catalog with localized placeholders', async () => {
    const wrapper = mountTab(false)
    await flushPromises()

    expect(wrapper.findAll('[data-testid="rate-limit-input"]')).toHaveLength(4)
    expect(wrapper.findAll('input[type="number"]').length).toBeGreaterThan(0)
    expect(wrapper.text()).toContain('每分钟发送上限')
    expect(wrapper.text()).toContain('平台·邮箱')
  })

  it('uses platform and policy key as the stable table row key', () => {
    const table = wrapperTable(mountTab(false))
    expect(table.props('rowKey')).toBe('rowId')
  })

  it('disables inputs and hides save buttons without update permission', async () => {
    const wrapper = mountTab(false)
    await flushPromises()

    expect(wrapper.findAll('[data-testid^="rate-limit-save-"]')).toHaveLength(0)
  })

  it('sends only the edited row and keeps a per-row saving state', async () => {
    vi.mocked(mailApi.updateMailRateLimitPolicy).mockResolvedValue({
      platformId: 1,
      policy: { ...policies[0], limit: 2, windowSeconds: 120 },
    })
    const wrapper = mountTab(true)
    await flushPromises()

    const inputs = wrapper.findAll('[data-testid="rate-limit-input"] input')
    await inputs[0].setValue(2)
    await flushPromises()

    const save = wrapper.find('[data-testid="rate-limit-save-1:business_email_minute"]')
    expect(save.exists()).toBe(true)
    await save.trigger('click')
    await flushPromises()

    expect(mailApi.updateMailRateLimitPolicy).toHaveBeenCalledWith(1, 'business_email_minute', {
      limit: 2,
      windowSeconds: 60,
    })
  })

  it('restores the server value when an update fails', async () => {
    vi.mocked(mailApi.updateMailRateLimitPolicy).mockRejectedValue(new Error('request failed'))
    const wrapper = mountTab(true)
    await flushPromises()

    const input = wrapper.find('[data-testid="rate-limit-input"] input')
    await input.setValue(2)
    await flushPromises()
    await wrapper.find('[data-testid="rate-limit-save-1:business_email_minute"]').trigger('click')
    await flushPromises()

    expect((input.element as HTMLInputElement).value).toBe('1')
  })
})

function wrapperTable(wrapper: VueWrapper) {
  return wrapper.findComponent({ name: 'ElTable' })
}
