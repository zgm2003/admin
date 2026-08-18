import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser, login } from '../../../api/auth'
import { pinia } from '../../../store'
import { useAuthStore } from '../../../store/auth'
import { ApiError } from '../../../types/http'
import LoginPage from './index.vue'

vi.mock('../../../api/auth', () => ({ login: vi.fn(), getCurrentUser: vi.fn() }))

const loginMock = vi.mocked(login)
const getCurrentUserMock = vi.mocked(getCurrentUser)

describe('Login page', () => {
  beforeEach(() => {
    useAuthStore(pinia).$reset()
    loginMock.mockReset()
    getCurrentUserMock.mockReset()
  })

  it('requires username and password', async () => {
    const { wrapper } = await mountLogin()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(loginMock).not.toHaveBeenCalled()
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
  const wrapper = mount(LoginPage, { global: { plugins: [ElementPlus, pinia, router] } })
  return { wrapper, router }
}
