import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser, getLoginConfig, login, sendLoginCode } from '@/api/auth/login'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { useAuthStore } from '@/store/auth'
import { ApiError } from '@/types/http'
import LoginPage from '@/views/auth/login/index.vue'

vi.mock('@/api/auth/login', () => ({
  login: vi.fn(),
  getCurrentUser: vi.fn(),
  getLoginConfig: vi.fn(),
  sendLoginCode: vi.fn(),
}))

const loginMock = vi.mocked(login)
const getCurrentUserMock = vi.mocked(getCurrentUser)
const getLoginConfigMock = vi.mocked(getLoginConfig)
const sendLoginCodeMock = vi.mocked(sendLoginCode)

describe('Login page', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    useAuthStore(pinia).$reset()
    loginMock.mockReset()
    getCurrentUserMock.mockReset()
    getLoginConfigMock.mockReset()
    sendLoginCodeMock.mockReset()
    getLoginConfigMock.mockResolvedValue({
      loginTypes: [
        { value: 'password', label: '密码' },
        { value: 'email', label: '邮箱验证码' },
      ],
      allowRegister: false,
    })
  })

  it('renders the product identity and a login form', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.get('[data-testid="login-brand"]').text()).toContain('Admin')
    expect(wrapper.find('[data-testid="login-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-account"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-password"]').exists()).toBe(true)
  })

  it('does not expose a fallback form and retries config loading after failure', async () => {
    getLoginConfigMock
      .mockRejectedValueOnce(new ApiError(10006, '服务暂未就绪', 503))
      .mockResolvedValueOnce({
        loginTypes: [{ value: 'email', label: '邮箱验证码' }],
        allowRegister: true,
      })
    const { wrapper } = await mountLogin()
    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.find('[data-testid="login-account"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="login-config-error"]').text()).toContain(
      '暂时无法加载登录方式',
    )

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(getLoginConfigMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="login-config-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="login-account"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-code"]').exists()).toBe(true)
  })

  it('submits a password login, loads me, and follows a safe redirect', async () => {
    const order: string[] = []
    loginMock.mockImplementation(async () => {
      order.push('login')
      return { accessToken: 'jwt', expiresIn: 900, isNewUser: false }
    })
    getCurrentUserMock.mockImplementation(async () => {
      order.push('me')
      return { userId: 1, username: 'admin', email: 'admin@example.com', phone: null, avatar: '' }
    })
    const { wrapper, router } = await mountLogin('/login?redirect=/secure')
    await wrapper.get('[data-testid="login-account"]').setValue(' Admin@Example.COM ')
    await wrapper.get('[data-testid="login-password"]').setValue('  password  ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(loginMock).toHaveBeenCalledWith({
      loginType: 'password',
      loginAccount: 'Admin@Example.COM',
      password: '  password  ',
    })
    expect(order).toEqual(['login', 'me'])
    expect(useAuthStore(pinia).status).toBe('authenticated')
    expect(router.currentRoute.value.path).toBe('/secure')
  })

  it('switches to email code mode and sends a code', async () => {
    sendLoginCodeMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-07T10:10:00Z',
    })
    const { wrapper } = await mountLogin()
    await wrapper.find('[data-testid="login-account"]').setValue('admin@example.com')
    // Switch to email code mode via the segmented control.
    const segmented = wrapper.findComponent({ name: 'ElSegmented' })
    await segmented.vm.$emit('update:modelValue', 'email')
    await flushPromises()

    expect(wrapper.find('[data-testid="login-code"]').exists()).toBe(true)
    await wrapper.get('[data-testid="login-send-code"]').trigger('click')
    await flushPromises()
    expect(sendLoginCodeMock).toHaveBeenCalledWith(
      'admin@example.com',
      'email',
      'login',
      expect.any(String),
    )
  })

  it('rotates the challenge after a successful delivery', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-07T10:00:00Z'))
    try {
      sendLoginCodeMock
        .mockResolvedValueOnce({ challengeId: 'challenge-1', expiresAt: '2026-09-07T10:00:01Z' })
        .mockResolvedValueOnce({ challengeId: 'challenge-2', expiresAt: '2026-09-07T10:00:02Z' })
      const { wrapper } = await mountLogin()
      await wrapper.get('[data-testid="login-account"]').setValue('admin@example.com')
      await wrapper.findComponent({ name: 'ElSegmented' }).vm.$emit('update:modelValue', 'email')
      await flushPromises()

      await wrapper.get('[data-testid="login-send-code"]').trigger('click')
      await flushPromises()
      const firstChallenge = sendLoginCodeMock.mock.calls[0]?.[3]

      await vi.advanceTimersByTimeAsync(1000)
      await wrapper.get('[data-testid="login-send-code"]').trigger('click')
      await flushPromises()
      const secondChallenge = sendLoginCodeMock.mock.calls[1]?.[3]

      expect(firstChallenge).toBeTypeOf('string')
      expect(secondChallenge).toBeTypeOf('string')
      expect(secondChallenge).not.toBe(firstChallenge)
    } finally {
      vi.useRealTimers()
    }
  })

  it('rotates the challenge before retrying a failed delivery', async () => {
    sendLoginCodeMock
      .mockRejectedValueOnce(new ApiError(10006, '服务暂未就绪', 503))
      .mockResolvedValueOnce({
        challengeId: 'challenge-2',
        expiresAt: new Date(Date.now() + 600_000).toISOString(),
      })
    const { wrapper } = await mountLogin()
    await wrapper.get('[data-testid="login-account"]').setValue('admin@example.com')
    await wrapper.findComponent({ name: 'ElSegmented' }).vm.$emit('update:modelValue', 'email')
    await flushPromises()

    await wrapper.get('[data-testid="login-send-code"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="login-send-code"]').trigger('click')
    await flushPromises()

    expect(sendLoginCodeMock.mock.calls[1]?.[3]).not.toBe(sendLoginCodeMock.mock.calls[0]?.[3])
  })

  it('locks submit while pending and shows one credential error', async () => {
    let rejectLogin: (error: Error) => void = () => undefined
    loginMock.mockImplementation(
      () =>
        new Promise((_, reject) => {
          rejectLogin = reject
        }),
    )
    const { wrapper } = await mountLogin()
    await wrapper.get('[data-testid="login-account"]').setValue('admin@example.com')
    await wrapper.get('[data-testid="login-password"]').setValue('wrong')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="login-submit"]').attributes('disabled')).toBeDefined()

    rejectLogin(new ApiError(10002, '未登录或登录已失效', 401))
    await flushPromises()
    expect(wrapper.get('[data-testid="login-error"]').text()).toContain('账号或凭据错误')
  })

  it('leaves non-credential failures to the request notification', async () => {
    loginMock.mockRejectedValue(new ApiError(10006, '服务暂未就绪', 503))
    const { wrapper } = await mountLogin()
    await wrapper.get('[data-testid="login-account"]').setValue('admin@example.com')
    await wrapper.get('[data-testid="login-password"]').setValue('password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-testid="login-error"]').exists()).toBe(false)
  })

  it('shows the explicit cold-start service error', async () => {
    useAuthStore(pinia).setError('服务暂未就绪')
    const { wrapper } = await mountLogin()
    expect(wrapper.get('[data-testid="bootstrap-error"]').text()).toContain('服务暂未就绪')
  })
})

async function mountLogin(initialPath = '/login') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', component: LoginPage },
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/secure', component: { template: '<div />' } },
    ],
  })
  await router.push(initialPath)
  await router.isReady()
  const wrapper = mount(LoginPage, {
    attachTo: document.body,
    global: { plugins: [ElementPlus, pinia, router, appI18n] },
  })
  await flushPromises()
  return { wrapper, router }
}
