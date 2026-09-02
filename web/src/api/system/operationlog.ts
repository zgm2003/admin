import { request } from '@/utils/request'
import type { YesNo } from '@/enums/yes-no'
import { expectInteger, expectPage, expectRecord, expectString } from '@/api/protocol'
import { isYesNo } from '@/enums/yes-no'
import { ProtocolError } from '@/types/http'
export interface OperationLogListQuery {
  page: number
  pageSize: number
  userId?: number
  action?: string
  route?: string
  isSuccess?: YesNo
  from?: string
  to?: string
}
export interface OperationLogItem {
  id: number
  requestId: string
  userId: number | null
  userName: string
  sessionId: number | null
  platform: string
  method: string
  route: string
  module: string
  action: string
  clientIp: string
  userAgent: string
  statusCode: number
  isSuccess: YesNo
  latencyMs: number
  requestData: unknown
  responseData: unknown
  createdAt: string
  updatedAt: string
}
export interface OperationLogPage {
  list: OperationLogItem[]
  total: number
  page: number
  pageSize: number
}

export async function getOperationLogs(query: OperationLogListQuery): Promise<OperationLogPage> {
  return expectPage(
    await request<unknown>({
      method: 'GET',
      url: '/api/admin/v1/operation-logs',
      params: query,
    }),
    (value, index) => {
      const item = expectRecord(value, `operation logs.list[${index}]`)
      const isSuccess = item.isSuccess
      if (!isYesNo(isSuccess))
        throw new ProtocolError(`operation logs.list[${index}].isSuccess is invalid`)
      return {
        id: expectInteger(item.id, 'operation log.id'),
        requestId: expectString(item.requestId, 'operation log.requestId'),
        userId: item.userId === null ? null : expectInteger(item.userId, 'operation log.userId'),
        userName: expectString(item.userName, 'operation log.userName'),
        sessionId:
          item.sessionId === null ? null : expectInteger(item.sessionId, 'operation log.sessionId'),
        platform: expectString(item.platform, 'operation log.platform'),
        method: expectString(item.method, 'operation log.method'),
        route: expectString(item.route, 'operation log.route'),
        module: expectString(item.module, 'operation log.module'),
        action: expectString(item.action, 'operation log.action'),
        clientIp: expectString(item.clientIp, 'operation log.clientIp'),
        userAgent: expectString(item.userAgent, 'operation log.userAgent'),
        statusCode: expectInteger(item.statusCode, 'operation log.statusCode'),
        isSuccess,
        latencyMs: expectInteger(item.latencyMs, 'operation log.latencyMs'),
        requestData: item.requestData,
        responseData: item.responseData,
        createdAt: expectString(item.createdAt, 'operation log.createdAt'),
        updatedAt: expectString(item.updatedAt, 'operation log.updatedAt'),
      }
    },
    'operation logs',
  )
}
