import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as loginLogAPI from '@/api/user/loginLog'
import type { LoginLogItem } from '@/api/user/loginLog'
import { appI18n, setLocale } from '@/i18n'
import LoginLogManagement from '@/views/user/loginLog/index.vue'

vi.mock('@/api/user/loginLog', async () => {
  const actual = await vi.importActual<typeof import('@/api/user/loginLog')>('@/api/user/loginLog')
  return { ...actual, getLoginLogs: vi.fn() }
})

const getLoginLogs = vi.mocked(loginLogAPI.getLoginLogs)
const mountedWrappers: VueWrapper[] = []

describe('login log management', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    getLoginLogs.mockReset()
    getLoginLogs.mockResolvedValue({ list: rows(), total: 3, page: 1, pageSize: 20 })
  })

  afterEach(() => {
    for (const wrapper of mountedWrappers.splice(0)) wrapper.unmount()
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('renders numeric event tags, login type labels, and logout dash', async () => {
    const wrapper = mount(LoginLogManagement, {
      attachTo: document.body,
      global: { plugins: [appI18n, ElementPlus] },
    })
    mountedWrappers.push(wrapper)
    await flushPromises()

    expect(getLoginLogs).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
    expect(wrapper.text()).toContain('注册')
    expect(wrapper.text()).toContain('登录')
    expect(wrapper.text()).toContain('登出')
    expect(wrapper.text()).toContain('邮箱')
    expect(wrapper.text()).toContain('密码')
    expect(wrapper.text()).toContain('-')
    expect(wrapper.find('.el-tag--primary').exists()).toBe(true)
    expect(wrapper.find('.el-tag--success').exists()).toBe(true)
    expect(wrapper.find('.el-tag--danger').exists()).toBe(true)
  })
})

function rows(): LoginLogItem[] {
  return [
    item(1, 1, 2, 'a@example.com'),
    item(2, 2, 1, 'a@example.com'),
    item(3, 3, null, 'user_abc'),
  ]
}

function item(id: number, eventType: 1 | 2 | 3, loginType: 1 | 2 | 3 | null, account: string) {
  return {
    id,
    userId: 1,
    platform: 'admin',
    account,
    eventType,
    loginType,
    isSuccess: 1 as const,
    reasonCode: 'success',
    clientIp: '127.0.0.1',
    userAgent: 'Vitest',
    createdAt: '2026-01-01T00:00:00Z',
  }
}
