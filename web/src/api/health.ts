import { request } from '@/utils/request'
import { expectRecord, expectString } from '@/api/protocol'
import { ProtocolError } from '@/types/http'

export interface HealthStatus {
  status: 'up'
}

export interface Readiness {
  postgresql: 'up'
  redis: 'up'
}

export async function getHealth(): Promise<HealthStatus> {
  const value = expectRecord(await request<unknown>({ method: 'GET', url: '/health' }), 'health')
  if (Object.keys(value).some((key) => key !== 'status'))
    throw new ProtocolError('health has unknown fields')
  if (expectString(value.status, 'health.status') !== 'up')
    throw new ProtocolError('health.status is invalid')
  return { status: 'up' }
}

export async function getReadiness(): Promise<Readiness> {
  const value = expectRecord(await request<unknown>({ method: 'GET', url: '/ready' }), 'readiness')
  if (Object.keys(value).some((key) => key !== 'postgresql' && key !== 'redis')) {
    throw new ProtocolError('readiness has unknown fields')
  }
  if (expectString(value.postgresql, 'readiness.postgresql') !== 'up') {
    throw new ProtocolError('readiness.postgresql is invalid')
  }
  if (expectString(value.redis, 'readiness.redis') !== 'up')
    throw new ProtocolError('readiness.redis is invalid')
  return { postgresql: 'up', redis: 'up' }
}
