import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus, { ElMessage } from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n, setLocale } from '@/i18n'
import * as profileAPI from '@/api/user/profile'
import ProfilePage from '@/views/user/profile/index.vue'
import UpMedia from '@/components/UpMedia/index.vue'
import ProfileHero from '@/views/user/profile/components/ProfileHero/index.vue'
import { usePermissionStore } from '@/store/permission'
import { useAuthStore } from '@/store/auth'

vi.mock('@/api/user/profile', () => ({
  getAccountProfile: vi.fn(),
  updateAccountProfile: vi.fn(),
  changePassword: vi.fn(),
  setPassword: vi.fn(),
}))

const getAccountProfile = vi.mocked(profileAPI.getAccountProfile)
const updateAccountProfile = vi.mocked(profileAPI.updateAccountProfile)
const changePassword = vi.mocked(profileAPI.changePassword)
const setPassword = vi.mocked(profileAPI.setPassword)

describe('account profile permissions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    getAccountProfile.mockResolvedValue({
      userId: 7,
      username: 'alice',
      email: 'alice@example.com',
      phone: null,
      avatar: '',
      birthday: null,
      gender: 0,
    })
  })

  it.each([
    { permissions: ['user:profile:update'], save: true, password: false },
    { permissions: ['user:password:update'], save: false, password: true },
    { permissions: [], save: false, password: false },
  ])('shows only actions granted by $permissions', async ({ permissions, save, password }) => {
    const wrapper = mountPage(permissions)
    await flushPromises()

    expect(wrapper.find('[data-testid="account-profile-save"]').exists()).toBe(save)
    expect(wrapper.find('[data-testid="account-password-submit"]').exists()).toBe(password)
  })

  it('uses the virtualized select with explicit gender options', async () => {
    const wrapper = mountPage([])
    await flushPromises()

    const genderSelect = wrapper.getComponent({ name: 'ElSelectV2' })
    expect(genderSelect.attributes('data-testid')).toBe('account-profile-gender')
    expect(genderSelect.props('options')).toEqual([
      { label: '未知', value: 0 },
      { label: '男', value: 1 },
      { label: '女', value: 2 },
    ])
  })

  it('does not emit a second error toast when saving the profile fails', async () => {
    updateAccountProfile.mockRejectedValue(new Error('保存失败'))
    const errorSpy = vi.spyOn(ElMessage, 'error')
    const wrapper = mountPage(['user:profile:update'])
    await flushPromises()

    await wrapper.get('[data-testid="account-profile-save"]').trigger('click')
    await flushPromises()

    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('loads the avatar object key and submits it with profile changes', async () => {
    getAccountProfile.mockResolvedValue({
      userId: 7,
      username: 'alice',
      email: 'alice@example.com',
      phone: null,
      avatar: 'avatar/old.png',
      birthday: null,
      gender: 0,
    })
    updateAccountProfile.mockResolvedValue({
      userId: 7,
      username: 'alice',
      email: 'alice@example.com',
      phone: null,
      avatar: 'avatar/new.png',
      birthday: null,
      gender: 0,
      updatedAt: '2026-08-30T00:00:00Z',
    })
    const wrapper = mountPage(['user:profile:update'])
    await flushPromises()

    const media = wrapper.findComponent(UpMedia)
    expect(media.props('modelValue')).toBe('avatar/old.png')
    media.vm.$emit('preview-change', 'https://cdn.example/avatar/old.png')
    await flushPromises()
    expect(wrapper.getComponent(ProfileHero).props('avatarUrl')).toBe(
      'https://cdn.example/avatar/old.png',
    )
    media.vm.$emit('update:modelValue', 'avatar/new.png')
    await wrapper.get('[data-testid="account-profile-save"]').trigger('click')
    await flushPromises()

    expect(updateAccountProfile).toHaveBeenCalledWith(
      expect.objectContaining({ avatar: 'avatar/new.png' }),
    )
    expect(useAuthStore().user?.avatar).toBe('avatar/new.png')
  })

  it('does not emit a second error toast when changing password fails', async () => {
    changePassword.mockRejectedValue(new Error('密码错误'))
    const errorSpy = vi.spyOn(ElMessage, 'error')
    const wrapper = mountPage(['user:password:update'])
    await flushPromises()

    await wrapper.get('[data-testid="account-password-submit"]').trigger('click')
    await flushPromises()

    expect(errorSpy).not.toHaveBeenCalled()
  })

  it('sets the first password without asking for the current one', async () => {
    setPassword.mockResolvedValue(undefined)
    const wrapper = mountPage(['user:password:update'], true)
    await flushPromises()

    expect(wrapper.text()).toContain('设置密码')
    expect(wrapper.find('[data-testid="account-password-current"]').exists()).toBe(false)

    await wrapper.get('[data-testid="account-password-new"]').setValue('NewPassw0rd!')
    await wrapper.get('[data-testid="account-password-confirm"]').setValue('NewPassw0rd!')
    await wrapper.get('[data-testid="account-password-submit"]').trigger('click')
    await flushPromises()

    expect(setPassword).toHaveBeenCalledWith({
      newPassword: 'NewPassw0rd!',
      confirmPassword: 'NewPassw0rd!',
    })
    expect(changePassword).not.toHaveBeenCalled()
    expect(useAuthStore().passwordSetRequired).toBe(false)
  })
})

function mountPage(permissionCodes: string[], passwordSetRequired = false) {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  useAuthStore(pinia).setAuthenticated({
    userId: 7,
    username: 'alice',
    email: 'alice@example.com',
    phone: null,
    avatar: '',
    passwordSetRequired,
  })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/login', name: 'login', component: { template: '<div />' } }],
  })
  return mount(ProfilePage, { global: { plugins: [pinia, appI18n, ElementPlus, router] } })
}
