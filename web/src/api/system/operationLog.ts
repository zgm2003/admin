import { request } from '@/utils/request'
import type { YesNo } from '@/enums/yesNo'

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
  actionLabel: string
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
  return request.get<OperationLogPage>('/api/admin/v1/system/operationlog', { params: query })
}
