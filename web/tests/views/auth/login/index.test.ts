import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus, { ElNotification } from 'element-plus'
import { isVNode } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCaptcha, getCurrentUser, getLoginConfig, login, sendLoginCode } from '@/api/auth/login'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { useAuthStore } from '@/store/auth'
import { ApiError } from '@/types/http'
import LoginPage from '@/views/auth/login/index.vue'
import CaptchaDialog from '@/views/auth/components/CaptchaDialog/index.vue'

vi.mock('@/api/auth/login', () => ({
  login: vi.fn(),
  getCurrentUser: vi.fn(),
  getLoginConfig: vi.fn(),
  sendLoginCode: vi.fn(),
  getCaptcha: vi.fn(),
}))

const loginMock = vi.mocked(login)
const getCurrentUserMock = vi.mocked(getCurrentUser)
const getLoginConfigMock = vi.mocked(getLoginConfig)
const sendLoginCodeMock = vi.mocked(sendLoginCode)
const getCaptchaMock = vi.mocked(getCaptcha)

describe('Login page', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    useAuthStore(pinia).$reset()
    loginMock.mockReset()
    getCurrentUserMock.mockReset()
    getLoginConfigMock.mockReset()
    sendLoginCodeMock.mockReset()
    getCaptchaMock.mockReset()
    getCaptchaMock.mockResolvedValue({
      captchaId: 'captcha-1', captchaType: 'slide', masterImage: 'data:image/png;base64,master',
      tileImage: 'data:image/png;base64,tile', tileX: 80, tileY: 40, tileWidth: 48,
      tileHeight: 48, imageWidth: 300, imageHeight: 220, expiresIn: 120,
    })
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
    expect(wrapper.get('[data-testid="login-brand"]').text()).toContain('智澜')
    expect(wrapper.find('[data-testid="login-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-account"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-password"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-register-hint"]').exists()).toBe(false)
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
    expect(wrapper.get('[data-testid="login-register-hint"]').text()).toContain(
      '自动注册为普通用户',
    )
  })

  it('submits a password login, loads me, and follows a safe redirect', async () => {
    const successSpy = vi.spyOn(ElNotification, 'success')
    const order: string[] = []
    loginMock.mockImplementation(async () => {
      order.push('login')
      return { accessToken: 'jwt', expiresIn: 900, isNewUser: false, passwordSetRequired: false }
    })
    getCurrentUserMock.mockImplementation(async () => {
      order.push('me')
      return {
        userId: 1,
        username: 'admin',
        email: 'admin@example.com',
        phone: null,
        avatar: '',
        passwordSetRequired: false,
      }
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
    expect(successSpy).toHaveBeenCalledWith({ title: '登录成功' })
    successSpy.mockRestore()
  })

  it('links a newly registered user to Personal center from the success notification', async () => {
    const successSpy = vi
      .spyOn(ElNotification, 'success')
      .mockImplementation(() => ({ close: () => undefined }))
    getLoginConfigMock.mockResolvedValue({
      loginTypes: [{ value: 'email', label: '邮箱验证码' }],
      allowRegister: true,
    })
    loginMock.mockResolvedValue({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: true,
      passwordSetRequired: true,
    })
    getCurrentUserMock.mockResolvedValue({
      userId: 2,
      username: 'new-user',
      email: 'new@example.com',
      phone: null,
      avatar: '',
      passwordSetRequired: true,
    })

    const { wrapper, router } = await mountLogin()
    await wrapper.get('[data-testid="login-account"]').setValue('new@example.com')
    await wrapper.findComponent({ name: 'ElInputOtp' }).vm.$emit('update:modelValue', '123456')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    const options = successSpy.mock.calls[0]?.[0]
    expect(options).toEqual(expect.objectContaining({ title: '注册并登录成功', duration: 8000 }))
    if (
      options === undefined ||
      typeof options === 'string' ||
      isVNode(options) ||
      !('message' in options) ||
      !isVNode(options.message)
    ) {
      throw new Error('Expected registration notification content to be a VNode')
    }
    const message = options.message
    const content = mount({ render: () => message }, { global: { plugins: [ElementPlus] } })
    expect(content.text()).toContain('已创建普通用户账号。')
    expect(content.get('a').text()).toBe('前往个人中心')
    expect(content.get('a').attributes('href')).toBe('/user/profile')

    await content.get('a').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/user/profile')
    content.unmount()
    successSpy.mockRestore()
  })

  it('switches to email code mode and sends a code', async () => {
    sendLoginCodeMock.mockResolvedValue({
      challengeId: 'challenge-1',
      expiresAt: '2026-09-07T10:10:00Z',
      resendAfterSeconds: 60,
    })
    const { wrapper } = await mountLogin()
    await wrapper.find('[data-testid="login-account"]').setValue('admin@example.com')
    // Switch to email code mode via the segmented control.
    const segmented = wrapper.findComponent({ name: 'ElTabs' })
    await segmented.vm.$emit('update:modelValue', 'email')
    await flushPromises()

    expect(wrapper.find('[data-testid="login-code"]').exists()).toBe(true)
    await wrapper.get('[data-testid="login-send-code"]').trigger('click')
    await flushPromises()
    await completeCaptcha(wrapper)
    expect(sendLoginCodeMock).toHaveBeenCalledWith(
      'admin@example.com',
      'email',
      'login',
      expect.any(String),
      expect.objectContaining({ captchaId: 'captcha-1' }),
    )
  })

  it('submits the challenge returned by the last successful delivery', async () => {
    sendLoginCodeMock.mockResolvedValue({
      challengeId: 'challenge-proof',
      expiresAt: '2026-09-07T10:10:00Z',
      resendAfterSeconds: 60,
    })
    loginMock.mockResolvedValue({
      accessToken: 'jwt',
      expiresIn: 900,
      isNewUser: false,
      passwordSetRequired: false,
    })
    getCurrentUserMock.mockResolvedValue({
      userId: 1,
      username: 'admin',
      email: 'admin@example.com',
      phone: null,
      avatar: '',
      passwordSetRequired: false,
    })
    const { wrapper } = await mountLogin()
    await wrapper.get('[data-testid="login-account"]').setValue('admin@example.com')
    await wrapper.findComponent({ name: 'ElTabs' }).vm.$emit('update:modelValue', 'email')
    await flushPromises()
    await wrapper.get('[data-testid="login-send-code"]').trigger('click')
    await flushPromises()
    await completeCaptcha(wrapper)
    await wrapper.findComponent({ name: 'ElInputOtp' }).vm.$emit('update:modelValue', '123456')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(loginMock).toHaveBeenCalledWith({
      loginType: 'email',
      loginAccount: 'admin@example.com',
      challengeId: 'challenge-proof',
      code: '123456',
    })
  })

  it('counts down from resendAfterSeconds, not the code expiry', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-07T10:00:00Z'))
    try {
      sendLoginCodeMock.mockResolvedValue({
        challengeId: 'challenge-1',
        expiresAt: '2026-09-07T10:05:00Z',
        resendAfterSeconds: 60,
      })
      const { wrapper } = await mountLogin()
      await wrapper.get('[data-testid="login-account"]').setValue('admin@example.com')
      await wrapper.findComponent({ name: 'ElTabs' }).vm.$emit('update:modelValue', 'email')
      await flushPromises()

      await wrapper.get('[data-testid="login-send-code"]').trigger('click')
      await flushPromises()
      await completeCaptcha(wrapper)

      const buttonText = wrapper.get('[data-testid="login-send-code"]').text()
      expect(buttonText).toContain('60')
      expect(buttonText).not.toContain('300')
    } finally {
      vi.useRealTimers()
    }
  })

  it('rotates the challenge after a successful delivery', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-07T10:00:00Z'))
    try {
      sendLoginCodeMock
        .mockResolvedValueOnce({
          challengeId: 'challenge-1',
          expiresAt: '2026-09-07T10:05:00Z',
          resendAfterSeconds: 1,
        })
        .mockResolvedValueOnce({
          challengeId: 'challenge-2',
          expiresAt: '2026-09-07T10:10:00Z',
          resendAfterSeconds: 1,
        })
      const { wrapper } = await mountLogin()
      await wrapper.get('[data-testid="login-account"]').setValue('admin@example.com')
      await wrapper.findComponent({ name: 'ElTabs' }).vm.$emit('update:modelValue', 'email')
      await flushPromises()

      await wrapper.get('[data-testid="login-send-code"]').trigger('click')
      await flushPromises()
      await completeCaptcha(wrapper)
      const firstChallenge = sendLoginCodeMock.mock.calls[0]?.[3]

      await vi.advanceTimersByTimeAsync(1000)
      await wrapper.get('[data-testid="login-send-code"]').trigger('click')
      await flushPromises()
      await completeCaptcha(wrapper)
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
        resendAfterSeconds: 60,
      })
    const { wrapper } = await mountLogin()
    await wrapper.get('[data-testid="login-account"]').setValue('admin@example.com')
    await wrapper.findComponent({ name: 'ElTabs' }).vm.$emit('update:modelValue', 'email')
    await flushPromises()

    await wrapper.get('[data-testid="login-send-code"]').trigger('click')
    await flushPromises()
    await completeCaptcha(wrapper)
    await wrapper.get('[data-testid="login-send-code"]').trigger('click')
    await flushPromises()
    await completeCaptcha(wrapper)

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

  it('links to forgot-password and prefills the account from the query', async () => {
    const { wrapper } = await mountLogin('/login?account=Admin%40Example.COM')
    expect(wrapper.get('[data-testid="login-forgot-link"]').attributes('href')).toBe(
      '/forgotPassword',
    )
    const accountInput = wrapper.get('[data-testid="login-account"]')
    expect((accountInput.element as HTMLInputElement).value).toBe('Admin@Example.COM')
  })
})

async function mountLogin(initialPath = '/login') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', component: LoginPage },
      { path: '/forgotPassword', component: { template: '<div />' } },
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/secure', component: { template: '<div />' } },
      { path: '/user/profile', component: { template: '<div />' } },
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

async function completeCaptcha(wrapper: VueWrapper): Promise<void> {
  wrapper.findComponent(CaptchaDialog).vm.$emit('complete', { captchaId: 'captcha-1', captchaAnswer: { x: 80, y: 40 } })
  await flushPromises()
}
