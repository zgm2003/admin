import { request } from '../utils/request'
import { parseOperationLogPage, type OperationLogListQuery } from './operation-log.contract'

export async function getOperationLogs(query: OperationLogListQuery) {
  return parseOperationLogPage(await request<unknown>({ method: 'GET', url: '/api/v1/operation-logs', params: query }))
}
