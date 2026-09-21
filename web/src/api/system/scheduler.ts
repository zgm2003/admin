import { request } from '@/utils/request'
import {
  expectArray,
  expectBoolean,
  expectEmptyObject,
  expectExactKeys,
  expectInteger,
  expectNullableString,
  expectRecord,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'
import { isJobStatus, isRunStatus } from '@/enums/scheduler'
import type { JobStatus, RunStatus } from '@/enums/scheduler'

export interface Schedule {
  id: number
  name: string
  description: string
  taskType: string
  cronExpression: string
  timezone: string
  params: Record<string, unknown>
  isEnabled: boolean
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
  resultSummary: string
}
export interface TaskOption {
  type: string
  displayName: string
  adminCreatable: boolean
  builtinKey: string
  defaultParams: Record<string, unknown>
}

function object(value: unknown, context: string): Record<string, unknown> {
  return expectRecord(value, context)
}
function jsonObject(value: unknown, context: string): Record<string, unknown> {
  const record = object(value, context)
  return record
}
function parseSchedule(value: unknown, context: string): Schedule {
  const r = expectExactKeys(
    value,
    [
      'id',
      'name',
      'description',
      'taskType',
      'cronExpression',
      'timezone',
      'params',
      'isEnabled',
      'nextRunAt',
      'builtinKey',
      'createdAt',
      'updatedAt',
    ],
    context,
  )
  return {
    id: expectInteger(r.id, `${context}.id`),
    name: expectString(r.name, `${context}.name`),
    description: expectString(r.description, `${context}.description`),
    taskType: expectString(r.taskType, `${context}.taskType`),
    cronExpression: expectString(r.cronExpression, `${context}.cronExpression`),
    timezone: expectString(r.timezone, `${context}.timezone`),
    params: jsonObject(r.params, `${context}.params`),
    isEnabled: expectBoolean(r.isEnabled, `${context}.isEnabled`),
    nextRunAt: expectNullableString(r.nextRunAt, `${context}.nextRunAt`),
    builtinKey: expectString(r.builtinKey, `${context}.builtinKey`),
    createdAt: expectString(r.createdAt, `${context}.createdAt`),
    updatedAt: expectString(r.updatedAt, `${context}.updatedAt`),
  }
}
function parseJob(value: unknown, context: string): Job {
  const r = expectExactKeys(
    value,
    [
      'id',
      'scheduleId',
      'taskType',
      'payload',
      'triggerSource',
      'scheduledAt',
      'availableAt',
      'status',
      'attemptCount',
      'maxAttempts',
      'errorClass',
      'lastError',
      'completedAt',
      'createdAt',
      'updatedAt',
    ],
    context,
  )
  const status = expectInteger(r.status, `${context}.status`)
  if (!isJobStatus(status)) throw new ProtocolError(`${context}.status is invalid`)
  return {
    id: expectInteger(r.id, `${context}.id`),
    scheduleId: r.scheduleId === null ? null : expectInteger(r.scheduleId, `${context}.scheduleId`),
    taskType: expectString(r.taskType, `${context}.taskType`),
    payload: jsonObject(r.payload, `${context}.payload`),
    triggerSource: expectString(r.triggerSource, `${context}.triggerSource`),
    scheduledAt: expectString(r.scheduledAt, `${context}.scheduledAt`),
    availableAt: expectString(r.availableAt, `${context}.availableAt`),
    status,
    attemptCount: expectInteger(r.attemptCount, `${context}.attemptCount`),
    maxAttempts: expectInteger(r.maxAttempts, `${context}.maxAttempts`),
    errorClass: expectString(r.errorClass, `${context}.errorClass`),
    lastError: expectString(r.lastError, `${context}.lastError`),
    completedAt: expectNullableString(r.completedAt, `${context}.completedAt`),
    createdAt: expectString(r.createdAt, `${context}.createdAt`),
    updatedAt: expectString(r.updatedAt, `${context}.updatedAt`),
  }
}
function parseRun(value: unknown, context: string): Run {
  const r = expectExactKeys(
    value,
    [
      'id',
      'jobId',
      'attemptNo',
      'status',
      'workerId',
      'startedAt',
      'finishedAt',
      'durationMs',
      'errorClass',
      'errorMessage',
      'resultSummary',
    ],
    context,
  )
  return {
    id: expectInteger(r.id, `${context}.id`),
    jobId: expectInteger(r.jobId, `${context}.jobId`),
    attemptNo: expectInteger(r.attemptNo, `${context}.attemptNo`),
    status: (() => {
      const status = expectInteger(r.status, `${context}.status`)
      if (!isRunStatus(status)) throw new ProtocolError(`${context}.status is invalid`)
      return status
    })(),
    workerId: expectString(r.workerId, `${context}.workerId`),
    startedAt: expectString(r.startedAt, `${context}.startedAt`),
    finishedAt: expectNullableString(r.finishedAt, `${context}.finishedAt`),
    durationMs: r.durationMs === null ? null : expectInteger(r.durationMs, `${context}.durationMs`),
    errorClass: expectString(r.errorClass, `${context}.errorClass`),
    errorMessage: expectString(r.errorMessage, `${context}.errorMessage`),
    resultSummary: expectString(r.resultSummary, `${context}.resultSummary`),
  }
}
export async function listSchedules(afterId = 0): Promise<Schedule[]> {
  const value = await request({
    method: 'GET',
    url: '/api/admin/v1/system/scheduler/schedule',
    params: { afterId, limit: 100 },
  })
  return expectArray(value, 'scheduler schedules').map((item, index) =>
    parseSchedule(item, `scheduler schedules[${index}]`),
  )
}
export async function listJobs(afterId = 0, status?: JobStatus): Promise<Job[]> {
  const value = await request({
    method: 'GET',
    url: '/api/admin/v1/system/scheduler/job',
    params: { afterId, limit: 100, ...(status ? { status } : {}) },
  })
  return expectArray(value, 'scheduler jobs').map((item, index) =>
    parseJob(item, `scheduler jobs[${index}]`),
  )
}
export async function listRuns(jobId: number): Promise<Run[]> {
  const value = await request({
    method: 'GET',
    url: `/api/admin/v1/system/scheduler/job/${jobId}/run`,
    params: { afterId: 0, limit: 100 },
  })
  return expectArray(value, 'scheduler runs').map((item, index) =>
    parseRun(item, `scheduler runs[${index}]`),
  )
}
export async function getTaskOptions(): Promise<TaskOption[]> {
  const value = await request({
    method: 'GET',
    url: '/api/admin/v1/system/scheduler/options',
  })
  return expectArray(value, 'scheduler options').map((item, index) => {
    const r = expectExactKeys(
      item,
      ['type', 'displayName', 'adminCreatable', 'builtinKey', 'defaultParams'],
      `scheduler options[${index}]`,
    )
    return {
      type: expectString(r.type, 'scheduler option.type'),
      displayName: expectString(r.displayName, 'scheduler option.displayName'),
      adminCreatable: expectBoolean(r.adminCreatable, 'scheduler option.adminCreatable'),
      builtinKey: expectString(r.builtinKey, 'scheduler option.builtinKey'),
      defaultParams: jsonObject(r.defaultParams, 'scheduler option.defaultParams'),
    }
  })
}
export async function createSchedule(input: {
  name: string
  description: string
  taskType: string
  cronExpression: string
  timezone: string
  params: Record<string, unknown>
  isEnabled: boolean
}): Promise<Schedule> {
  const value = await request({
    method: 'POST',
    url: '/api/admin/v1/system/scheduler/schedule',
    data: input,
  })
  return parseSchedule(value, 'scheduler create')
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
  const value = await request({
    method: 'PUT',
    url: `/api/admin/v1/system/scheduler/schedule/${id}`,
    data: input,
  })
  return parseSchedule(value, 'scheduler update')
}
export async function setScheduleStatus(id: number, isEnabled: boolean): Promise<void> {
  const value = await request({
    method: 'PATCH',
    url: `/api/admin/v1/system/scheduler/schedule/${id}/status`,
    data: { isEnabled },
  })
  expectEmptyObject(value, 'scheduler status')
}
export async function deleteSchedule(id: number): Promise<void> {
  const value = await request({
    method: 'DELETE',
    url: `/api/admin/v1/system/scheduler/schedule/${id}`,
  })
  expectEmptyObject(value, 'scheduler delete')
}
export async function executeSchedule(id: number): Promise<Job> {
  const value = await request({
    method: 'POST',
    url: `/api/admin/v1/system/scheduler/schedule/${id}/execute`,
  })
  return parseJob(value, 'scheduler execute')
}
export async function retryJob(id: number): Promise<Job> {
  const value = await request({
    method: 'POST',
    url: `/api/admin/v1/system/scheduler/job/${id}/retry`,
  })
  return parseJob(value, 'scheduler retry')
}
