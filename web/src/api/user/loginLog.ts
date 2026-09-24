import { request } from '@/utils/request'
import type { PageRequest, PageResult } from '@/types/pagination'
import {
  expectArray,
  expectExactKeys,
  expectInteger,
  expectPage,
  expectString,
} from '@/api/protocol'
import { ProtocolError } from '@/types/http'

export type LoginLogEventType = 1 | 2 | 3
export type LoginLogType = 1 | 2 | 3

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
  return request({
    method: 'GET',
    url: '/api/admin/v1/user/loginlog/page-init',
  }).then(parseLoginLogPageInit)
}

export function getLoginLogs(query: LoginLogListQuery): Promise<LoginLogPage> {
  return request({
    method: 'GET',
    url: '/api/admin/v1/user/loginlog',
    params: query,
  }).then((value) => expectPage(value, parseLoginLogItem, 'login logs'))
}

function parseLoginLogPageInit(value: unknown): LoginLogPageInit {
  const record = expectExactKeys(value, ['eventTypes', 'loginTypes'], 'login log page init')
  const eventTypes = expectArray(record.eventTypes, 'login log page init.eventTypes').map((item) =>
    parseEnum(item, [1, 2, 3] as const, 'login log page init.eventTypes[]'),
  )
  const loginTypes = expectArray(record.loginTypes, 'login log page init.loginTypes').map((item) =>
    parseEnum(item, [1, 2, 3] as const, 'login log page init.loginTypes[]'),
  )
  return { eventTypes, loginTypes }
}

function parseLoginLogItem(value: unknown, index: number): LoginLogItem {
  const record = expectExactKeys(
    value,
    [
      'id',
      'userId',
      'platform',
      'account',
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
    platform: expectString(record.platform, `login logs[${index}].platform`),
    account: expectString(record.account, `login logs[${index}].account`),
    eventType: parseEnum(record.eventType, [1, 2, 3] as const, `login logs[${index}].eventType`),
    loginType:
      record.loginType === null
        ? null
        : parseEnum(record.loginType, [1, 2, 3] as const, `login logs[${index}].loginType`),
    isSuccess,
    reasonCode: expectString(record.reasonCode, `login logs[${index}].reasonCode`),
    clientIp: expectString(record.clientIp, `login logs[${index}].clientIp`),
    userAgent: expectString(record.userAgent, `login logs[${index}].userAgent`),
    createdAt: expectString(record.createdAt, `login logs[${index}].createdAt`),
  }
}

function parseEnum<T extends number>(value: unknown, allowed: readonly T[], context: string): T {
  if (!allowed.includes(value as T)) throw new ProtocolError(`${context} is invalid`)
  return value as T
}
