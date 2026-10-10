import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/api/system/scheduler'
import { appI18n, setLocale } from '@/i18n'
import { usePermissionStore } from '@/store/permission'
import SchedulerPage from '@/views/system/scheduler/index.vue'

vi.mock('@/api/system/scheduler', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/system/scheduler')>()),
  getSchedulerOptions: vi.fn(),
  listSchedules: vi.fn(),
  listJobs: vi.fn(),
  retryJob: vi.fn(),
}))

const options: api.SchedulerOptions = {
  defaults: { cronExpression: '*/11 * * * *', timezone: 'UTC' },
  tasks: [],
  jobStatuses: [{ value: 51, label: '后端新状态', tone: 'warning' }],
  runStatuses: [],
  scheduleStatuses: [],
  triggerSources: [],
  cronPresets: [{ value: '*/11 * * * *', label: '后端新周期' }],
}
const timestamp = '2026-09-21T00:00:00Z'
function job(id: number, status: number, retry: boolean): api.Job {
  return {
    id,
    scheduleId: null,
    taskType: 'unregistered.task',
    payload: {},
    triggerSource: 'new-trigger',
    scheduledAt: timestamp,
    availableAt: timestamp,
    status,
    actions: { retry },
    attemptCount: 1,
    maxAttempts: 3,
    errorClass: '',
    lastError: '',
    completedAt: null,
    createdAt: timestamp,
    updatedAt: timestamp,
  }
}
const wrappers: VueWrapper[] = []
function mountPage(codes: string[]): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore().applySnapshot({ roleCodes: [], menuTree: [], permissionCodes: codes })
  const wrapper = mount(SchedulerPage, { global: { plugins: [pinia, ElementPlus, appI18n] } })
  wrappers.push(wrapper)
  return wrapper
}
async function openJobs(wrapper: VueWrapper): Promise<void> {
  const tab = wrapper.findAll('[role="tab"]').find((item) => item.text() === '执行记录')
  if (!tab) throw new Error('jobs tab missing')
  await tab.trigger('click')
  await flushPromises()
}

describe('backend-owned scheduler presentation and actions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    vi.mocked(api.getSchedulerOptions).mockResolvedValue(options)
    vi.mocked(api.listSchedules).mockResolvedValue([])
    vi.mocked(api.listJobs).mockResolvedValue([job(1, 51, true), job(2, 99, false)])
    vi.mocked(api.retryJob).mockResolvedValue(job(3, 51, false))
  })
  afterEach(() => wrappers.splice(0).forEach((wrapper) => wrapper.unmount()))

  it('uses backend labels and raw unknown values and intersects retry actions with Access', async () => {
    const wrapper = mountPage(['system:scheduler:list', 'system:scheduler:retry'])
    await flushPromises()
    await openJobs(wrapper)
    expect(wrapper.text()).toContain('后端新状态')
    expect(wrapper.text()).toContain('99')
    expect(wrapper.text()).toContain('unregistered.task')
    expect(wrapper.text()).toContain('new-trigger')
    const retryButtons = wrapper.findAll('button').filter((button) => button.text() === '重试')
    expect(retryButtons).toHaveLength(1)
    await retryButtons[0]?.trigger('click')
    await flushPromises()
    expect(api.retryJob).toHaveBeenCalledWith(1)
    usePermissionStore().applySnapshot({
      roleCodes: [],
      menuTree: [],
      permissionCodes: ['system:scheduler:list'],
    })
    await flushPromises()
    expect(wrapper.findAll('button').filter((button) => button.text() === '重试')).toHaveLength(0)
  })

  it('preserves a zero-valued backend status filter', async () => {
    const wrapper = mountPage(['system:scheduler:list'])
    await flushPromises()
    await openJobs(wrapper)
    wrapper.findComponent({ name: 'ElSelectV2' }).vm.$emit('update:modelValue', 0)
    await flushPromises()
    expect(api.listJobs).toHaveBeenLastCalledWith(0, 0)
  })
})
