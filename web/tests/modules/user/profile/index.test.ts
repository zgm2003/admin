import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n, setLocale } from '@src/i18n'
import * as profileAPI from '@src/api/user/profile'
import ProfilePage from '@src/modules/user/profile/index.vue'
import { useAccessStore } from '@src/store/access'
import { useAuthStore } from '@src/store/auth'

vi.mock('@src/api/user/profile', () => ({
  getAccountProfile: vi.fn(),
  updateAccountProfile: vi.fn(),
  changePassword: vi.fn(),
}))

const getAccountProfile = vi.mocked(profileAPI.getAccountProfile)

describe('account profile permissions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    getAccountProfile.mockResolvedValue({ userId: 7, username: 'alice', email: 'alice@example.com', phone: null, birthday: null, gender: 0 })
  })

  it.each([
    { permissions: ['account:profile:update'], save: true, password: false },
    { permissions: ['account:password:update'], save: false, password: true },
    { permissions: [], save: false, password: false },
  ])('shows only actions granted by $permissions', async ({ permissions, save, password }) => {
    const wrapper = mountPage(permissions)
    await flushPromises()

    expect(wrapper.find('[data-testid="account-profile-save"]').exists()).toBe(save)
    expect(wrapper.find('[data-testid="account-password-submit"]').exists()).toBe(password)
  })
})

function mountPage(permissionCodes: string[]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  useAuthStore(pinia).setAuthenticated({ userId: 7, username: 'alice', email: 'alice@example.com', phone: null })
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/login', name: 'login', component: { template: '<div />' } }] })
  return mount(ProfilePage, { global: { plugins: [pinia, appI18n, ElementPlus, router] } })
}
