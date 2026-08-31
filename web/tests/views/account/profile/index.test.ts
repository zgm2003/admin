import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus, { ElMessage } from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n, setLocale } from '@src/i18n'
import * as profileAPI from '@src/api/user/profile'
import ProfilePage from '@src/views/account/profile/index.vue'
import UpMedia from '@src/components/UpMedia/src/index.vue'
import { useAccessStore } from '@src/store/access'
import { useAuthStore } from '@src/store/auth'

vi.mock('@src/api/user/profile', () => ({
  getAccountProfile: vi.fn(),
  updateAccountProfile: vi.fn(),
  changePassword: vi.fn(),
}))

const getAccountProfile = vi.mocked(profileAPI.getAccountProfile)
const updateAccountProfile = vi.mocked(profileAPI.updateAccountProfile)
const changePassword = vi.mocked(profileAPI.changePassword)

describe('account profile permissions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    getAccountProfile.mockResolvedValue({ userId: 7, username: 'alice', email: 'alice@example.com', phone: null, avatar: '', birthday: null, gender: 0 })
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

  it('does not emit a second error toast when saving the profile fails', async () => {
    updateAccountProfile.mockRejectedValue(new Error('保存失败'))
    const errorSpy = vi.spyOn(ElMessage, 'error')
    const wrapper = mountPage(['account:profile:update'])
    await flushPromises()

    await wrapper.get('[data-testid="account-profile-save"]').trigger('click')
    await flushPromises()

    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('loads the avatar object key and submits it with profile changes', async () => {
    getAccountProfile.mockResolvedValue({ userId: 7, username: 'alice', email: 'alice@example.com', phone: null, avatar: 'avatar/old.png', birthday: null, gender: 0 })
    updateAccountProfile.mockResolvedValue({ userId: 7, username: 'alice', email: 'alice@example.com', phone: null, avatar: 'avatar/new.png', birthday: null, gender: 0, updatedAt: '2026-08-30T00:00:00Z' })
    const wrapper = mountPage(['account:profile:update'])
    await flushPromises()

    const media = wrapper.findComponent(UpMedia)
    expect(media.props('modelValue')).toBe('avatar/old.png')
    media.vm.$emit('update:modelValue', 'avatar/new.png')
    await wrapper.get('[data-testid="account-profile-save"]').trigger('click')
    await flushPromises()

    expect(updateAccountProfile).toHaveBeenCalledWith(expect.objectContaining({ avatar: 'avatar/new.png' }))
    expect(useAuthStore().user?.avatar).toBe('avatar/new.png')
  })

  it('does not emit a second error toast when changing password fails', async () => {
    changePassword.mockRejectedValue(new Error('密码错误'))
    const errorSpy = vi.spyOn(ElMessage, 'error')
    const wrapper = mountPage(['account:password:update'])
    await flushPromises()

    await wrapper.get('[data-testid="account-password-submit"]').trigger('click')
    await flushPromises()

    expect(errorSpy).not.toHaveBeenCalled()
  })
})

function mountPage(permissionCodes: string[]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  useAuthStore(pinia).setAuthenticated({ userId: 7, username: 'alice', email: 'alice@example.com', phone: null, avatar: '' })
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/login', name: 'login', component: { template: '<div />' } }] })
  return mount(ProfilePage, { global: { plugins: [pinia, appI18n, ElementPlus, router] } })
}
