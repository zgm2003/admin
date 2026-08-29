import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getHealth, getReadiness } from '@src/api/health'
import { appI18n, setLocale } from '@src/i18n'
import Dashboard from '@src/views/dashboard/index.vue'

vi.mock('@src/api/health', () => ({
  getHealth: vi.fn(),
  getReadiness: vi.fn(),
}))

const mockedHealth = vi.mocked(getHealth)
const mockedReadiness = vi.mocked(getReadiness)

describe('Dashboard', () => {
  beforeEach(() => {
    localStorage.clear()
    setLocale('zh-CN')
    vi.clearAllMocks()
    mockedHealth.mockResolvedValue({ status: 'up' })
    mockedReadiness.mockResolvedValue({ postgresql: 'up', redis: 'up' })
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

  it('renders Dashboard status in the selected locale', async () => {
    setLocale('en-US')
    const wrapper = mountDashboard()
    await flushPromises()
    expect(wrapper.get('#dashboard-title').text()).toBe('Dashboard')
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
})

function mountDashboard() {
  return mount(Dashboard, {
    global: {
      plugins: [ElementPlus, appI18n],
      stubs: {
        ReadinessChart: true,
      },
    },
  })
}
