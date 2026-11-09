import { request } from '@/utils/request'

export interface ServerCapacity {
  totalBytes: number
  availableBytes: number
  usedPercent: number
}

export interface ServerSnapshot {
  sampledAt: string
  hostname: string
  os: string
  arch: string
  logicalCores: number
  cpuPercent: number
  memory: ServerCapacity
  disk: ServerCapacity
  diskPath: string
  hostUptimeSeconds: number
  processUptimeSeconds: number
  startedAt: string
  goVersion: string
  goroutines: number
  heapAllocBytes: number
}

export function getServerSnapshot(): Promise<ServerSnapshot> {
  return request.get<ServerSnapshot>('/api/admin/v1/system/server')
}
