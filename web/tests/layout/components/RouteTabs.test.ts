import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter, type RouteRecordRaw } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { PermissionMenuNode } from '@/api/permission/permission'
import { YesNo } from '@/enums/yesNo'
import { appI18n, setLocale } from '@/i18n'
import RouteTabs from '@/layout/components/RouteTabs/index.vue'

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
    path: '/user/account',
    name: 'account-users',
    component: views,
    meta: { requiresAuth: true, i18nKey: 'navigation.main' },
  },
  {
    path: '/permission/role',
    name: 'access-roles',
    component: views,
    meta: { requiresAuth: true, i18nKey: 'reports.orders.list' },
  },
  {
    path: '/system/operationLog',
    name: 'system-operation-logs',
    component: views,
    meta: { requiresAuth: true, i18nKey: 'navigation.main' },
  },
  {
    path: '/user/profile',
    name: 'account-profile',
    component: views,
    meta: { requiresAuth: true, i18nKey: 'navigation.main' },
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
    await router.push('/user/account')
    await flushPromises()
    await router.push('/user/account')
    await flushPromises()

    expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(2)
    expect(
      wrapper.get('[data-testid="route-tab"][data-path="/dashboard"]').attributes('data-affix'),
    ).toBe('true')
    expect(wrapper.find('[data-testid="route-tab-dashboard-close"]').exists()).toBe(false)
  })

  it('uses the complete access tree instead of dynamic route meta for titles', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/user/account')
    await flushPromises()
    expect(wrapper.get('[data-testid="route-tab"][data-path="/user/account"]').text()).toContain(
      '用户管理',
    )
    expect(wrapper.text()).not.toContain('主导航')

    await router.push('/system/operationLog')
    await flushPromises()
    expect(
      wrapper.get('[data-testid="route-tab"][data-path="/system/operationLog"]').text(),
    ).toContain('操作日志')
  })

  it('uses the hidden profile access node metadata without a route special case', async () => {
    const tree = accessTree()
    const account = tree[0]
    if (account === undefined) throw new Error('missing account fixture')
    account.children.push(
      page('user:profile:list', '/user/profile', 'user/profile', 'layout.user.profile'),
    )
    account.children[1].isHidden = YesNo.Yes
    const { wrapper, router } = await mountTabs('/dashboard', tree)
    await router.push('/user/profile')
    await flushPromises()
    expect(wrapper.get('[data-testid="route-tab"][data-path="/user/profile"]').text()).toContain(
      '个人中心',
    )
  })

  it('renders an unknown access-tree i18n key instead of inventing a title', async () => {
    const tree = accessTree()
    const accessRoot = tree[1]
    if (accessRoot === undefined || accessRoot.children[0] === undefined)
      throw new Error('missing access fixture')
    accessRoot.children[0].i18nKey = 'reports.orders.list'
    const { wrapper, router } = await mountTabs('/dashboard', tree)
    await router.push('/permission/role')
    await flushPromises()
    expect(wrapper.get('[data-testid="route-tab"][data-path="/permission/role"]').text()).toContain(
      'reports.orders.list',
    )
  })

  it('closes the active tab and selects the nearest remaining tab', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/user/account')
    await router.push('/permission/role')
    await flushPromises()
    await wrapper.get('[data-testid="route-tab-permission-role-close"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/user/account')
  })

  it('close others and close all retain Dashboard', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/user/account')
    await router.push('/permission/role')
    await flushPromises()
    await wrapper
      .get('[data-testid="route-tab"][data-path="/permission/role"]')
      .trigger('contextmenu')
    await wrapper.get('[data-testid="route-tabs-close-others-context"]').trigger('click')
    expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(2)

    await wrapper
      .get('[data-testid="route-tab"][data-path="/permission/role"]')
      .trigger('contextmenu')
    await wrapper.get('[data-testid="route-tabs-close-all-context"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(wrapper.findAll('[data-testid="route-tab"]')).toHaveLength(1)
  })

  it('navigates with previous and next controls and exposes disabled ends', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await router.push('/user/account')
    await router.push('/permission/role')
    await flushPromises()

    expect(wrapper.get('[data-testid="route-tabs-next"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="route-tabs-previous"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/user/account')
    await wrapper.get('[data-testid="route-tabs-next"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/permission/role')
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

  it('shows a direct exit action while content fullscreen is active', async () => {
    const { wrapper } = await mountTabs('/dashboard')

    expect(wrapper.find('[data-testid="exit-content-fullscreen"]').exists()).toBe(false)

    await wrapper.setProps({ fullscreen: true })
    await wrapper.get('[data-testid="exit-content-fullscreen"]').trigger('click')

    expect(wrapper.emitted('toggleFullscreen')).toHaveLength(1)
  })

  it('dismisses context menu and scrolls the active tab into view', async () => {
    const { wrapper, router } = await mountTabs('/dashboard')
    await wrapper.get('[data-testid="route-tab"][data-path="/dashboard"]').trigger('contextmenu')
    expect(wrapper.find('[role="menu"]').exists()).toBe(true)
    await wrapper.get('[data-testid="route-tab"][data-path="/dashboard"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)

    await router.push('/user/account')
    await flushPromises()
    expect(scrollIntoViewMock).toHaveBeenCalled()
    await wrapper.get('[data-testid="route-tab"][data-path="/user/account"]').trigger('contextmenu')
    await wrapper.get('[data-testid="route-tabs-close"]').trigger('click')
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
    await wrapper.get('[data-testid="route-tab"][data-path="/dashboard"]').trigger('contextmenu')
    await wrapper.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="menu"]').exists()).toBe(false)
  })
})

async function mountTabs(initialPath: string, menuTree: PermissionMenuNode[] = accessTree()) {
  const router = createRouter({ history: createMemoryHistory(), routes })
  await router.push(initialPath)
  await router.isReady()
  const wrapper = mount(RouteTabs, {
    props: { menuTree },
    global: { plugins: [ElementPlus, appI18n, router] },
  })
  await flushPromises()
  return { wrapper, router }
}

function accessTree(): PermissionMenuNode[] {
  return [
    directory(
      'account',
      'navigation.user',
      page('user:account:list', '/user/account', 'user/account', 'navigation.userAccount'),
    ),
    directory(
      'access',
      'navigation.permission',
      page(
        'permission:role:list',
        '/permission/role',
        'permission/role',
        'navigation.permissionRole',
      ),
    ),
    directory(
      'system',
      'navigation.system',
      page(
        'system:operationLog:list',
        '/system/operationLog',
        'system/operationLog',
        'navigation.systemOperationLog',
      ),
    ),
  ]
}

function directory(code: string, i18nKey: string, child: PermissionMenuNode): PermissionMenuNode {
  return {
    code,
    menuType: 'directory',
    path: null,
    componentPath: null,
    i18nKey,
    icon: null,
    isHidden: YesNo.No,
    children: [child],
  }
}

function page(
  code: string,
  path: string,
  componentPath: string,
  i18nKey: string,
): PermissionMenuNode {
  return {
    code,
    menuType: 'page',
    path,
    componentPath,
    i18nKey,
    icon: null,
    isHidden: YesNo.No,
    children: [],
  }
}

function getPopupItem(testId: string): HTMLElement {
  const items = Array.from(document.body.querySelectorAll<HTMLElement>(`[data-testid="${testId}"]`))
  const item = items.at(-1)
  if (item === undefined) throw new Error(`Missing dropdown item: ${testId}`)
  return item
}
