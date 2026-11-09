import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { VueWrapper } from '@vue/test-utils'
import type { ServerSnapshot } from '@/api/system/server'
import { useBrandStore } from '@/store/brand'

import { getHealth, getReadiness } from '@/api/health'
import { getServerSnapshot } from '@/api/system/server'
import { usePermissionStore } from '@/store/permission'
import { appI18n, setLocale } from '@/i18n'
import Dashboard from '@/views/dashboard/index.vue'

vi.mock('@/api/health', () => ({
  getHealth: vi.fn(),
  getReadiness: vi.fn(),
}))
vi.mock('@/api/system/server', () => ({ getServerSnapshot: vi.fn() }))

const mockedHealth = vi.mocked(getHealth)
const mockedReadiness = vi.mocked(getReadiness)
const serverSnapshot: ServerSnapshot = {
  sampledAt: '2026-11-06T10:00:00Z',
  hostname: 'node-a',
  os: 'linux',
  arch: 'amd64',
  logicalCores: 8,
  cpuPercent: 12.5,
  memory: { totalBytes: 17179869184, availableBytes: 8589934592, usedPercent: 50 },
  disk: { totalBytes: 107374182400, availableBytes: 53687091200, usedPercent: 50 },
  diskPath: '/',
  hostUptimeSeconds: 90061,
  processUptimeSeconds: 3661,
  startedAt: '2026-11-06T08:58:59Z',
  goVersion: 'go1.26.5',
  goroutines: 42,
  heapAllocBytes: 33554432,
}
const mounted: VueWrapper[] = []
afterEach(() => {
  mounted.splice(0).forEach((wrapper) => wrapper.unmount())
})

describe('Dashboard', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    vi.resetAllMocks()
    setActivePinia(createPinia())
    mockedHealth.mockResolvedValue({ status: 'up' })
    mockedReadiness.mockResolvedValue({ postgresql: 'up', redis: 'up' })
    vi.mocked(getServerSnapshot).mockResolvedValue(serverSnapshot)
  })

  it('shows the real API and dependency states', async () => {
    const wrapper = mountDashboard()

    await flushPromises()

    const summary = wrapper.get('[data-testid="dashboard-summary"]')
    expect(summary.find('[data-testid="api-status"]').exists()).toBe(true)
    expect(summary.find('[data-testid="postgresql-status"]').exists()).toBe(true)
    expect(summary.find('[data-testid="redis-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="api-status"]').text()).toContain('运行正常')
    expect(wrapper.get('[data-testid="postgresql-status"]').text()).toContain('运行正常')
    expect(wrapper.get('[data-testid="redis-status"]').text()).toContain('运行正常')
  })

  it('uses the three real dependency results as dashboard summaries', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.findAll('[data-testid$="-status"]')).toHaveLength(3)
    expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('API')
    expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('PostgreSQL')
    expect(wrapper.get('[data-testid="dashboard-summary"]').text()).toContain('Redis')
  })

  it('renders Dashboard status in the selected locale without a duplicated page header', async () => {
    setLocale('en-US')
    const wrapper = mountDashboard()
    await flushPromises()
    expect(wrapper.find('.dashboard-toolbar').exists()).toBe(false)
    expect(wrapper.get('.dashboard-intro .section-kicker').text()).toBe('Operations overview')
    expect(wrapper.get('[data-testid="api-status"]').text()).toContain('Operational')
  })

  it('shows an explicit readiness failure instead of fake healthy states', async () => {
    mockedReadiness.mockRejectedValue(new Error('connection refused'))
    const wrapper = mountDashboard()

    await flushPromises()

    expect(wrapper.get('[data-testid="api-status"]').text()).toContain('运行正常')
    expect(wrapper.get('[data-testid="postgresql-status"]').text()).toContain('检查失败')
    expect(wrapper.get('[data-testid="redis-status"]').text()).toContain('检查失败')
    expect(wrapper.get('[data-testid="health-error"]').text()).toContain('connection refused')
  })

  it('does not render the removed example task controls', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.find('[data-testid="task-submit"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="task-message"]').exists()).toBe(false)
    expect(wrapper.find('.task-panel').exists()).toBe(false)
  })

  it('shows server details to every signed-in dashboard user', async () => {
    usePermissionStore().permissionCodes = []
    const wrapper = mountDashboard()
    await flushPromises()
    expect(getServerSnapshot).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="server-host"]').text()).toContain('node-a')
    expect(wrapper.get('[data-testid="project-introduction"]').text()).toContain('Vue 3')
  })

  it('shows real metrics and clears old values on refresh failure', async () => {
    usePermissionStore().permissionCodes = []
    const wrapper = mountDashboard()
    await flushPromises()
    expect(wrapper.get('[data-testid="server-metrics"]').text()).toContain('12.5%')
    expect(wrapper.get('[data-testid="server-host"]').text()).toContain('node-a')
    vi.mocked(getServerSnapshot).mockRejectedValue(new Error('metrics unavailable'))
    await wrapper.get('[data-testid="server-refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="server-metrics"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="server-error"]').text()).toContain('metrics unavailable')
    expect(wrapper.get('[data-testid="api-status"]').text()).toContain('运行正常')
  })

  it('does not show operational links for unregistered or unauthorized pages', async () => {
    const wrapper = mountDashboard()
    await flushPromises()
    expect(wrapper.find('[data-testid="operations-links"]').exists()).toBe(false)
  })

  it('coalesces local and global refresh while the metrics request is pending', async () => {
    let resolve: (value: ServerSnapshot) => void = () => {
      throw new Error('request not started')
    }
    vi.mocked(getServerSnapshot).mockImplementation(
      () =>
        new Promise((done) => {
          resolve = done
        }),
    )
    const wrapper = mountDashboard()
    await flushPromises()
    await wrapper.get('[data-testid="server-refresh"]').trigger('click')
    await wrapper.get('[data-testid="dashboard-refresh"]').trigger('click')
    await flushPromises()
    expect(getServerSnapshot).toHaveBeenCalledTimes(1)
    resolve(serverSnapshot)
    await flushPromises()
    expect(wrapper.get('[data-testid="server-host"]').text()).toContain('node-a')
  })

  it('binds the project name to system brand and language settings', async () => {
    useBrandStore().apply({
      titleZhCN: '运维管理台',
      titleEnUS: 'Operations console',
      defaultAvatar: '',
    })
    const wrapper = mountDashboard()
    await flushPromises()
    expect(wrapper.get('.dashboard-intro h2').text()).toBe('运维管理台')
    setLocale('en-US')
    await flushPromises()
    expect(wrapper.get('.dashboard-intro h2').text()).toBe('Operations console')
  })

  it('shows only directly granted and registered operation pages', async () => {
    const permissions = usePermissionStore()
    permissions.permissionCodes = ['system:server:view', 'system:setting:view']
    const wrapper = mountDashboard(true)
    await flushPromises()
    const links = wrapper.get('[data-testid="operations-links"]').findAll('a')
    expect(links).toHaveLength(1)
    expect(links[0]?.attributes('href')).toBe('/system/server')
    permissions.permissionCodes = ['system:setting:view']
    await flushPromises()
    expect(wrapper.find('[data-testid="operations-links"]').exists()).toBe(false)
  })
})

function mountDashboard(registerServer = false) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div />' } }],
  })
  if (registerServer)
    router.addRoute({
      path: '/system/server',
      name: 'access:system:server:view',
      component: { template: '<div />' },
    })
  const wrapper = mount(Dashboard, {
    global: {
      plugins: [ElementPlus, appI18n, router],
    },
  })
  mounted.push(wrapper)
  return wrapper
}
