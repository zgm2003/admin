import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getAuthPolicy, getCurrentUser, login } from '@src/api/auth'
import { appI18n, setLocale } from '@src/i18n'
import { pinia } from '@src/store'
import { useAuthStore } from '@src/store/auth'
import { ApiError } from '@src/types/http'
import LoginPage from '@src/views/auth/login/index.vue'

vi.mock('@src/api/auth', () => ({ getAuthPolicy: vi.fn(), login: vi.fn(), getCurrentUser: vi.fn() }))

const loginMock = vi.mocked(login)
const getCurrentUserMock = vi.mocked(getCurrentUser)
const getAuthPolicyMock = vi.mocked(getAuthPolicy)

describe('Login page', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    useAuthStore(pinia).$reset()
    loginMock.mockReset()
    getCurrentUserMock.mockReset()
    getAuthPolicyMock.mockReset()
    getAuthPolicyMock.mockResolvedValue({ code: 'admin', name: 'Admin', allowRegister: 0 })
  })

  it('requires username and password', async () => {
    const { wrapper } = await mountLogin()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(loginMock).not.toHaveBeenCalled()
  })

  it('hides the registration entry when policy disables registration', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.find('a[href="/register"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('注册新账号')
  })

  it('shows a registration entry when policy enables registration', async () => {
    getAuthPolicyMock.mockResolvedValue({ code: 'admin', name: 'Admin', allowRegister: 1 })
    const { wrapper } = await mountLogin()

    expect(wrapper.find('a[href="/register"]').exists()).toBe(true)
  })

  it('shows an explicit policy failure instead of treating registration as disabled', async () => {
    getAuthPolicyMock.mockRejectedValue(new ApiError(10006, '服务暂未就绪', 503))
    const { wrapper } = await mountLogin()

    expect(wrapper.get('[data-testid="policy-error"]').text()).toContain('服务暂未就绪')
    expect(wrapper.find('a[href="/register"]').exists()).toBe(false)
  })

  it('renders the product identity and the existing login form', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.get('[data-testid="login-brand"]').text()).toContain('Admin')
    expect(wrapper.find('[data-testid="login-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-username"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-password"]').exists()).toBe(true)
  })

  it('renders the selected locale messages', async () => {
    setLocale('en-US')
    const { wrapper } = await mountLogin()
    expect(wrapper.get('[data-testid="login-username"]').attributes('placeholder')).toBe('Enter username')
    expect(wrapper.get('[data-testid="login-submit"]').text()).toBe('Sign in to console')
  })

  it('submits exact credentials, loads me, and follows a safe redirect', async () => {
    const order: string[] = []
    loginMock.mockImplementation(async () => {
      order.push('login')
      return { accessToken: 'jwt', expiresIn: 900 }
    })
    getCurrentUserMock.mockImplementation(async () => {
      order.push('me')
      return { userId: 1, username: 'admin', email: 'admin@example.com' }
    })
    const { wrapper, router } = await mountLogin('/login?redirect=/secure')
    await wrapper.get('[data-testid="login-username"]').setValue('admin')
    await wrapper.get('[data-testid="login-password"]').setValue('password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(loginMock).toHaveBeenCalledWith({ username: 'admin', password: 'password' })
    expect(order).toEqual(['login', 'me'])
    expect(useAuthStore(pinia).status).toBe('authenticated')
    expect(router.currentRoute.value.path).toBe('/secure')
  })

  it('locks submit while pending and shows one credential error', async () => {
    let rejectLogin: (error: Error) => void = () => undefined
    loginMock.mockImplementation(() => new Promise((_, reject) => { rejectLogin = reject }))
    const { wrapper } = await mountLogin()
    await wrapper.get('[data-testid="login-username"]').setValue('admin')
    await wrapper.get('[data-testid="login-password"]').setValue('wrong')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="login-submit"]').attributes('disabled')).toBeDefined()

    rejectLogin(new ApiError(10002, '未登录或登录已失效', 401))
    await flushPromises()
    expect(wrapper.get('[data-testid="login-error"]').text()).toContain('用户名或密码错误')
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
      { path: '/register', component: { template: '<div />' } },
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/secure', component: { template: '<div />' } },
    ],
  })
  await router.push(initialPath)
  await router.isReady()
  const wrapper = mount(LoginPage, { global: { plugins: [ElementPlus, pinia, router, appI18n] } })
  await flushPromises()
  return { wrapper, router }
}
