import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it } from 'vitest'

import { appI18n, setLocale } from '@src/i18n'
import AppAside from '@src/layout/components/AppAside.vue'
import { useAccessStore } from '@src/store/access'

describe('AppAside profile access', () => {
  beforeEach(() => setLocale('zh-CN'))

  it('shows the profile entry only with profile-list permission and always keeps logout', async () => {
    const { access, wrapper } = mountAside([])
    expect(document.body.querySelector('[data-testid="aside-account-profile"]')).toBeNull()
    expect(document.body.querySelector('[data-testid="aside-account-logout"]')).not.toBeNull()

    access.applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: ['account:profile:list'] })
    await wrapper.vm.$nextTick()
    expect(document.body.querySelector('[data-testid="aside-account-profile"]')).not.toBeNull()
    expect(document.body.querySelector('[data-testid="aside-account-logout"]')).not.toBeNull()
  })

  it('opens the dynamic profile URL without depending on its generated route name', async () => {
    const { router, wrapper } = mountAside(['account:profile:list'])
    wrapper.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'profile')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/account/profile')
  })
})

function mountAside(permissionCodes: string[]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const access = useAccessStore(pinia)
  access.applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/account/profile', component: { template: '<div />' } },
    ],
  })
  const wrapper = mount(AppAside, {
    props: { collapsed: false, uniqueOpened: true },
    global: { plugins: [ElementPlus, pinia, router, appI18n] },
  })
  return { access, router, wrapper }
}
