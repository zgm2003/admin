import { request } from '@/utils/request'

export interface HealthStatus {
  status: 'up'
}

export interface Readiness {
  postgresql: 'up'
  redis: 'up'
}

export async function getHealth(): Promise<HealthStatus> {
  return request.get<HealthStatus>('/health')
}

export async function getReadiness(): Promise<Readiness> {
  return request.get<Readiness>('/ready')
}
