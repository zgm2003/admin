import { ProtocolError, request } from '../utils/request'

export interface HealthStatus {
  status: 'up'
}

export interface Readiness {
  postgresql: 'up'
  redis: 'up'
}

export async function getHealth(): Promise<HealthStatus> {
  const data = await request<unknown>({ method: 'GET', url: '/health' })
  if (!isHealthStatus(data)) {
    throw new ProtocolError('GET /health returned invalid data')
  }
  return data
}

export async function getReadiness(): Promise<Readiness> {
  const data = await request<unknown>({ method: 'GET', url: '/ready' })
  if (!isReadiness(data)) {
    throw new ProtocolError('GET /ready returned invalid data')
  }
  return data
}

function isHealthStatus(value: unknown): value is HealthStatus {
  return isRecord(value) && value.status === 'up'
}

function isReadiness(value: unknown): value is Readiness {
  return isRecord(value) && value.postgresql === 'up' && value.redis === 'up'
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
