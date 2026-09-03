import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n, setLocale } from '@/i18n'
import AppAside from '@/layout/components/AppAside/index.vue'
import { usePermissionStore } from '@/store/permission'
import { requestObjectURL } from '@/api/storage/upload'

vi.mock('@/api/storage/upload', () => ({ requestObjectURL: vi.fn() }))

const requestObjectURLMock = vi.mocked(requestObjectURL)

describe('AppAside profile access', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    requestObjectURLMock.mockReset()
  })

  it('renders the resolved avatar image when the current user has an avatar key', async () => {
    requestObjectURLMock.mockResolvedValue({ url: 'https://cdn.example.com/avatar.png' })
    const { wrapper } = mountAside([], { avatar: 'avatar/profile.png' })

    await flushPromises()

    expect(requestObjectURLMock).toHaveBeenCalledWith('avatar', 'avatar/profile.png')
    expect(wrapper.findComponent({ name: 'ElAvatar' }).exists()).toBe(true)
    expect(wrapper.get('.app-aside__avatar img').attributes('src')).toBe(
      'https://cdn.example.com/avatar.png',
    )
  })

  it('shows the profile entry only with profile-view permission and always keeps logout', async () => {
    const { access, wrapper } = mountAside([])
    expect(document.body.querySelector('[data-testid="aside-account-profile"]')).toBeNull()
    expect(document.body.querySelector('[data-testid="aside-account-logout"]')).not.toBeNull()

    access.applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: ['account:profile:view'] })
    await wrapper.vm.$nextTick()
    expect(document.body.querySelector('[data-testid="aside-account-profile"]')).not.toBeNull()
    expect(document.body.querySelector('[data-testid="aside-account-logout"]')).not.toBeNull()
  })

  it('opens the dynamic profile URL without depending on its generated route name', async () => {
    const { router, wrapper } = mountAside(['account:profile:view'])
    wrapper.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'profile')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/account/profile')
  })
})

function mountAside(permissionCodes: string[], props: { avatar?: string } = {}) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const access = usePermissionStore(pinia)
  access.applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/account/profile', component: { template: '<div />' } },
    ],
  })
  const wrapper = mount(AppAside, {
    props: { collapsed: false, uniqueOpened: true, ...props },
    global: { plugins: [ElementPlus, pinia, router, appI18n] },
  })
  return { access, router, wrapper }
}
