import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '@/api/system/queueMonitor'
import { appI18n } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import QueueMonitorPage from '@/views/system/queueMonitor/index.vue'

vi.mock('@/api/system/queueMonitor', () => ({
  QUEUE_MONITOR_UI_URL: '/api/admin/v1/system/queuemonitor/ui/',
  grantQueueMonitor: vi.fn(),
}))

describe('queue monitor page', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.mocked(api.grantQueueMonitor).mockReset().mockResolvedValue({ expiresAt: '2026-09-14T12:00:00Z' })
  })

  it('loads iframe only after grant and renews without changing src', async () => {
    const wrapper = mountPage(['system:queueMonitor:list'])
    expect(wrapper.find('iframe').exists()).toBe(false)
    await flushPromises()
    const frame = wrapper.get('iframe')
    const src = frame.attributes('src')
    expect(src).toBe('/api/admin/v1/system/queuemonitor/ui/')
    expect(src).not.toContain('token')

    await vi.advanceTimersByTimeAsync(45_000)
    expect(api.grantQueueMonitor).toHaveBeenCalledTimes(2)
    expect(wrapper.get('iframe').attributes('src')).toBe(src)
    wrapper.unmount()
  })

  it('does not request a grant without list permission', async () => {
    const wrapper = mountPage([])
    await flushPromises()
    expect(api.grantQueueMonitor).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('无权查看任务队列')
    wrapper.unmount()
  })

  it('shows a retry state when the embedded monitor fails to load', async () => {
    const wrapper = mountPage(['system:queueMonitor:list'])
    await flushPromises()
    await wrapper.get('iframe').trigger('error')
    expect(wrapper.text()).toContain('任务队列界面加载失败')
    expect(wrapper.find('iframe').exists()).toBe(false)
    wrapper.unmount()
  })
})

function mountPage(permissionCodes: string[]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore(pinia).applySnapshot({ roleCodes: [], menuTree: [], permissionCodes })
  return mount(QueueMonitorPage, { global: { plugins: [pinia, appI18n] } })
}
