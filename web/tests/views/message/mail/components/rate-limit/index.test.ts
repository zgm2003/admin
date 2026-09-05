import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as mailApi from '@/api/message/mail'
import { appI18n, setLocale } from '@/i18n'
import RateLimitTab from '@/views/message/mail/components/rate-limit/index.vue'

vi.mock('@/api/message/mail', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/message/mail')>()
  return { ...actual, updateMailRateLimitPolicy: vi.fn() }
})

const policies = [
  {
    key: 'business_email_minute',
    mode: 'business' as const,
    dimension: 'platform_scene_email',
    limit: 1,
    windowSeconds: 60,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    key: 'business_email_10m',
    mode: 'business' as const,
    dimension: 'platform_scene_email',
    limit: 5,
    windowSeconds: 600,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    key: 'business_ip_minute',
    mode: 'business' as const,
    dimension: 'platform_ip',
    limit: 10,
    windowSeconds: 60,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    key: 'business_scene_minute',
    mode: 'business' as const,
    dimension: 'platform_scene',
    limit: 30,
    windowSeconds: 60,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    key: 'admin_test_user_10m',
    mode: 'admin_test' as const,
    dimension: 'admin_user',
    limit: 5,
    windowSeconds: 600,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    key: 'admin_test_ip_minute',
    mode: 'admin_test' as const,
    dimension: 'ip',
    limit: 10,
    windowSeconds: 60,
    updatedAt: '2026-09-04T12:00:00Z',
  },
  {
    key: 'admin_test_email_10m',
    mode: 'admin_test' as const,
    dimension: 'email',
    limit: 3,
    windowSeconds: 600,
    updatedAt: '2026-09-04T12:00:00Z',
  },
]

function mountTab(canUpdate: boolean): VueWrapper {
  return mount(RateLimitTab, {
    props: { policies, loading: false, canUpdate },
    global: { plugins: [ElementPlus, appI18n] },
  })
}

describe('mail rate limit tab', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    vi.clearAllMocks()
  })

  it('renders seven fixed policies with localized placeholders', async () => {
    const wrapper = mountTab(false)
    await flushPromises()

    expect(wrapper.findAll('[data-testid="rate-limit-input"]')).toHaveLength(7)
    expect(wrapper.findAll('input[type="number"]').length).toBeGreaterThan(0)
    expect(wrapper.text()).toContain('单邮箱每分钟')
    expect(wrapper.text()).toContain('平台·场景·邮箱')
  })

  it('uses policy key as the stable table row key', () => {
    const table = wrapperTable(mountTab(false))
    expect(table.props('rowKey')).toBe('key')
  })

  it('disables inputs and hides save buttons without update permission', async () => {
    const wrapper = mountTab(false)
    await flushPromises()

    expect(wrapper.findAll('[data-testid^="rate-limit-save-"]')).toHaveLength(0)
  })

  it('sends only the edited row and keeps a per-row saving state', async () => {
    vi.mocked(mailApi.updateMailRateLimitPolicy).mockResolvedValue({
      version: 2,
      policy: { ...policies[0], limit: 2, windowSeconds: 120 },
    })
    const wrapper = mountTab(true)
    await flushPromises()

    const inputs = wrapper.findAll('[data-testid="rate-limit-input"] input')
    await inputs[0].setValue(2)
    await flushPromises()

    const save = wrapper.find('[data-testid="rate-limit-save-business_email_minute"]')
    expect(save.exists()).toBe(true)
    await save.trigger('click')
    await flushPromises()

    expect(mailApi.updateMailRateLimitPolicy).toHaveBeenCalledWith('business_email_minute', {
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
    await wrapper.find('[data-testid="rate-limit-save-business_email_minute"]').trigger('click')
    await flushPromises()

    expect((input.element as HTMLInputElement).value).toBe('1')
  })
})

function wrapperTable(wrapper: VueWrapper) {
  return wrapper.findComponent({ name: 'ElTable' })
}
