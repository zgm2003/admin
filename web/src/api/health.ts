import { request } from '../utils/request'

export interface HealthStatus {
  status: 'up'
}

export interface Readiness {
  postgresql: 'up'
  redis: 'up'
}

export async function getHealth(): Promise<HealthStatus> {
  return request<HealthStatus>({ method: 'GET', url: '/health' })
}

export async function getReadiness(): Promise<Readiness> {
  return request<Readiness>({ method: 'GET', url: '/ready' })
}
