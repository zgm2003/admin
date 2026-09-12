import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { forgotPassword, getCaptcha, resetPassword } from '@/api/auth/login'
import { appI18n, setLocale } from '@/i18n'
import { pinia } from '@/store'
import { useAuthStore } from '@/store/auth'
import ForgotPasswordPage from '@/views/auth/forgotPassword/index.vue'
import CaptchaDialog from '@/views/auth/components/CaptchaDialog/index.vue'

vi.mock('@/api/auth/login', () => ({
  forgotPassword: vi.fn(),
  resetPassword: vi.fn(),
  getCaptcha: vi.fn(),
}))

const forgotPasswordMock = vi.mocked(forgotPassword)
const resetPasswordMock = vi.mocked(resetPassword)
const getCaptchaMock = vi.mocked(getCaptcha)

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
    getCaptchaMock.mockReset()
    getCaptchaMock.mockResolvedValue({
      captchaId: 'captcha-1', captchaType: 'slide', masterImage: 'data:image/png;base64,master',
      tileImage: 'data:image/png;base64,tile', tileX: 80, tileY: 40, tileWidth: 48,
      tileHeight: 48, imageWidth: 300, imageHeight: 220, expiresIn: 120,
    })
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
    await completeCaptcha(wrapper)

    expect(forgotPasswordMock).toHaveBeenCalledWith('admin@example.com', 'email', expect.objectContaining({ captchaId: 'captcha-1' }))
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
    await completeCaptcha(wrapper)

    wrapper.findComponent({ name: 'ElInputOtp' }).vm.$emit('update:modelValue', '123456')
    await wrapper.vm.$nextTick()
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
    await completeCaptcha(wrapper)

    wrapper.findComponent({ name: 'ElInputOtp' }).vm.$emit('update:modelValue', '123456')
    await wrapper.vm.$nextTick()
    await wrapper.get('[data-testid="forgot-new-password"]').setValue('NewPassw0rd!')
    await wrapper.get('[data-testid="forgot-confirm-password"]').setValue('NewPassw0rd!')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(resetPasswordMock).toHaveBeenCalledWith({
      account: 'admin@example.com',
      loginType: 'email',
      challengeId: 'challenge-1',
      code: '123456',
      newPassword: 'NewPassw0rd!',
      confirmPassword: 'NewPassw0rd!',
    })
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.account).toBe('admin@example.com')
  })

  it('prefills the email from the query string', async () => {
    const { wrapper } = await mountPage('/forgotPassword?account=admin%40example.com')
    const emailInput = wrapper.get('[data-testid="forgot-email"]')
    expect((emailInput.element as HTMLInputElement).value).toBe('admin@example.com')
  })

  it('clears the previous identity proof when switching recovery channel', async () => {
    forgotPasswordMock.mockResolvedValue(sendCodeResult)
    const { wrapper } = await mountPage()
    await wrapper.get('[data-testid="forgot-email"]').setValue('admin@example.com')
    await wrapper.get('[data-testid="forgot-send-code"]').trigger('click')
    await flushPromises()
    await completeCaptcha(wrapper)
    expect(wrapper.find('[data-testid="forgot-code"]').exists()).toBe(true)

    await wrapper.findComponent({ name: 'ElRadioGroup' }).vm.$emit('update:modelValue', 'phone')
    await flushPromises()

    expect(wrapper.find('[data-testid="forgot-code"]').exists()).toBe(false)
    expect((wrapper.get('[data-testid="forgot-email"]').element as HTMLInputElement).value).toBe('')
    expect(wrapper.get('[data-testid="forgot-email"]').attributes('placeholder')).toContain('手机号')
  })
})

async function mountPage(initialPath = '/forgotPassword') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/forgotPassword', component: ForgotPasswordPage },
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

async function completeCaptcha(wrapper: VueWrapper): Promise<void> {
  wrapper.findComponent(CaptchaDialog).vm.$emit('complete', { captchaId: 'captcha-1', captchaAnswer: { x: 80, y: 40 } })
  await flushPromises()
}
