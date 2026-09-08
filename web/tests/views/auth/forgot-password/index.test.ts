import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { forgotPassword, resetPassword } from '@/api/auth/login'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { useAuthStore } from '@/store/auth'
import ForgotPasswordPage from '@/views/auth/forgot-password/index.vue'

vi.mock('@/api/auth/login', () => ({
  forgotPassword: vi.fn(),
  resetPassword: vi.fn(),
}))

const forgotPasswordMock = vi.mocked(forgotPassword)
const resetPasswordMock = vi.mocked(resetPassword)

const sendCodeResult = {
  challengeId: 'challenge-1',
  expiresAt: '2026-09-08T10:05:00Z',
  resendAfterSeconds: 60,
}

describe('Forgot password page', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    useAuthStore(pinia).$reset()
    forgotPasswordMock.mockReset()
    resetPasswordMock.mockReset()
  })

  it('renders the email step first and hides the reset fields', async () => {
    const { wrapper } = await mountPage()
    expect(wrapper.find('[data-testid="forgot-email"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="forgot-code"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="forgot-new-password"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="forgot-submit"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="forgot-back-login"]').attributes('href')).toBe('/login')
  })

  it('requires an email before sending the code', async () => {
    const { wrapper } = await mountPage()
    await wrapper.get('[data-testid="forgot-send-code"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="forgot-error"]').text()).toContain('请输入邮箱')
    expect(forgotPasswordMock).not.toHaveBeenCalled()
  })

  it('sends the code, reveals the reset fields, and counts down the resend window', async () => {
    forgotPasswordMock.mockResolvedValue(sendCodeResult)
    const { wrapper } = await mountPage()
    await wrapper.get('[data-testid="forgot-email"]').setValue('admin@example.com')
    await wrapper.get('[data-testid="forgot-send-code"]').trigger('click')
    await flushPromises()

    expect(forgotPasswordMock).toHaveBeenCalledWith('admin@example.com')
    expect(wrapper.find('[data-testid="forgot-code"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="forgot-new-password"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="forgot-confirm-password"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="forgot-send-code"]').text()).toContain('60')
  })

  it('rejects mismatching confirmation locally before calling the API', async () => {
    forgotPasswordMock.mockResolvedValue(sendCodeResult)
    const { wrapper } = await mountPage()
    await wrapper.get('[data-testid="forgot-email"]').setValue('admin@example.com')
    await wrapper.get('[data-testid="forgot-send-code"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="forgot-code"]').setValue('123456')
    await wrapper.get('[data-testid="forgot-new-password"]').setValue('NewPassw0rd!')
    await wrapper.get('[data-testid="forgot-confirm-password"]').setValue('DifferentPass1!')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(resetPasswordMock).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="forgot-error"]').text()).toContain('两次输入的密码不一致')
  })

  it('resets the password and returns to the login page with the account prefilled', async () => {
    forgotPasswordMock.mockResolvedValue(sendCodeResult)
    resetPasswordMock.mockResolvedValue(undefined)
    const { wrapper, router } = await mountPage()
    await wrapper.get('[data-testid="forgot-email"]').setValue('admin@example.com')
    await wrapper.get('[data-testid="forgot-send-code"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="forgot-code"]').setValue('123456')
    await wrapper.get('[data-testid="forgot-new-password"]').setValue('NewPassw0rd!')
    await wrapper.get('[data-testid="forgot-confirm-password"]').setValue('NewPassw0rd!')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(resetPasswordMock).toHaveBeenCalledWith({
      email: 'admin@example.com',
      code: '123456',
      newPassword: 'NewPassw0rd!',
      confirmPassword: 'NewPassw0rd!',
    })
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.account).toBe('admin@example.com')
  })

  it('prefills the email from the query string', async () => {
    const { wrapper } = await mountPage('/forgot-password?account=admin%40example.com')
    const emailInput = wrapper.get('[data-testid="forgot-email"]')
    expect((emailInput.element as HTMLInputElement).value).toBe('admin@example.com')
  })
})

async function mountPage(initialPath = '/forgot-password') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/forgot-password', component: ForgotPasswordPage },
      { path: '/login', component: { template: '<div />' } },
    ],
  })
  await router.push(initialPath)
  await router.isReady()
  const wrapper = mount(ForgotPasswordPage, {
    attachTo: document.body,
    global: { plugins: [ElementPlus, pinia, router, appI18n] },
  })
  await flushPromises()
  return { wrapper, router }
}
