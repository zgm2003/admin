import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'
import {
  expectArray,
  expectExactKeys,
  expectInteger,
  expectNullableString,
  expectPage,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'

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
  return request<unknown>({
    method: 'GET',
    url: '/api/admin/v1/users/login-logs/page-init',
  }).then(parseLoginLogPageInit)
}

export function getLoginLogs(query: LoginLogListQuery): Promise<LoginLogPage> {
  return request<unknown>({
    method: 'GET',
    url: '/api/admin/v1/users/login-logs',
    params: query,
  }).then((value) => expectPage(value, parseLoginLogItem, 'login logs'))
}

function parseLoginLogPageInit(value: unknown): LoginLogPageInit {
  const record = expectExactKeys(value, ['eventTypes', 'loginTypes'], 'login log page init')
  const eventTypes = expectArray(record.eventTypes, 'login log page init.eventTypes').map((item) =>
    expectString(item, 'login log page init.eventTypes[]'),
  )
  const loginTypes = expectArray(record.loginTypes, 'login log page init.loginTypes').map((item) =>
    expectString(item, 'login log page init.loginTypes[]'),
  )
  return { eventTypes, loginTypes }
}

function parseLoginLogItem(value: unknown, index: number): LoginLogItem {
  const record = expectExactKeys(
    value,
    [
      'id',
      'userId',
      'sessionId',
      'platform',
      'loginAccount',
      'eventType',
      'loginType',
      'isSuccess',
      'reasonCode',
      'clientIp',
      'userAgent',
      'createdAt',
    ],
    `login logs[${index}]`,
  )
  const isSuccess = record.isSuccess
  if (isSuccess !== 0 && isSuccess !== 1) throw new ProtocolError('login log isSuccess is invalid')
  return {
    id: expectInteger(record.id, `login logs[${index}].id`),
    userId:
      record.userId === null ? null : expectInteger(record.userId, `login logs[${index}].userId`),
    sessionId:
      record.sessionId === null
        ? null
        : expectInteger(record.sessionId, `login logs[${index}].sessionId`),
    platform: expectString(record.platform, `login logs[${index}].platform`),
    loginAccount: expectString(record.loginAccount, `login logs[${index}].loginAccount`),
    eventType: expectString(record.eventType, `login logs[${index}].eventType`),
    loginType: expectNullableString(record.loginType, `login logs[${index}].loginType`),
    isSuccess,
    reasonCode: expectString(record.reasonCode, `login logs[${index}].reasonCode`),
    clientIp: expectString(record.clientIp, `login logs[${index}].clientIp`),
    userAgent: expectString(record.userAgent, `login logs[${index}].userAgent`),
    createdAt: expectString(record.createdAt, `login logs[${index}].createdAt`),
  }
}
