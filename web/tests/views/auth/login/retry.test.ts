import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { RouterView, createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser, getLoginConfig, refresh } from '@/api/auth/login'
import { getPermission } from '@/api/permission/permission'
import { appI18n, setLocale } from '@/i18n'
import { installPermissionGuard } from '@/permission'
import { pinia } from '@/store'
import { useAuthStore } from '@/store/auth'
import { usePermissionStore } from '@/store/permission'
import { ApiError, ProtocolError } from '@/types/http'
import LoginPage from '@/views/auth/login/index.vue'

vi.mock('@/api/auth/login', () => ({
  getCurrentUser: vi.fn(),
  getLoginConfig: vi.fn(),
  refresh: vi.fn(),
  login: vi.fn(),
  sendLoginCode: vi.fn(),
  getCaptcha: vi.fn(),
}))

vi.mock('@/api/permission/permission', () => ({ getPermission: vi.fn() }))

const config = {
  loginTypes: [{ value: 'password' as const, label: '密码' }],
  allowRegister: false,
}
const credential = {
  accessToken: 'restored-token',
  expiresIn: 900,
  isNewUser: false,
  passwordSetRequired: false,
}
const unavailable = () => new ApiError(10006, '认证服务暂不可用', 503)
const wrappers: VueWrapper[] = []

