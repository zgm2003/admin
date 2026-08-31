import { request } from '../../utils/request'
import type { YesNo } from '../../enums/yes-no'
export interface OperationLogListQuery { page: number; pageSize: number; userId?: number; action?: string; route?: string; isSuccess?: YesNo; from?: string; to?: string }
export interface OperationLogItem { id: number; requestId: string; userId: number | null; userName: string; sessionId: number | null; platform: string; method: string; route: string; module: string; action: string; clientIp: string; userAgent: string; statusCode: number; isSuccess: YesNo; latencyMs: number; requestData: unknown; responseData: unknown; createdAt: string; updatedAt: string }
export interface OperationLogPage { list: OperationLogItem[]; total: number; page: number; pageSize: number }

export async function getOperationLogs(query: OperationLogListQuery): Promise<OperationLogPage> {
  return request<OperationLogPage>({ method: 'GET', url: '/api/admin/v1/operation-logs', params: query })
}
