import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n, setLocale } from '../../i18n'
import RouteTabs from './RouteTabs.vue'

const views = {
  template: '<div data-testid="route-view" />',
}
const scrollIntoViewMock = vi.fn()

const routes: RouteRecordRaw[] = [
  {
    path: '/dashboard',
    name: 'dashboard',
    component: views,
    meta: { requiresAuth: true, titleKey: 'navigation.dashboard', affix: true },
  },
  {
    path: '/users',
    name: 'users',
    component: views,
    meta: { requiresAuth: true, titleKey: 'navigation.main' },
  },
  {
    path: '/roles',
    name: 'roles',
    component: views,
    meta: { requiresAuth: true, titleKey: 'layout.routeTabs.contextMenu' },
  },
]

describe('RouteTabs', () => {
  beforeEach(() => {
    setLocale('zh-CN')
    scrollIntoViewMock.mockReset()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoViewMock,
    })
  })

  it('adds each visited leaf once and keeps Dashboard fixed', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/users')
    await flushPromises()
    await router.push('/users')
    await flushPromises()

    expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(2)
    expect(wrapper.get('[data-testid="route-tab-dashboard"]').attributes('data-affix')).toBe('true')
    expect(wrapper.find('[data-testid="route-tab-dashboard-close"]').exists()).toBe(false)
  })

  it('closes the active tab and selects the nearest remaining tab', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/users')
    await router.push('/roles')
    await flushPromises()
    await wrapper.get('[data-testid="route-tab-roles-close"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/users')
  })

  it('close others and close all retain Dashboard', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/users')
    await router.push('/roles')
    await flushPromises()
    await wrapper.get('[data-testid="route-tab-roles-menu"]').trigger('contextmenu')
    await wrapper.get('[data-testid="route-tabs-close-others-context"]').trigger('click')
    expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(2)

    await wrapper.get('[data-testid="route-tab-roles-menu"]').trigger('contextmenu')
    await wrapper.get('[data-testid="route-tabs-close-all-context"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(1)
  })

  it('navigates with previous and next controls and exposes disabled ends', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/users')
    await router.push('/roles')
    await flushPromises()

    expect(wrapper.get('[data-testid="route-tabs-next"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="route-tabs-previous"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/users')
    await wrapper.get('[data-testid="route-tabs-next"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/roles')
  })

  it('emits refresh and fullscreen commands', async () => {
    const { wrapper } = await mountTabs('/dashboard')
    await wrapper.get('[data-testid="route-tabs-refresh"]').trigger('click')
    await wrapper.get('[data-testid="route-tabs-fullscreen"]').trigger('click')

    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('toggleFullscreen')).toHaveLength(1)
  })

  it('dismisses context menu and scrolls the active tab into view', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await wrapper.get('[data-testid="route-tab-dashboard-menu"]').trigger('contextmenu')
    expect(wrapper.find('[role="menu"]').exists()).toBe(true)
    await wrapper.get('[data-testid="route-tab-dashboard"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)

    await router.push('/users')
    await flushPromises()
    expect(scrollIntoViewMock).toHaveBeenCalled()
    await wrapper.get('[data-testid="route-tab-users"]').trigger('contextmenu')
    await wrapper.get('[data-testid="route-tabs-close"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    await wrapper.get('[data-testid="route-tab-dashboard-menu"]').trigger('contextmenu')
    await wrapper.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
  })
})

async function mountTabs(initialPath: string) {
  const router = createRouter({ history: createMemoryHistory(), routes })
  await router.push(initialPath)
  await router.isReady()
  const wrapper = mount(RouteTabs, {
    global: { plugins: [ElementPlus, appI18n, router] },
  })
  await flushPromises()
  return { wrapper, router }
}