describe('Login initialization recovery', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    setLocale('zh-CN')
    useAuthStore(pinia).$reset()
    usePermissionStore(pinia).reset()
    vi.mocked(refresh)
      .mockRejectedValueOnce(unavailable())
      .mockImplementation(async () => {
        // The real refresh request publishes the returned credential to Auth Store.
        useAuthStore(pinia).setCredential(credential)
        return credential
      })
    vi.mocked(getCurrentUser).mockResolvedValue({
      userId: 1,
      username: 'admin',
      email: 'admin@example.com',
      phone: null,
      avatar: '',
      passwordSetRequired: false,
    })
    vi.mocked(getLoginConfig).mockRejectedValueOnce(unavailable()).mockResolvedValue(config)
    vi.mocked(getPermission).mockResolvedValue({ roleCodes: [], permissionCodes: [], menuTree: [] })
  })

  afterEach(() => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
    document.body.innerHTML = ''
  })

  it('retries the failed session bootstrap and returns to the original protected URL', async () => {
    const { wrapper, router } = await mountColdLogin()
    expect(wrapper.get('[data-testid="bootstrap-error"]').text()).toBe('认证服务暂不可用')
    expect(wrapper.find('[data-testid="login-account"]').exists()).toBe(false)

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(useAuthStore(pinia).status).toBe('authenticated')
    expect(useAuthStore(pinia).errorMessage).toBe('')
    expect(useAuthStore(pinia).accessToken).toBe('restored-token')
    expect(usePermissionStore(pinia).status).toBe('ready')
    expect(router.currentRoute.value.fullPath).toBe('/secure?tab=profile#section')
    expect(wrapper.find('[data-testid="bootstrap-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="protected-page"]').exists()).toBe(true)
  })

  it('clears the stale bootstrap error only after the server confirms an expired session', async () => {
    vi.mocked(refresh).mockRejectedValueOnce(new ApiError(10002, '登录已失效', 401))
    const { wrapper, router } = await mountColdLogin()

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toBe('/login?redirect=/secure?tab=profile%23section')
    expect(useAuthStore(pinia).status).toBe('anonymous')
    expect(useAuthStore(pinia).errorMessage).toBe('')
    expect(wrapper.find('[data-testid="bootstrap-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="login-config-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="login-account"]').exists()).toBe(true)
    expect(getCurrentUser).not.toHaveBeenCalled()
    expect(getPermission).not.toHaveBeenCalled()
  })

  it('offers a retry when only the authentication bootstrap failed', async () => {
    vi.mocked(getLoginConfig).mockReset().mockResolvedValue(config)
    const { wrapper, router } = await mountColdLogin()
    expect(wrapper.find('[data-testid="login-account"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-config-error"]').exists()).toBe(false)

    expect(wrapper.find('[data-testid="login-config-retry"]').exists()).toBe(true)
    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/secure')
    expect(useAuthStore(pinia).status).toBe('authenticated')
  })

  it('does not confuse successful config loading with successful authentication', async () => {
    vi.mocked(refresh).mockRejectedValueOnce(new ApiError(10006, '认证依赖仍未恢复', 503))
    const { wrapper, router } = await mountColdLogin()

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(useAuthStore(pinia).status).toBe('error')
    expect(wrapper.get('[data-testid="bootstrap-error"]').text()).toBe('认证依赖仍未恢复')
    expect(wrapper.find('[data-testid="login-config-error"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="login-config-retry"]').exists()).toBe(true)
    expect(router.currentRoute.value.name).toBe('login')

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/secure')
  })

  it('coalesces repeated clicks and locks the login form while restoring a session', async () => {
    let complete: (() => void) | undefined
    vi.mocked(refresh).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          complete = () => {
            useAuthStore(pinia).setCredential(credential)
            resolve(credential)
          }
        }),
    )
    vi.mocked(getLoginConfig).mockReset().mockResolvedValue(config)
    const { wrapper } = await mountColdLogin()
    expect(wrapper.find('[data-testid="login-config-retry"]').exists()).toBe(true)

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')

    expect(refresh).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="login-config-retry"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="login-submit"]').exists()).toBe(false)
    if (complete === undefined) throw new Error('Session recovery was not started')
    complete()
    await flushPromises()
    expect(wrapper.find('[data-testid="protected-page"]').exists()).toBe(true)
  })

  it('keeps retry progress visible while the current user is still being verified', async () => {
    const user = {
      userId: 1,
      username: 'admin',
      email: 'admin@example.com',
      phone: null,
      avatar: '',
      passwordSetRequired: false,
    }
    let complete: (() => void) | undefined
    vi.mocked(getCurrentUser).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          complete = () => resolve(user)
        }),
    )
    vi.mocked(getLoginConfig).mockReset().mockResolvedValue(config)
    const { wrapper } = await mountColdLogin()

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(useAuthStore(pinia).accessToken).toBe('restored-token')
    expect(wrapper.find('[data-testid="login-config-retry"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="login-config-retry"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-testid="login-submit"]').exists()).toBe(false)
    if (complete === undefined) throw new Error('Current-user verification was not started')
    complete()
    await flushPromises()
    expect(wrapper.find('[data-testid="protected-page"]').exists()).toBe(true)
  })

  it('preserves the selected allowed login method and draft after a session expires', async () => {
    vi.mocked(getLoginConfig)
      .mockReset()
      .mockResolvedValue({
        loginTypes: [
          { value: 'email', label: '邮箱验证码' },
          { value: 'password', label: '密码' },
        ],
        allowRegister: false,
      })
    vi.mocked(refresh).mockRejectedValueOnce(new ApiError(10002, '登录已失效', 401))
    const { wrapper } = await mountColdLogin()
    await wrapper.get('[data-testid="login-account"]').setValue('draft@example.com')
    await wrapper.findComponent({ name: 'ElTabs' }).vm.$emit('update:modelValue', 'password')
    await flushPromises()
    await wrapper.get('[data-testid="login-password"]').setValue('draft-password')

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="login-password"]').exists()).toBe(true)
    expect((wrapper.get('[data-testid="login-password"]').element as HTMLInputElement).value).toBe(
      'draft-password',
    )
    expect((wrapper.get('[data-testid="login-account"]').element as HTMLInputElement).value).toBe(
      'draft@example.com',
    )
  })

  it.each(['/login', '/forgotPassword', '//outside.example/path'])(
    'does not mistake redirect %s for a protected session check',
    async (redirect) => {
      const { wrapper, router } = await mountColdLogin()
      await router.replace({ name: 'login', query: { redirect } })

      await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
      await flushPromises()

      expect(useAuthStore(pinia).status).toBe('authenticated')
      expect(router.currentRoute.value.path).toBe('/dashboard')
    },
  )

  it('preserves an allowed code-login draft across an intermediate config failure', async () => {
    const supportedConfig = {
      loginTypes: [
        { value: 'password' as const, label: '密码' },
        { value: 'email' as const, label: '邮箱验证码' },
      ],
      allowRegister: false,
    }
    vi.mocked(getLoginConfig)
      .mockReset()
      .mockResolvedValueOnce(supportedConfig)
      .mockRejectedValueOnce(unavailable())
      .mockResolvedValue(supportedConfig)
    vi.mocked(refresh).mockRejectedValueOnce(new ApiError(10002, '登录已失效', 401))
    const { wrapper } = await mountColdLogin()
    await wrapper.findComponent({ name: 'ElTabs' }).vm.$emit('update:modelValue', 'email')
    await flushPromises()
    await wrapper.findComponent({ name: 'ElInputOtp' }).vm.$emit('update:modelValue', '123456')

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="login-config-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="login-submit"]').exists()).toBe(false)
    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="login-code"]').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'ElInputOtp' }).props('modelValue')).toBe('123456')
    expect(useAuthStore(pinia).status).toBe('anonymous')
  })

  it('retains an actionable error when navigation fails after authentication recovers', async () => {
    vi.mocked(getLoginConfig).mockReset().mockResolvedValue(config)
    const { wrapper, router } = await mountColdLogin()
    router.onError(() => undefined)
    const removeGuard = router.beforeResolve((to) => {
      if (to.path === '/secure') throw new ProtocolError('private route details')
    })

    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="bootstrap-error"]').text()).not.toContain(
      'private route details',
    )
    expect(wrapper.find('[data-testid="login-config-retry"]').exists()).toBe(true)
    expect(useAuthStore(pinia).status).toBe('authenticated')
    expect(wrapper.find('[data-testid="login-submit"]').exists()).toBe(false)
    removeGuard()
    await wrapper.get('[data-testid="login-config-retry"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/secure')
  })
})

async function mountColdLogin() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', name: 'login', component: LoginPage, meta: { requiresAuth: false } },
      {
        path: '/forgotPassword',
        component: { template: '<div />' },
        meta: { requiresAuth: false },
      },
      {
        path: '/',
        name: 'admin-layout',
        component: { template: '<router-view />' },
        meta: { requiresAuth: true },
        children: [
          {
            path: 'dashboard',
            name: 'dashboard',
            component: { template: '<div />' },
            meta: { requiresAuth: true },
          },
          {
            path: 'secure',
            component: { template: '<div data-testid="protected-page" />' },
            meta: { requiresAuth: true },
          },
        ],
      },
    ],
  })
  installPermissionGuard(router)
  await router.push('/secure?tab=profile#section')
  await router.isReady()
  const wrapper = mount(RouterView, {
    attachTo: document.body,
    global: { plugins: [ElementPlus, pinia, router, appI18n] },
  })
  wrappers.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}
