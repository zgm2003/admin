import type { PageResult } from '@/types/pagination'
import { request } from '@/utils/request'
import type { BackendOption, StatusOption } from '@/types/option'
export type JobStatus = number
export type RunStatus = number

export interface Schedule {
  id: number
  name: string
  description: string
  taskType: string
  cronExpression: string
  timezone: string
  params: Record<string, unknown>
  status: boolean
  status: number
  actions: { delete: boolean }
  nextRunAt: string | null
  builtinKey: string
  createdAt: string
  updatedAt: string
}

export interface Job {
  id: number
  scheduleId: number | null
  taskType: string
  payload: Record<string, unknown>
  triggerSource: string
  scheduledAt: string
  availableAt: string
  status: JobStatus
  actions: { retry: boolean }
  attemptCount: number
  maxAttempts: number
  errorClass: string
  lastError: string
  completedAt: string | null
  createdAt: string
  updatedAt: string
}

export interface Run {
  id: number
  jobId: number
  attemptNo: number
  status: RunStatus
  workerId: string
  startedAt: string
  finishedAt: string | null
  durationMs: number | null
  errorClass: string
  errorMessage: string
}

export interface TaskOption {
  type: string
  displayName: string
  adminCreatable: boolean
  builtinKey: string
  defaultParams: Record<string, unknown>
}

export interface SchedulerOptions {
  defaults: { cronExpression: string; timezone: string }
  tasks: TaskOption[]
  jobStatuses: StatusOption[]
  runStatuses: StatusOption[]
  scheduleStatuses: StatusOption[]
  triggerSources: BackendOption<string>[]
  cronPresets: BackendOption<string>[]
}

export async function listSchedules(beforeId = 0): Promise<Schedule[]> {
  return request.get<Schedule[]>('/api/admin/v1/system/scheduler/schedule', {
    params: { beforeId, limit: 100 },
  })
}

export interface JobQuery {
  scheduleId: number
  status?: JobStatus
  page: number
  pageSize: number
}

export async function listJobs(query: JobQuery): Promise<PageResult<Job>> {
  return request.get<PageResult<Job>>('/api/admin/v1/system/scheduler/job', { params: query })
}

export async function listRuns(jobId: number): Promise<Run[]> {
  return request.get<Run[]>(`/api/admin/v1/system/scheduler/job/${jobId}/run`, {
    params: { afterId: 0, limit: 100 },
  })
}

export function getSchedulerOptions(): Promise<SchedulerOptions> {
  return request.get<SchedulerOptions>('/api/admin/v1/system/scheduler/options')
}

export async function createSchedule(input: {
  name: string
  description: string
  taskType: string
  cronExpression: string
  timezone: string
  params: Record<string, unknown>
  status: boolean
}): Promise<Schedule> {
  return request.post<Schedule>('/api/admin/v1/system/scheduler/schedule', input)
}

export async function updateSchedule(
  id: number,
  input: {
    name: string
    description: string
    cronExpression: string
    timezone: string
    params: Record<string, unknown>
  },
): Promise<Schedule> {
  return request.put<Schedule>(`/api/admin/v1/system/scheduler/schedule/${id}`, input)
}

export async function setScheduleStatus(id: number, status: boolean): Promise<void> {
  return request.patch<void>(`/api/admin/v1/system/scheduler/schedule/${id}/status`, { status })
}

export async function deleteSchedule(id: number): Promise<void> {
  return request.delete<void>(`/api/admin/v1/system/scheduler/schedule/${id}`)
}

export async function executeSchedule(id: number): Promise<Job> {
  return request.post<Job>(`/api/admin/v1/system/scheduler/schedule/${id}/execute`, undefined)
}

export async function retryJob(id: number): Promise<Job> {
  return request.post<Job>(`/api/admin/v1/system/scheduler/job/${id}/retry`, undefined)
}
