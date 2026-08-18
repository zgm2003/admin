import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { logout } from '../api/auth'
import { pinia } from '../store'
import { useAuthStore } from '../store/auth'
import Layout from './index.vue'

vi.mock('../api/auth', () => ({ logout: vi.fn() }))

const logoutMock = vi.mocked(logout)

describe('admin layout', () => {
  beforeEach(() => {
    logoutMock.mockReset()
    logoutMock.mockResolvedValue()
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.style.removeProperty('color-scheme')
    useAuthStore(pinia).$reset()
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

  it('toggles the Element Plus dark theme from the Header', async () => {
    const { wrapper } = await mountLayout()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    await wrapper.get('[data-testid="toggle-theme"]').trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('admin:theme')).toBe('dark')
  })

  it('opens a Drawer instead of collapsing on mobile', async () => {
    const { wrapper } = await mountLayout()
    window.innerWidth = 600
    window.dispatchEvent(new Event('resize'))
    await wrapper.vm.$nextTick()
    await wrapper.get('[data-testid="toggle-menu"]').trigger('click')
    const drawer = wrapper.findComponent({ name: 'ElDrawer' })
    expect(drawer.props('modelValue')).toBe(true)
    expect(wrapper.get('[data-testid="app-aside"]').attributes('data-collapsed')).toBe('false')
  })

  it('logs out, clears memory auth, and routes to Login', async () => {
    const { wrapper, router } = await mountLayout()
    await wrapper.get('[data-testid="logout"]').trigger('click')
    await flushPromises()
    expect(logoutMock).toHaveBeenCalledOnce()
    expect(useAuthStore(pinia).status).toBe('anonymous')
    expect(router.currentRoute.value.path).toBe('/login')
  })
})

async function mountLayout() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/dashboard',
        name: 'dashboard',
        component: { template: '<div data-testid="layout-content">dashboard content</div>' },
      },
      { path: '/login', name: 'login', component: { template: '<div>login</div>' } },
    ],
  })
  await router.push('/dashboard')
  await router.isReady()
  const wrapper = mount(Layout, { global: { plugins: [ElementPlus, pinia, router] } })
  return { wrapper, router }
}
