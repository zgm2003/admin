import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { logout } from '@src/api/auth'
import { appI18n, setLocale } from '@src/i18n'
import { pinia } from '@src/store'
import { useAccessStore } from '@src/store/access'
import { useAuthStore } from '@src/store/auth'
import { useUIPreferencesStore } from '@src/store/ui-preferences'
import Layout from '@src/layout/index.vue'

vi.mock('@src/api/auth', () => ({ logout: vi.fn() }))

const logoutMock = vi.mocked(logout)
let layoutRenderCount = 0

describe('admin layout', () => {
  beforeEach(() => {
    logoutMock.mockReset()
    logoutMock.mockResolvedValue()
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.style.removeProperty('color-scheme')
    setLocale('zh-CN')
    useAccessStore(pinia).reset()
    useAuthStore(pinia).$reset()
    useUIPreferencesStore(pinia).initializeSafely()
    useAuthStore(pinia).setCredential({ accessToken: 'jwt', expiresIn: 900 })
    useAuthStore(pinia).setAuthenticated({ userId: 1, username: 'admin', email: 'admin@example.com' })
    Object.defineProperty(window, 'innerWidth', { configurable: true, writable: true, value: 1200 })
  })

  it('renders one Aside, Header, Main, Footer, RouterView, username, and Dashboard item', async () => {
    const { wrapper } = await mountLayout()
    expect(wrapper.findAll('.admin-layout__aside')).toHaveLength(1)
    expect(wrapper.findAll('.admin-layout__header')).toHaveLength(1)
    expect(wrapper.findAll('.admin-layout__main')).toHaveLength(1)
    expect(wrapper.findAll('.admin-layout__footer')).toHaveLength(1)
    expect(wrapper.get('[data-testid="current-username"]').text()).toBe('admin')
    expect(wrapper.findAll('[data-testid="dashboard-menu-item"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="layout-content"]').text()).toContain('dashboard content')
  })

  it('collapses the desktop Aside without changing the shell tracks', async () => {
    const { wrapper } = await mountLayout()
    expect(wrapper.get('[data-testid="app-aside"]').attributes('data-collapsed')).toBe('false')
    await wrapper.get('[data-testid="toggle-menu"]').trigger('click')
    expect(wrapper.get('[data-testid="app-aside"]').attributes('data-collapsed')).toBe('true')
  })

  it('exposes the system settings entry in the Header', async () => {
    const { wrapper } = await mountLayout()
    expect(wrapper.find('[data-testid="open-settings"]').exists()).toBe(true)
  })

  it('switches the current interface language from the Header', async () => {
    const { wrapper } = await mountLayout()
    wrapper.findComponent({ name: 'ElDropdown' }).vm.$emit('command', 'en-US')
    await wrapper.vm.$nextTick()
    expect(document.documentElement.lang).toBe('en-US')
    expect(localStorage.getItem('admin:locale')).toBe('en-US')
    expect(wrapper.get('.app-header__breadcrumb').text()).toContain('Dashboard')
  })

  it('renders RouteTabs between Header and Main', async () => {
    const { wrapper } = await mountLayout()
    const order = wrapper.findAll('.admin-layout__workspace > *').map((node) => (
      node.classes().find((name) => name.startsWith('admin-layout__'))
    ))
    expect(order).toEqual([
      'admin-layout__header',
      'admin-layout__tabs',
      'admin-layout__main',
      'admin-layout__footer',
    ])
  })

  it('remounts the current view and hides outer chrome in fullscreen', async () => {
    const { wrapper } = await mountLayout()
    const before = wrapper.get('[data-testid="layout-content"]').attributes('data-render')
    await wrapper.get('[data-testid="route-tabs-settings"]').trigger('click')
    await flushPromises()
    getPopupItem('route-tabs-refresh').click()
    await flushPromises()
    expect(wrapper.get('[data-testid="layout-content"]').attributes('data-render')).not.toBe(before)

    await wrapper.get('[data-testid="route-tabs-settings"]').trigger('click')
    await flushPromises()
    getPopupItem('route-tabs-fullscreen').click()
    await flushPromises()
    expect(wrapper.find('.admin-layout__aside').exists()).toBe(false)
    expect(wrapper.find('.admin-layout__header').exists()).toBe(false)
    expect(wrapper.find('.admin-layout__footer').exists()).toBe(false)
    expect(wrapper.find('.admin-layout__tabs').exists()).toBe(true)
    expect(wrapper.find('.admin-layout__main').exists()).toBe(true)
  })

  it('exposes Main as the scroll owner and RouteTabs as horizontal scroll', async () => {
    const { wrapper } = await mountLayout()
    expect(wrapper.get('.admin-layout__main').classes()).toContain('admin-layout__scroll-owner')
    expect(wrapper.get('.admin-layout__tabs').classes()).toContain('admin-layout__horizontal-scroll')
  })

  it('renders directory and leaf breadcrumbs from the RBAC menu tree', async () => {
    useAccessStore(pinia).applySnapshot({
      roleCodes: [],
      menuTree: [{
        code: 'system',
        menuType: 'directory',
        path: null,
        viewKey: null,
        titleKey: 'navigation.system',
        icon: 'Folder',
        children: [{
          code: 'system:user:list',
          menuType: 'page',
          path: '/system/users',
          viewKey: 'system-users',
          titleKey: 'navigation.systemUsers',
          icon: 'User',
          children: [],
        }],
      }],
      permissionCodes: [],
    })
    const { wrapper } = await mountLayout('/system/users')

    expect(wrapper.get('.app-header__breadcrumb').text()).toContain('系统管理')
    expect(wrapper.get('.app-header__breadcrumb').text()).toContain('用户管理')
  })

  it('applies display preferences to the shell and prevents a stuck collapsed Aside', async () => {
    const { wrapper } = await mountLayout()
    const uiPreferences = useUIPreferencesStore(pinia)

    await wrapper.get('[data-testid="toggle-menu"]').trigger('click')
    uiPreferences.update({ showRouteTabs: false, showFooter: false, showBreadcrumb: false, showMenuToggle: false })
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.admin-layout__tabs').exists()).toBe(false)
    expect(wrapper.find('.admin-layout__footer').exists()).toBe(false)
    expect(wrapper.find('.app-header__breadcrumb').exists()).toBe(false)
    expect(wrapper.get('[data-testid="app-aside"]').attributes('data-collapsed')).toBe('false')
  })

  it('keeps RouteTabs mounted as the fullscreen exit path', async () => {
    const { wrapper } = await mountLayout()
    const uiPreferences = useUIPreferencesStore(pinia)
    wrapper.findComponent({ name: 'RouteTabs' }).vm.$emit('toggle-fullscreen')
    await wrapper.vm.$nextTick()
    uiPreferences.update({ showRouteTabs: false })
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.admin-layout__tabs').exists()).toBe(true)
  })

  it('shows visible preference and missing-breadcrumb errors', async () => {
    const uiPreferences = useUIPreferencesStore(pinia)
    localStorage.setItem('admin:ui-preferences', '{broken')
    uiPreferences.initializeSafely()
    const { wrapper } = await mountLayout('/system/missing')
    useAccessStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: [] })
    await wrapper.vm.$nextTick()

    expect(wrapper.find('[data-testid="preference-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="breadcrumb-missing"]').exists()).toBe(true)
  })

  it('opens a Drawer instead of collapsing on mobile', async () => {
    useAccessStore(pinia).applySnapshot({
      roleCodes: [],
      menuTree: [{
        code: 'system:user:list',
        menuType: 'page',
        path: '/system/users',
        viewKey: 'system-users',
			titleKey: 'navigation.systemMenus',
        icon: 'User',
        children: [],
      }],
      permissionCodes: [],
    })
    const { wrapper } = await mountLayout()
    window.innerWidth = 600
    window.dispatchEvent(new Event('resize'))
    await wrapper.vm.$nextTick()
    await wrapper.get('[data-testid="toggle-menu"]').trigger('click')
    const drawer = wrapper.findAllComponents({ name: 'ElDrawer' }).find((component) => component.props('size') === '264px')
    if (drawer === undefined) throw new Error('Missing mobile menu drawer')
    expect(drawer.props('modelValue')).toBe(true)
    const asides = wrapper.findAllComponents({ name: 'AppAside' })
    expect(asides).toHaveLength(2)
    expect(asides.every((aside) => aside.findAllComponents({ name: 'AccessMenuNode' }).length === 1)).toBe(true)
    expect(asides.every((aside) => aside.attributes('data-collapsed') === 'false')).toBe(true)
  })

  it('keeps RouterView mounted and shows one non-closable access error', async () => {
    useAccessStore(pinia).fail(new Error('权限快照加载失败'))
    const { wrapper } = await mountLayout()

    expect(wrapper.get('[data-testid="layout-content"]').text()).toContain('dashboard content')
    expect(wrapper.findAll('[data-testid="access-error"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="access-error"]').text()).toContain('加载访问权限失败')
    expect(wrapper.findComponent({ name: 'ElAlert' }).props('closable')).toBe(false)
  })

  it('logs out, clears access and memory auth, and routes to Login', async () => {
    useAccessStore(pinia).applySnapshot({
      roleCodes: ['admin'],
      menuTree: [],
      permissionCodes: ['system:user:list'],
    })
    const { wrapper, router } = await mountLayout()
    await wrapper.get('[data-testid="logout"]').trigger('click')
    await flushPromises()
    expect(logoutMock).toHaveBeenCalledOnce()
    expect(useAccessStore(pinia).status).toBe('idle')
    expect(useAccessStore(pinia).roleCodes).toEqual([])
    expect(useAccessStore(pinia).permissionCodes).toEqual([])
    expect(useAuthStore(pinia).status).toBe('anonymous')
    expect(router.currentRoute.value.path).toBe('/login')
  })
})

async function mountLayout(path = '/dashboard') {
  const layoutContent = {
    setup() {
      const render = ++layoutRenderCount
      return { render }
    },
    template: '<div data-testid="layout-content" :data-render="render">dashboard content</div>',
  }
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/dashboard',
        name: 'dashboard',
        component: layoutContent,
        meta: { requiresAuth: true, titleKey: 'navigation.dashboard', affix: true },
      },
      {
        path: '/system/users',
        name: 'system-users',
        component: layoutContent,
        meta: { requiresAuth: true, titleKey: 'navigation.systemUsers' },
      },
      {
        path: '/system/missing',
        name: 'system-missing',
        component: layoutContent,
        meta: { requiresAuth: true, titleKey: 'navigation.systemUsers' },
      },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(Layout, { global: { plugins: [ElementPlus, pinia, router, appI18n] } })
  return { wrapper, router }
}

function getPopupItem(testId: string): HTMLElement {
  const items = Array.from(document.body.querySelectorAll<HTMLElement>(`[data-testid="${testId}"]`))
  const item = items.at(-1)
  if (item === undefined) throw new Error(`Missing dropdown item: ${testId}`)
  return item
}
