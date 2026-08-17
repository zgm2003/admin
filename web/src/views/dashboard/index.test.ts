import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getHealth, getReadiness } from '../../api/health'
import { createExampleTask } from '../../api/taskDemo'
import Dashboard from './index.vue'

vi.mock('../../api/health', () => ({
  getHealth: vi.fn(),
  getReadiness: vi.fn(),
}))

vi.mock('../../api/taskDemo', () => ({
  createExampleTask: vi.fn(),
}))

const mockedHealth = vi.mocked(getHealth)
const mockedReadiness = vi.mocked(getReadiness)
const mockedCreateTask = vi.mocked(createExampleTask)

describe('Dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedHealth.mockResolvedValue({ status: 'up' })
    mockedReadiness.mockResolvedValue({ postgresql: 'up', redis: 'up' })
  })

  it('shows the real API and dependency states', async () => {
    const wrapper = mountDashboard()

    await flushPromises()

    const topbar = wrapper.get('header.admin-header')
    expect(topbar.find('[data-testid="api-status"]').exists()).toBe(true)
    expect(topbar.find('[data-testid="postgresql-status"]').exists()).toBe(true)
    expect(topbar.find('[data-testid="redis-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="api-status"]').text()).toContain('运行正常')
    expect(wrapper.get('[data-testid="postgresql-status"]').text()).toContain('运行正常')
    expect(wrapper.get('[data-testid="redis-status"]').text()).toContain('运行正常')
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

  it('requires a message and displays the returned task ID', async () => {
    mockedCreateTask.mockResolvedValue({ taskId: 'task-123' })
    const wrapper = mountDashboard()
    await flushPromises()

    const submit = wrapper.get('[data-testid="task-submit"]')
    expect(submit.attributes('disabled')).toBeDefined()

    await wrapper.get('[data-testid="task-message"]').setValue('foundation-check')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(mockedCreateTask).toHaveBeenCalledWith({ message: 'foundation-check' })
    expect(wrapper.get('[data-testid="task-id"]').text()).toContain('task-123')
  })

  it('shows an explicit task submission failure', async () => {
    mockedCreateTask.mockRejectedValue(new Error('queue unavailable'))
    const wrapper = mountDashboard()
    await flushPromises()

    await wrapper.get('[data-testid="task-message"]').setValue('foundation-check')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('.task-panel .inline-error').text()).toContain('queue unavailable')
  })
})

function mountDashboard() {
  return mount(Dashboard, {
    global: {
      plugins: [ElementPlus],
      stubs: {
        ReadinessChart: true,
      },
    },
  })
}
