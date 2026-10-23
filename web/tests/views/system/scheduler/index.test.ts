import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
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
  setScheduleStatus: vi.fn(),
  deleteSchedule: vi.fn(),
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
function jobPage(list: api.Job[], page = 1, total = list.length) {
  return { list, total, page, pageSize: 20 }
}
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
  const wrapper = mount(SchedulerPage, {
    attachTo: document.body,
    global: { plugins: [pinia, ElementPlus, appI18n] },
  })
  wrappers.push(wrapper)
  return wrapper
}
async function openJobs(wrapper: VueWrapper): Promise<void> {
  expect(wrapper.findComponent({ name: 'ElTabs' }).exists()).toBe(false)
  await wrapper.get('[data-testid="schedule-jobs-42"]').trigger('click')
  await flushPromises()
}

describe('backend-owned scheduler presentation and actions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue(
      'confirm' as unknown as Awaited<ReturnType<typeof ElMessageBox.confirm>>,
    )
    setLocale('zh-CN')
    vi.mocked(api.getSchedulerOptions).mockResolvedValue(options)
    vi.mocked(api.listSchedules).mockResolvedValue([
      {
        id: 42,
        name: 'Cleanup',
        description: '',
        taskType: 'cleanup',
        cronExpression: '*/5 * * * *',
        timezone: 'UTC',
        params: {},
        status: true,
        status: 2,
        actions: { delete: true },
        nextRunAt: null,
        builtinKey: '',
        createdAt: timestamp,
        updatedAt: timestamp,
      },
    ])
    vi.mocked(api.listJobs).mockResolvedValue(jobPage([job(1, 51, true), job(2, 99, false)]))
    vi.mocked(api.retryJob).mockResolvedValue(job(3, 51, false))
  })
  afterEach(() => {
    wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
    document.body.innerHTML = ''
  })

  it('only requests execution history after selecting a plan and filters the main table', async () => {
    const wrapper = mountPage(['system:scheduler:list'])
    await flushPromises()
    expect(api.listJobs).not.toHaveBeenCalled()
    await openJobs(wrapper)
    expect(api.listJobs).toHaveBeenCalledWith({ scheduleId: 42, page: 1, pageSize: 20 })
  })

  it('filters plans with a typed dropdown and clears back to the full loaded list', async () => {
    const first = (await api.listSchedules())[0]
    if (!first) throw new Error('missing fixture')
    vi.mocked(api.listSchedules).mockResolvedValue([
      first,
      { ...first, id: 89, name: 'Other plan' },
    ])
    const wrapper = mountPage(['system:scheduler:list'])
    await flushPromises()
    const filter = wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((item) => item.attributes('data-testid') === 'schedule-plan-filter')
    filter?.vm.$emit('update:modelValue', 89)
    await flushPromises()
    expect(
      wrapper
        .findComponent({ name: 'AppTable' })
        .props('data')
        .map((row: api.Schedule) => row.id),
    ).toEqual([89])
    filter?.vm.$emit('update:modelValue', '')
    await flushPromises()
    expect(wrapper.findComponent({ name: 'AppTable' }).props('data')).toHaveLength(2)
    expect(api.listJobs).not.toHaveBeenCalled()
  })

  it('rejects a late history response after switching plans', async () => {
    const first = (await api.listSchedules())[0]
    if (!first) throw new Error('missing fixture')
    vi.mocked(api.listSchedules).mockResolvedValue([
      first,
      { ...first, id: 89, name: 'Other plan' },
    ])
    let resolveOlder!: (jobs: Awaited<ReturnType<typeof api.listJobs>>) => void
    const older = new Promise<Awaited<ReturnType<typeof api.listJobs>>>((resolve) => {
      resolveOlder = resolve
    })
    vi.mocked(api.listJobs)
      .mockReturnValueOnce(older)
      .mockResolvedValueOnce(jobPage([job(777, 51, false)]))
    const wrapper = mountPage(['system:scheduler:list'])
    await flushPromises()
    await wrapper.get('[data-testid="schedule-jobs-42"]').trigger('click')
    await flushPromises()
    wrapper
      .findAllComponents({ name: 'AppDialog' })
      .find((item) => item.props('title').includes('执行记录'))
      ?.vm.$emit('update:modelValue', false)
    await flushPromises()
    await wrapper.get('[data-testid="schedule-jobs-89"]').trigger('click')
    await flushPromises()
    resolveOlder(jobPage([job(666, 99, false)]))
    await flushPromises()
    const table = wrapper
      .findAllComponents({ name: 'AppTable' })
      .find((item) =>
        item.props('columns').some((column: { prop: string }) => column.prop === 'attemptCount'),
      )
    expect(table?.props('data').map((row: api.Job) => row.id)).toEqual([777])
    expect(api.listJobs).toHaveBeenLastCalledWith({ scheduleId: 89, page: 1, pageSize: 20 })
  })

  it('paginates history with backend totals and replaces the current page', async () => {
    vi.mocked(api.listJobs)
      .mockResolvedValueOnce(
        jobPage(
          Array.from({ length: 20 }, (_, index) => job(40 - index, 51, false)),
          1,
          21,
        ),
      )
      .mockResolvedValueOnce(jobPage([job(20, 51, false)], 2, 21))
    const wrapper = mountPage(['system:scheduler:list'])
    await flushPromises()
    await openJobs(wrapper)
    const table = wrapper
      .findAllComponents({ name: 'AppTable' })
      .find((item) =>
        item.props('columns').some((column: { prop: string }) => column.prop === 'attemptCount'),
      )
    expect(table?.props('pagination')).toEqual({ currentPage: 1, pageSize: 20, total: 21 })
    table?.vm.$emit('update:pagination', { currentPage: 2, pageSize: 20, total: 21 })
    await flushPromises()
    expect(api.listJobs).toHaveBeenLastCalledWith({ scheduleId: 42, page: 2, pageSize: 20 })
    expect(table?.props('data').map((row: api.Job) => row.id)).toEqual([20])
    expect(table?.props('pagination').currentPage).toBe(2)
    wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((item) => item.attributes('data-testid') === 'schedule-job-status')
      ?.vm.$emit('update:modelValue', 51)
    await flushPromises()
    expect(api.listJobs).toHaveBeenLastCalledWith({
      scheduleId: 42,
      page: 1,
      pageSize: 20,
      status: 51,
    })

    expect(document.querySelector('[data-testid="schedule-jobs-more"]')).toBeNull()
    table?.vm.$emit('update:pagination', { currentPage: 2, pageSize: 50, total: 21 })
    await flushPromises()
    expect(api.listJobs).toHaveBeenLastCalledWith({
      scheduleId: 42,
      page: 1,
      pageSize: 50,
      status: 51,
    })
  })

  it.each([true, false])(
    'confirms status changes from enabled=%s before sending a request',
    async (status) => {
      const schedule: api.Schedule = {
        id: 42,
        name: 'Cleanup',
        description: '',
        taskType: 'cleanup',
        cronExpression: '*/5 * * * *',
        timezone: 'UTC',
        params: {},
        status,
        status: 1,
        actions: { delete: true },
        nextRunAt: null,
        builtinKey: '',
        createdAt: timestamp,
        updatedAt: timestamp,
      }
      vi.mocked(api.listSchedules).mockResolvedValue([schedule])
      vi.mocked(ElMessageBox.confirm).mockRejectedValueOnce('cancel')
      const wrapper = mountPage(['system:scheduler:list', 'system:scheduler:status'])
      await flushPromises()
      const button = wrapper
        .findAll('button')
        .find((button) => button.text() === (status ? '停用' : '启用'))
      expect(button).toBeDefined()
      await button?.trigger('click')
      await flushPromises()
      expect(ElMessageBox.confirm).toHaveBeenCalledOnce()
      expect(api.setScheduleStatus).not.toHaveBeenCalled()
      await button?.trigger('click')
      await flushPromises()
      expect(api.setScheduleStatus).toHaveBeenCalledWith(42, !status)
    },
  )

  it('uses backend labels and raw unknown values and intersects retry actions with Access', async () => {
    const wrapper = mountPage(['system:scheduler:list', 'system:scheduler:retry'])
    await flushPromises()
    await openJobs(wrapper)
    expect(document.body.textContent).toContain('后端新状态')
    expect(document.body.textContent).toContain('99')
    expect(document.body.textContent).toContain('unregistered.task')
    expect(document.body.textContent).toContain('new-trigger')
    const retryButtons = Array.from(document.querySelectorAll('button'))
      .filter((button) => button.textContent?.trim() === '重试')
      .map((button) => new DOMWrapper(button))
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
    expect(
      Array.from(document.querySelectorAll('button')).filter(
        (button) => button.textContent?.trim() === '重试',
      ),
    ).toHaveLength(0)
  })

  it('preserves a zero-valued backend status filter', async () => {
    const wrapper = mountPage(['system:scheduler:list'])
    await flushPromises()
    await openJobs(wrapper)
    wrapper
      .findAllComponents({ name: 'ElSelectV2' })
      .find((item) => item.attributes('data-testid') === 'schedule-job-status')
      ?.vm.$emit('update:modelValue', 0)
    await flushPromises()
    expect(api.listJobs).toHaveBeenLastCalledWith({
      scheduleId: 42,
      status: 0,
      page: 1,
      pageSize: 20,
    })
  })
})
