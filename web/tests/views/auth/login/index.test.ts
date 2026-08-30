import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser, login } from '@src/api/auth/login'
import { appI18n, setLocale } from '@src/i18n'
import { pinia } from '@src/store'
import { useAuthStore } from '@src/store/auth'
import { ApiError } from '@src/types/http'
import LoginPage from '@src/views/auth/login/index.vue'

vi.mock('@src/api/auth/login', () => ({ login: vi.fn(), getCurrentUser: vi.fn() }))

const loginMock = vi.mocked(login)
const getCurrentUserMock = vi.mocked(getCurrentUser)

describe('Login page', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    useAuthStore(pinia).$reset()
    loginMock.mockReset()
    getCurrentUserMock.mockReset()
  })

  it('requires email and password', async () => {
    const { wrapper } = await mountLogin()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(loginMock).not.toHaveBeenCalled()
  })

  it('renders the product identity and the existing login form', async () => {
    const { wrapper } = await mountLogin()
    expect(wrapper.get('[data-testid="login-brand"]').text()).toContain('Admin')
    expect(wrapper.find('[data-testid="login-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-email"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-password"]').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'ElRow' }).exists()).toBe(true)
    expect(wrapper.findAllComponents({ name: 'ElCol' })).toHaveLength(2)
    expect(wrapper.find('[data-testid="login-locale-switch"]').exists()).toBe(false)
  })

  it('renders the selected locale messages', async () => {
    setLocale('en-US')
    const { wrapper } = await mountLogin()
    expect(wrapper.get('[data-testid="login-email"]').attributes('placeholder')).toBe('Enter email address')
    expect(wrapper.get('[data-testid="login-submit"]').text()).toBe('Sign in to console')
  })

  it('updates validation messages when the locale changes', async () => {
    const { wrapper } = await mountLogin()
    const form = wrapper.findComponent({ name: 'ElForm' })
    const initialRules = form.props('rules') as { email: Array<{ message: string }> }
    expect(initialRules.email[0]?.message).toBe('请输入邮箱')

    setLocale('en-US')
    await wrapper.vm.$nextTick()

    const englishRules = form.props('rules') as { email: Array<{ message: string }> }
    expect(englishRules.email[0]?.message).toBe('Email is required')
  })

  it('reads the remembered locale before authentication', async () => {
    setLocale('en-US')
    const { wrapper } = await mountLogin()

    expect(wrapper.text()).toContain('Welcome back')
    expect(wrapper.get('[data-testid="login-email"]').attributes('placeholder')).toBe('Enter email address')
  })

  it('submits trimmed email and original password, loads me, and follows a safe redirect', async () => {
    const order: string[] = []
    loginMock.mockImplementation(async () => {
      order.push('login')
      return { accessToken: 'jwt', expiresIn: 900 }
    })
    getCurrentUserMock.mockImplementation(async () => {
      order.push('me')
      return { userId: 1, username: 'admin', email: 'admin@example.com', phone: null }
    })
    const { wrapper, router } = await mountLogin('/login?redirect=/secure')
    await wrapper.get('[data-testid="login-email"]').setValue(' Admin@Example.COM ')
    await wrapper.get('[data-testid="login-password"]').setValue('  password  ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(loginMock).toHaveBeenCalledWith({ email: 'Admin@Example.COM', password: '  password  ' })
    expect(wrapper.find('a[href="/register"]').exists()).toBe(false)
    expect(order).toEqual(['login', 'me'])
    expect(useAuthStore(pinia).status).toBe('authenticated')
    expect(router.currentRoute.value.path).toBe('/secure')
  })

  it('locks submit while pending and shows one credential error', async () => {
    let rejectLogin: (error: Error) => void = () => undefined
    loginMock.mockImplementation(() => new Promise((_, reject) => { rejectLogin = reject }))
    const { wrapper } = await mountLogin()
    await wrapper.get('[data-testid="login-email"]').setValue('admin@example.com')
    await wrapper.get('[data-testid="login-password"]').setValue('wrong')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="login-submit"]').attributes('disabled')).toBeDefined()

    rejectLogin(new ApiError(10002, '未登录或登录已失效', 401))
    await flushPromises()
    expect(wrapper.get('[data-testid="login-error"]').text()).toContain('邮箱或密码错误')
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
  const wrapper = mount(LoginPage, { attachTo: document.body, global: { plugins: [ElementPlus, pinia, router, appI18n] } })
  await flushPromises()
  return { wrapper, router }
}
