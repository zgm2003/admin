import { request } from '../../utils/request'
import type { PageRequest, PageResult } from '../../types/pagination'

export interface LoginLogListQuery extends PageRequest {
  userId?: number
  platformId?: number
  eventType?: string
  loginType?: string
  isSuccess?: 0 | 1
  loginAccount?: string
  from?: string
  to?: string
}

export interface LoginLogItem {
  id: number
  userId: number | null
  sessionId: number | null
  platform: string
  loginAccount: string
  eventType: string
  loginType: string | null
  isSuccess: 0 | 1
  reasonCode: string
  clientIp: string
  userAgent: string
  createdAt: string
}

export type LoginLogPage = PageResult<LoginLogItem>

export interface LoginLogPageInit {
  eventTypes: string[]
  loginTypes: string[]
}

export function getLoginLogPageInit(): Promise<LoginLogPageInit> {
  return request<LoginLogPageInit>({ method: 'GET', url: '/api/admin/v1/users/login-logs/page-init' })
}

export function getLoginLogs(query: LoginLogListQuery): Promise<LoginLogPage> {
  return request<LoginLogPage>({ method: 'GET', url: '/api/admin/v1/users/login-logs', params: query })
}
