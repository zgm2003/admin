import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createAuthPlatform,
  deleteAuthPlatform,
  getAuthPlatforms,
  updateAuthPlatform,
  updateAuthPlatformStatus,
} from '@src/api/auth-platform'
import { YesNo } from '@src/enums/yes-no'
import { appI18n } from '@src/i18n'
import { pinia } from '@src/store'
import { useAccessStore } from '@src/store/access'
import AuthPlatformsPage from '@src/views/access/auth-platforms/index.vue'

vi.mock('@src/api/auth-platform', () => ({
  createAuthPlatform: vi.fn(),
  deleteAuthPlatform: vi.fn(),
  getAuthPlatforms: vi.fn(),
  updateAuthPlatform: vi.fn(),
  updateAuthPlatformStatus: vi.fn(),
}))

const getAuthPlatformsMock = vi.mocked(getAuthPlatforms)
const createAuthPlatformMock = vi.mocked(createAuthPlatform)
const updateAuthPlatformMock = vi.mocked(updateAuthPlatform)
const updateAuthPlatformStatusMock = vi.mocked(updateAuthPlatformStatus)
const deleteAuthPlatformMock = vi.mocked(deleteAuthPlatform)

const adminRow = {
  id: 2, code: 'admin', name: 'Admin', policyVersion: 1,
  accessTTLSeconds: 900, refreshTTLSeconds: 86_400, sessionCacheTTLSeconds: 7_200,
  accessCacheTTLSeconds: 600, bindDevice: YesNo.Yes, bindIP: YesNo.No, maxSessions: 0,
  allowRegister: YesNo.No, isEnabled: YesNo.Yes, isBuiltin: YesNo.Yes,
  createdAt: '2026-08-20T10:00:00Z', updatedAt: '2026-08-20T10:00:00Z',
}

