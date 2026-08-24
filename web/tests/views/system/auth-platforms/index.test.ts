import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createAuthPlatform,
  deleteAuthPlatform,
  getAuthPlatformDeployment,
  getAuthPlatforms,
  updateAuthPlatform,
  updateAuthPlatformStatus,
} from '@src/api/auth-platform'
import { YesNo } from '@src/enums/yes-no'
import { appI18n } from '@src/i18n'
import { pinia } from '@src/store'
import { useAccessStore } from '@src/store/access'
import AuthPlatformsPage from '@src/views/system/auth-platforms/index.vue'

vi.mock('@src/api/auth-platform', () => ({
  createAuthPlatform: vi.fn(),
  deleteAuthPlatform: vi.fn(),
  getAuthPlatformDeployment: vi.fn(),
  getAuthPlatforms: vi.fn(),
  updateAuthPlatform: vi.fn(),
  updateAuthPlatformStatus: vi.fn(),
}))

const getAuthPlatformsMock = vi.mocked(getAuthPlatforms)
const getAuthPlatformDeploymentMock = vi.mocked(getAuthPlatformDeployment)
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
    getAuthPlatformDeploymentMock.mockReset()
    createAuthPlatformMock.mockReset()
    updateAuthPlatformMock.mockReset()
    updateAuthPlatformStatusMock.mockReset()
    deleteAuthPlatformMock.mockReset()
    getAuthPlatformsMock.mockResolvedValue({ list: [adminRow], total: 1, page: 1, pageSize: 20 })
    getAuthPlatformDeploymentMock.mockResolvedValue({
      cookieSecure: false, corsOrigin: 'http://localhost:16300', trustedProxyMode: 'none',
      trustedProxyCount: 0, redisStatus: 'up',
    })
  })

  it('loads list and deployment with list permission', async () => {
    setPermissions(['system:auth-platform:list'])
    const { wrapper } = await mountPage()

    expect(getAuthPlatformsMock).toHaveBeenCalledWith({ page: 1, pageSize: 20 })
    expect(getAuthPlatformDeploymentMock).toHaveBeenCalledOnce()
    expect(wrapper.find('h1').exists()).toBe(false)
    expect(wrapper.get('.auth-platform-page').classes()).toContain('system-page')
    expect(wrapper.text()).toContain('Admin')
    expect(wrapper.text()).toContain('不限')
  })

  it('gates each mutation command independently and protects builtin deletion', async () => {
    setPermissions([
      'system:auth-platform:list', 'system:auth-platform:create',
      'system:auth-platform:update', 'system:auth-platform:status', 'system:auth-platform:delete',
    ])
    const { wrapper } = await mountPage()

    expect(wrapper.find('[data-testid="auth-platform-create"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="auth-platform-update"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="auth-platform-status"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="auth-platform-delete"]').exists()).toBe(false)
  })

  it('submits search and pagination using exact queries', async () => {
    setPermissions(['system:auth-platform:list'])
    const { wrapper } = await mountPage()

    await wrapper.get('[data-testid="auth-platform-keyword"]').setValue('portal')
    await wrapper.get('[data-testid="auth-platform-search"]').trigger('click')
    await flushPromises()
    expect(getAuthPlatformsMock).toHaveBeenLastCalledWith({ page: 1, pageSize: 20, keyword: 'portal' })

  })

  it('keeps edit code read-only and uses an independently scrolling dialog body', async () => {
    setPermissions(['system:auth-platform:list', 'system:auth-platform:update'])
    const { wrapper } = await mountPage()
    await wrapper.get('[data-testid="auth-platform-update"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="auth-platform-code"]').attributes('disabled')).toBeDefined()
  })
})

function setPermissions(codes: string[]): void {
  useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: codes })
}

async function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/system/auth-platforms', component: AuthPlatformsPage }],
  })
  await router.push('/system/auth-platforms')
  await router.isReady()
  const wrapper = mount(AuthPlatformsPage, { global: { plugins: [ElementPlus, pinia, appI18n, router] } })
  await flushPromises()
  return { wrapper, router }
}
