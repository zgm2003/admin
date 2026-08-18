import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { register } from '../../../api/auth'
import { pinia } from '../../../store'
import { useAuthStore } from '../../../store/auth'
import RegisterPage from './index.vue'

vi.mock('../../../api/auth', () => ({ register: vi.fn() }))

const registerMock = vi.mocked(register)

describe('Register page', () => {
  beforeEach(() => {
    useAuthStore(pinia).$reset()
    registerMock.mockReset()
  })

  it('requires all fields and exact password confirmation', async () => {
    const { wrapper } = await mountRegister()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(registerMock).not.toHaveBeenCalled()

    await fillRegister(wrapper, 'password', 'different')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(registerMock).not.toHaveBeenCalled()
  })

  it('submits exact input and returns to Login without authenticating', async () => {
    registerMock.mockResolvedValue({ userId: 1, username: 'admin', email: 'admin@example.com' })
    const { wrapper, router } = await mountRegister()
    await fillRegister(wrapper, 'password', 'password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(registerMock).toHaveBeenCalledWith({
      username: 'admin', email: 'admin@example.com', password: 'password', confirmPassword: 'password',
    })
    expect(router.currentRoute.value.path).toBe('/login')
    expect(useAuthStore(pinia).accessToken).toBe('')
  })

  it('locks submit and displays a registration conflict', async () => {
    let rejectRegistration: (error: Error) => void = () => undefined
    registerMock.mockImplementation(() => new Promise((_, reject) => { rejectRegistration = reject }))
    const { wrapper } = await mountRegister()
    await fillRegister(wrapper, 'password', 'password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="register-submit"]').attributes('disabled')).toBeDefined()

    rejectRegistration(new Error('用户名已存在'))
    await flushPromises()
    expect(wrapper.get('[data-testid="register-error"]').text()).toContain('用户名已存在')
  })
})

async function mountRegister() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', component: { template: '<div />' } },
      { path: '/register', component: RegisterPage },
    ],
  })
  await router.push('/register')
  await router.isReady()
  const wrapper = mount(RegisterPage, { global: { plugins: [ElementPlus, pinia, router] } })
  return { wrapper, router }
}

async function fillRegister(wrapper: ReturnType<typeof mount>, password: string, confirmPassword: string) {
  await wrapper.get('[data-testid="register-username"]').setValue('admin')
  await wrapper.get('[data-testid="register-email"]').setValue('admin@example.com')
  await wrapper.get('[data-testid="register-password"]').setValue(password)
  await wrapper.get('[data-testid="register-confirm-password"]').setValue(confirmPassword)
}
