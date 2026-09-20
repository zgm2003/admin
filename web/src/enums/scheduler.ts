export const JobStatus = {
  scheduled: 1,
  queued: 2,
  running: 3,
  completed: 4,
  failed: 5,
  canceled: 6,
} as const

export type JobStatus = (typeof JobStatus)[keyof typeof JobStatus]
export const JOB_STATUS_VALUES = Object.values(JobStatus)

export const RunStatus = {
  running: 1,
  succeeded: 2,
  failed: 3,
} as const

export type RunStatus = (typeof RunStatus)[keyof typeof RunStatus]
export const RUN_STATUS_VALUES = Object.values(RunStatus)

export function isJobStatus(value: number): value is JobStatus {
  return JOB_STATUS_VALUES.includes(value as JobStatus)
}

export function isRunStatus(value: number): value is RunStatus {
  return RUN_STATUS_VALUES.includes(value as RunStatus)
}
