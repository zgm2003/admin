import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { appI18n, setLocale } from '@src/i18n'
import RouteTabs from '@src/layout/components/RouteTabs.vue'

const views = {
  template: '<div data-testid="route-view" />',
}
const scrollIntoViewMock = vi.fn()

const routes: RouteRecordRaw[] = [
  {
    path: '/dashboard',
    name: 'dashboard',
    component: views,
		meta: { requiresAuth: true, i18nKey: 'navigation.dashboard', affix: true },
  },
  {
    path: '/users',
    name: 'users',
    component: views,
		meta: { requiresAuth: true, i18nKey: 'navigation.main' },
  },
  {
    path: '/roles',
    name: 'roles',
    component: views,
		meta: { requiresAuth: true, i18nKey: 'reports.orders.list' },
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

  it('keeps the TabTag composition with navigation around the scroll pane and one settings action', async () => {
    const { wrapper } = await mountTabs('/dashboard')

    expect(wrapper.find('.route-tabs__previous').exists()).toBe(true)
    expect(wrapper.find('.route-tabs__scroll').exists()).toBe(true)
    expect(wrapper.find('.route-tabs__next').exists()).toBe(true)
    expect(wrapper.find('.route-tabs__actions').exists()).toBe(true)
    expect(wrapper.find('[data-testid="route-tabs-settings"]').exists()).toBe(true)
    expect(wrapper.findAll('.route-tabs__menu-trigger')).toHaveLength(0)
  })

	it('adds each visited leaf once and keeps Dashboard fixed', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/users')
    await flushPromises()
    await router.push('/users')
    await flushPromises()

    expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(2)
    expect(wrapper.get('[data-testid="route-tab"][data-path="/dashboard"]').attributes('data-affix')).toBe('true')
    expect(wrapper.find('[data-testid="route-tab-dashboard-close"]').exists()).toBe(false)
	})

	it('renders an unknown dynamic i18n key instead of inventing a title', async () => {
		const { wrapper, router } = await mountTabs('/dashboard')
		await router.push('/roles')
		await flushPromises()
		expect(wrapper.get('[data-testid="route-tab"][data-path="/roles"]').text()).toContain('reports.orders.list')
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
    await wrapper.get('[data-testid="route-tab"][data-path="/roles"]').trigger('contextmenu')
    await wrapper.get('[data-testid="route-tabs-close-others-context"]').trigger('click')
    expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(2)

    await wrapper.get('[data-testid="route-tab"][data-path="/roles"]').trigger('contextmenu')
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
    await wrapper.get('[data-testid="route-tabs-settings"]').trigger('click')
    await flushPromises()
    getPopupItem('route-tabs-refresh').click()
    await flushPromises()
    await wrapper.get('[data-testid="route-tabs-settings"]').trigger('click')
    await flushPromises()
    getPopupItem('route-tabs-fullscreen').click()
    await flushPromises()

    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('toggleFullscreen')).toHaveLength(1)
  })

  it('dismisses context menu and scrolls the active tab into view', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await wrapper.get('[data-testid="route-tab"][data-path="/dashboard"]').trigger('contextmenu')
    expect(wrapper.find('[role="menu"]').exists()).toBe(true)
    await wrapper.get('[data-testid="route-tab"][data-path="/dashboard"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)

    await router.push('/users')
    await flushPromises()
    expect(scrollIntoViewMock).toHaveBeenCalled()
    await wrapper.get('[data-testid="route-tab"][data-path="/users"]').trigger('contextmenu')
    await wrapper.get('[data-testid="route-tabs-close"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    await wrapper.get('[data-testid="route-tab"][data-path="/dashboard"]').trigger('contextmenu')
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

function getPopupItem(testId: string): HTMLElement {
  const items = Array.from(document.body.querySelectorAll<HTMLElement>(`[data-testid="${testId}"]`))
  const item = items.at(-1)
  if (item === undefined) throw new Error(`Missing dropdown item: ${testId}`)
  return item
}
