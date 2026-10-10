import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as loginLogAPI from '@/api/user/loginLog'
import * as loginLogOptionsAPI from '@/api/user/loginLogOptions'
import type { LoginLogItem } from '@/api/user/loginLog'
import { appI18n, setLocale } from '@/i18n'
import LoginLogManagement from '@/views/user/loginLog/index.vue'

vi.mock('@/api/user/loginLog', async () => {
  const actual = await vi.importActual<typeof import('@/api/user/loginLog')>('@/api/user/loginLog')
  return { ...actual, getLoginLogs: vi.fn() }
})

const getLoginLogs = vi.mocked(loginLogAPI.getLoginLogs)
vi.mock('@/api/user/loginLogOptions', () => ({ getLoginLogOptions: vi.fn() }))
const mountedWrappers: VueWrapper[] = []

describe('login log management', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    getLoginLogs.mockReset()
    vi.mocked(loginLogOptionsAPI.getLoginLogOptions)
      .mockReset()
      .mockResolvedValue({
        eventTypes: [
          { value: 1, label: 'Go注册' },
          { value: 2, label: 'Go登录' },
          { value: 3, label: 'Go退出' },
        ],
        loginTypes: [
          { value: 1, label: 'Go密码' },
          { value: 2, label: 'Go邮箱' },
        ],
      })
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
    expect(wrapper.text()).toContain('Go退出')
    expect(wrapper.text()).toContain('邮箱')
    expect(wrapper.text()).toContain('密码')
    expect(wrapper.text()).toContain('-')
    expect(wrapper.find('.el-tag--info').exists()).toBe(true)
  })

  it('displays future numeric values using backend labels and raw values when unknown', async () => {
    vi.mocked(loginLogOptionsAPI.getLoginLogOptions).mockResolvedValue({
      eventTypes: [{ value: 99, label: '未来事件' }],
      loginTypes: [{ value: 99, label: '未来登录方式' }],
    })
    getLoginLogs.mockResolvedValue({
      list: [item(1, 99, 99, 'future'), item(2, 88, 88, 'unknown')],
      total: 2,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mount(LoginLogManagement, { global: { plugins: [appI18n, ElementPlus] } })
    mountedWrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.text()).toContain('未来事件')
    expect(wrapper.text()).toContain('未来登录方式')
    expect(wrapper.text()).toContain('88')
    const fields = wrapper.findAllComponents({ name: 'ElSelectV2' })
    expect(fields[0]?.props('options')).toEqual([{ value: 99, label: '未来事件' }])
    fields[0]?.vm.$emit('update:modelValue', 99)
    fields[1]?.vm.$emit('update:modelValue', 99)
    await wrapper.get('[data-testid="login-log-search"]').trigger('click')
    await flushPromises()
    expect(getLoginLogs).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      eventType: 99,
      loginType: 99,
    })
  })

  it('clears failed options and refreshes labels after a locale change', async () => {
    vi.mocked(loginLogOptionsAPI.getLoginLogOptions)
      .mockRejectedValueOnce(new Error('候选项失败'))
      .mockResolvedValue({ eventTypes: [{ value: 1, label: 'Go registration' }], loginTypes: [] })
    const wrapper = mount(LoginLogManagement, { global: { plugins: [appI18n, ElementPlus] } })
    mountedWrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.text()).toContain('候选项失败')
    expect(wrapper.findAllComponents({ name: 'ElSelectV2' })[0]?.props('options')).toEqual([])
    setLocale('en-US')
    await flushPromises()
    expect(wrapper.text()).toContain('Go registration')
    expect(loginLogOptionsAPI.getLoginLogOptions).toHaveBeenCalledTimes(2)
  })
})

function rows(): LoginLogItem[] {
  return [
    item(1, 1, 2, 'a@example.com'),
    item(2, 2, 1, 'a@example.com'),
    item(3, 3, null, 'user_abc'),
  ]
}

function item(id: number, eventType: number, loginType: number | null, account: string) {
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
