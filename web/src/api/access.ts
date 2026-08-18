import { request } from '../utils/request'
import { parseAccessSnapshot, type AccessSnapshot } from './access.contract'

export async function getAccess(): Promise<AccessSnapshot> {
  const data = await request<unknown>({ method: 'GET', url: '/api/v1/access' })
  return parseAccessSnapshot(data)
}
