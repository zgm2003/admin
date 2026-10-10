import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'

export type LoginLogEventType = number

export type LoginLogType = number

export interface LoginLogListQuery extends PageRequest {
  userId?: number
  platformId?: number
  eventType?: LoginLogEventType
  loginType?: LoginLogType
  isSuccess?: 0 | 1
  account?: string
  from?: string
  to?: string
}

export interface LoginLogItem {
  id: number
  userId: number | null
  platform: string
  account: string
  eventType: LoginLogEventType
  loginType: LoginLogType | null
  isSuccess: 0 | 1
  reasonCode: string
  clientIp: string
  userAgent: string
  createdAt: string
}

export type LoginLogPage = PageResult<LoginLogItem>

export interface LoginLogPageInit {
  eventTypes: LoginLogEventType[]
  loginTypes: LoginLogType[]
}

export function getLoginLogPageInit(): Promise<LoginLogPageInit> {
  return request.get<LoginLogPageInit>('/api/admin/v1/user/loginlog/page-init')
}

export function getLoginLogs(query: LoginLogListQuery): Promise<LoginLogPage> {
  return request.get<LoginLogPage>('/api/admin/v1/user/loginlog', { params: query })
}
