import { flushPromises, mount } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { listRuns } from '@/api/system/scheduler'
import type { Job, Run } from '@/api/system/scheduler'
import { JobStatus, RunStatus } from '@/enums/scheduler'
import { appI18n, setLocale } from '@/i18n'
import JobDetailDialog from '@/views/system/scheduler/components/JobDetailDialog/index.vue'

vi.mock('@/api/system/scheduler', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/system/scheduler')>()
  return { ...actual, listRuns: vi.fn() }
})

describe('JobDetailDialog request lifecycle', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLocale('zh-CN')
    document.body.innerHTML = ''
  })

  it('does not let an older run response overwrite a newly opened job', async () => {
    const older = deferred<Run[]>()
    vi.mocked(listRuns)
      .mockReturnValueOnce(older.promise)
      .mockResolvedValueOnce([run(2, 2, 'new-worker')])
    const wrapper = mount(JobDetailDialog, {
      attachTo: document.body,
      props: { modelValue: false, job: job(1), taskOptions: [] },
      global: { plugins: [ElementPlus, appI18n] },
    })

    await wrapper.setProps({ modelValue: true })
    await vi.waitFor(() => expect(listRuns).toHaveBeenCalledWith(1))
    await wrapper.setProps({ modelValue: false })
    await wrapper.setProps({ job: job(2) })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    expect(document.body.textContent).toContain('new-worker')

    older.resolve([run(1, 1, 'old-worker')])
    await flushPromises()
    expect(document.body.textContent).toContain('new-worker')
    expect(document.body.textContent).not.toContain('old-worker')

    wrapper.unmount()
  })
})

function job(id: number): Job {
  return {
    id,
    scheduleId: null,
    taskType: 'test.task',
    payload: {},
    triggerSource: 'manual',
    scheduledAt: '2026-09-21T00:00:00Z',
    availableAt: '2026-09-21T00:00:00Z',
    status: JobStatus.completed,
    attemptCount: 1,
    maxAttempts: 3,
    errorClass: '',
    lastError: '',
    completedAt: '2026-09-21T00:00:01Z',
    createdAt: '2026-09-21T00:00:00Z',
    updatedAt: '2026-09-21T00:00:01Z',
  }
}

function run(id: number, jobId: number, workerId: string): Run {
  return {
    id,
    jobId,
    attemptNo: 1,
    status: RunStatus.succeeded,
    workerId,
    startedAt: '2026-09-21T00:00:00Z',
    finishedAt: '2026-09-21T00:00:01Z',
    durationMs: 1000,
    errorClass: '',
    errorMessage: '',
    resultSummary: '',
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}