describe('authentication platform page', () => {
  beforeEach(() => {
    const access = useAccessStore(pinia)
    access.reset()
    getAuthPlatformsMock.mockReset()
    createAuthPlatformMock.mockReset()
    updateAuthPlatformMock.mockReset()
    updateAuthPlatformStatusMock.mockReset()
    deleteAuthPlatformMock.mockReset()
    getAuthPlatformsMock.mockResolvedValue({ list: [adminRow], total: 1, page: 1, pageSize: 20 })
  })

  it('loads the platform list with list permission', async () => {
    setPermissions(['auth:platform:list'])
    const { wrapper } = await mountPage()

    expect(getAuthPlatformsMock).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
    expect(getAuthPlatformsMock).toHaveBeenCalledOnce()
    expect(wrapper.find('h1').exists()).toBe(false)
		expect(wrapper.get('.auth-platform-page').classes()).toContain('management-page')
    expect(wrapper.text()).toContain('Admin')
    expect(wrapper.text()).toContain('不限')
  })

  it('gates each mutation command independently and protects builtin deletion', async () => {
    setPermissions([
      'auth:platform:list', 'auth:platform:create',
      'auth:platform:update', 'auth:platform:status', 'auth:platform:delete',
    ])
    const { wrapper } = await mountPage()

    expect(wrapper.find('[data-testid="auth-platform-create"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="auth-platform-update"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="auth-platform-status"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="auth-platform-delete"]').exists()).toBe(false)
  })

  it('uses the old-project table density with explicit policy states and actions', async () => {
    setPermissions([
      'auth:platform:list', 'auth:platform:create',
      'auth:platform:update', 'auth:platform:status',
    ])
    const { wrapper } = await mountPage()

    expect(wrapper.find('.app-table__toolbar-left [data-testid="auth-platform-create"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="auth-platform-security"]').text()).toContain('绑定设备')
    expect(wrapper.get('[data-testid="auth-platform-security"]').text()).toContain('未绑定 IP')
    expect(wrapper.get('[data-testid="auth-platform-registration"]').text()).toContain('禁止注册')
    expect(wrapper.get('[data-testid="auth-platform-status"]').text()).toBe('停用')
    expect(wrapper.get('.auth-platform-identity').attributes('style')).toBeUndefined()
  })

  it('keeps table identity and TTL labels centered without wrapping', async () => {
    setPermissions(['auth:platform:list', 'auth:platform:update'])
    const { wrapper } = await mountPage()

    expect(wrapper.get('.auth-platform-identity').classes()).toContain('auth-platform-identity--centered')
    await wrapper.get('[data-testid="auth-platform-update"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('.auth-platform-field-label').classes()).toContain('auth-platform-field-label--nowrap')
  })

  it('keeps mobile policy switches in one row and distributes pagination ends', async () => {
    setPermissions(['auth:platform:list', 'auth:platform:update'])
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="auth-platform-update"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('.auth-platform-policy-grid').classes()).toContain('auth-platform-policy-grid--three-up')
    expect(wrapper.find('.app-table__pagination--distributed').exists()).toBe(true)
  })

  it('uses one scroll container and removes the unused deployment panel', async () => {
    setPermissions(['auth:platform:list'])
    const { wrapper } = await mountPage()

    expect(wrapper.find('.auth-platform-deployment-strip').exists()).toBe(false)
    expect(wrapper.find('.auth-platform-dialog-body').exists()).toBe(false)
    expect(wrapper.find('[data-testid="auth-platform-form-grid"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('2026-08-20T10:00:00Z')
  })

  it('submits search and pagination using exact queries', async () => {
    setPermissions(['auth:platform:list'])
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="auth-platform-keyword"]').setValue('portal')
    await wrapper.get('[data-testid="auth-platform-search"]').trigger('click')
    await flushPromises()
    expect(getAuthPlatformsMock).toHaveBeenLastCalledWith({ page: 1, pageSize: 20, keyword: 'portal' })

  })

  it('keeps edit code read-only and uses an independently scrolling dialog body', async () => {
    setPermissions(['auth:platform:list', 'auth:platform:update'])
    const { wrapper } = await mountPage()
    await wrapper.get('[data-testid="auth-platform-update"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('.app-dialog__body--scroll').exists()).toBe(false)
    expect(wrapper.get('[data-testid="auth-platform-form"]').classes()).toContain('auth-platform-form-scroll')
    expect(wrapper.get('[data-testid="auth-platform-form"] .auth-platform-form-grid').classes()).toContain('auth-platform-form-grid')
    expect(wrapper.get('[data-testid="auth-platform-code"]').attributes('disabled')).toBeDefined()
  })

  it('explains every TTL and restores the system defaults without submitting', async () => {
    setPermissions(['auth:platform:list', 'auth:platform:update'])
    const customizedRow = {
      ...adminRow,
      accessTTLSeconds: 1_800,
      refreshTTLSeconds: 1_209_600,
      sessionCacheTTLSeconds: 1_800,
      accessCacheTTLSeconds: 1_800,
    }
    getAuthPlatformsMock.mockResolvedValue({ list: [customizedRow], total: 1, page: 1, pageSize: 20 })
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="auth-platform-update"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('[data-testid="auth-platform-ttl-help"]')).toHaveLength(4)
    await wrapper.get('[data-testid="auth-platform-ttl-defaults"]').trigger('click')

    expect(ttlInputValue(wrapper, 'access')).toBe(900)
    expect(ttlInputValue(wrapper, 'refresh')).toBe(86_400)
    expect(ttlInputValue(wrapper, 'session-cache')).toBe(7_200)
    expect(ttlInputValue(wrapper, 'access-cache')).toBe(600)
    expect(updateAuthPlatformMock).not.toHaveBeenCalled()
  })

  it('locks builtin admin registration off while preserving non-builtin registration choices', async () => {
    setPermissions(['auth:platform:list', 'auth:platform:update'])
    updateAuthPlatformMock.mockResolvedValue({})
    const staleAdminRow = { ...adminRow, allowRegister: YesNo.Yes }
    getAuthPlatformsMock.mockResolvedValue({ list: [staleAdminRow], total: 1, page: 1, pageSize: 20 })
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="auth-platform-update"]').trigger('click')
    await flushPromises()
    const adminSwitch = wrapper.get('[data-testid="auth-platform-allow-register"]')
    expect(adminSwitch.get('input').attributes('disabled')).toBeDefined()
    expect(adminSwitch.get('input').attributes('aria-checked')).toBe('false')

    const adminSwitchComponent = findAllowRegisterSwitch(wrapper)
    await adminSwitchComponent.vm.$emit('update:modelValue', YesNo.Yes)
    await wrapper.get('.el-dialog__footer .el-button--primary').trigger('click')
    await flushPromises()
    expect(updateAuthPlatformMock).toHaveBeenCalledWith(2, expect.objectContaining({ allowRegister: YesNo.No }))

    wrapper.unmount()
    const appRow = { ...adminRow, id: 3, code: 'app', isBuiltin: YesNo.No, allowRegister: YesNo.Yes }
    getAuthPlatformsMock.mockResolvedValue({ list: [appRow], total: 1, page: 1, pageSize: 20 })
    const { wrapper: appWrapper } = await mountPage()

    await appWrapper.get('[data-testid="auth-platform-update"]').trigger('click')
    await flushPromises()
    const appSwitch = appWrapper.get('[data-testid="auth-platform-allow-register"]')
    expect(appSwitch.get('input').attributes('disabled')).toBeUndefined()

    const appSwitchComponent = findAllowRegisterSwitch(appWrapper)
    await appSwitchComponent.vm.$emit('update:modelValue', YesNo.No)
    await appWrapper.get('.el-dialog__footer .el-button--primary').trigger('click')
    await flushPromises()
    expect(updateAuthPlatformMock).toHaveBeenLastCalledWith(3, expect.objectContaining({ allowRegister: YesNo.No }))
  })

  it('keeps registration editable and submits the selected value for a new platform', async () => {
    setPermissions(['auth:platform:list', 'auth:platform:create'])
    createAuthPlatformMock.mockResolvedValue({ id: 3 })
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="auth-platform-create"]').trigger('click')
    await wrapper.get('[data-testid="auth-platform-code"]').setValue('portal')
    await wrapper.get('[data-testid="auth-platform-name"]').setValue('Portal')
    const allowRegisterSwitch = wrapper.get('[data-testid="auth-platform-allow-register"]')
    expect(allowRegisterSwitch.get('input').attributes('disabled')).toBeUndefined()

    const allowRegisterSwitchComponent = findAllowRegisterSwitch(wrapper)
    await allowRegisterSwitchComponent.vm.$emit('update:modelValue', YesNo.Yes)
    await wrapper.get('.el-dialog__footer .el-button--primary').trigger('click')
    await flushPromises()

    expect(createAuthPlatformMock).toHaveBeenCalledWith(expect.objectContaining({
      code: 'portal', name: 'Portal', allowRegister: YesNo.Yes,
    }))
  })
})

function setPermissions(codes: string[]): void {
  useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: codes })
}

async function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/access/auth-platforms', component: AuthPlatformsPage }],
  })
  await router.push('/access/auth-platforms')
  await router.isReady()
  const wrapper = mount(AuthPlatformsPage, { global: { plugins: [ElementPlus, pinia, appI18n, router] } })
  await flushPromises()
  return { wrapper, router }
}

function findAllowRegisterSwitch(wrapper: Awaited<ReturnType<typeof mountPage>>['wrapper']) {
  const allowRegisterSwitch = wrapper
    .findAllComponents({ name: 'ElSwitch' })
    .find((switchWrapper) => switchWrapper.attributes('data-testid') === 'auth-platform-allow-register')
  if (allowRegisterSwitch === undefined) throw new Error('allow registration switch is missing')
  return allowRegisterSwitch
}

function ttlInputValue(
  wrapper: Awaited<ReturnType<typeof mountPage>>['wrapper'],
  name: 'access' | 'refresh' | 'session-cache' | 'access-cache',
): number {
  const input = wrapper
    .get(`[data-testid="auth-platform-${name}-ttl"]`)
    .get('input').element as HTMLInputElement
  return Number(input.value)
}
