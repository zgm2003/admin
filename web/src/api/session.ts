import { request } from '../utils/request'
import { parseSessionPage, parseSessionRevokeResult, parseSessionStats, type SessionListQuery } from './session.contract'

export async function getSessions(query: SessionListQuery) {
  return parseSessionPage(await request<unknown>({ method: 'GET', url: '/api/v1/sessions', params: query }))
}
export async function getSessionStats() {
  return parseSessionStats(await request<unknown>({ method: 'GET', url: '/api/v1/sessions/stats' }))
}
export async function revokeSession(id: number) {
  return parseSessionRevokeResult(await request<unknown>({ method: 'DELETE', url: '/api/v1/sessions/' + id }))
}
export async function revokeSessions(ids: number[]) {
  return parseSessionRevokeResult(await request<unknown>({ method: 'DELETE', url: '/api/v1/sessions', data: { ids } }))
}
