import {
  expectArray,
  expectEmptyObject,
  expectExactKeys,
  expectInteger,
  expectString,
} from '@/api/protocol'
import { isYesNo, type YesNo } from '@/enums/yesNo'
import { ProtocolError } from '@/types/http'
import type { PageResult } from '@/types/pagination'
import { request } from '@/utils/request'

export type SmsScene = 'login' | 'forget' | 'bind_phone' | 'change_password'
export type SmsStatus = 'pending' | 'sent' | 'failed'
export type SmsRuleScope = 'phone' | 'prefix'
export type SmsRuleAction = 'allow' | 'deny'
export type SmsRateLimitPolicyKey = 'business_phone_minute' | 'business_phone_10m'

export interface SmsConfig {
  configured: boolean
  smsSdkAppId: string
  signName: string
  region: string
  endpoint: string
  ttlMinutes: number
  isEnabled: YesNo
  lastTestAt: string | null
  lastTestError: string
}

export interface SmsConfigInput {
  secretId: string
  secretKey: string
  smsSdkAppId: string
  signName: string
  region: string
  endpoint: string
  ttlMinutes: number
  isEnabled: YesNo
}

export interface SmsTemplate {
  id: number
  scene: SmsScene
  name: string
  tencentTemplateId: string
  parameterKeys: ['code', 'ttl_minutes']
  exampleVariables: { code: string; ttl_minutes: string }
  isEnabled: YesNo
  createdAt: string
  updatedAt: string
}

export interface SmsTemplateInput {
  scene: SmsScene
  name: string
  tencentTemplateId: string
  parameterKeys: ['code', 'ttl_minutes']
  exampleVariables: { code: string; ttl_minutes: string }
}

export interface SmsRule {
  id: number
  scope: SmsRuleScope
  patternHint: string
  action: SmsRuleAction
  name: string
  remark: string
  isEnabled: YesNo
  createdAt: string
  updatedAt: string
}

export interface SmsLog {
  id: number
  platformId: number
  platform: string
  userId: number | null
  username: string
  scene: SmsScene
  templateId: number
  toPhoneHint: string
  status: SmsStatus
  requestId: string
  serialNo: string
  fee: number
  errorCode: string
  errorSummary: string
  latencyMs: number
  sentAt: string | null
  createdAt: string
  updatedAt: string
}

export interface SmsLogDetail {
  log: SmsLog
  toPhone: string
  verificationCode: string
  verificationExpiresAt: string | null
}

export interface SmsTestInput {
  toPhone: string
  scene: SmsScene
}

export interface SmsTestResult {
  logId: number
  status: SmsStatus
  requestId: string
  serialNo: string
}

export interface SmsSceneOption {
  scene: SmsScene
  name: string
  parameterKeys: ['code', 'ttl_minutes']
}

export interface SmsPageInit {
  scenes: SmsSceneOption[]
}

export interface SmsRuleInput {
  scope: SmsRuleScope
  pattern: string
  action: SmsRuleAction
  name: string
  remark: string
  isEnabled: YesNo
}

export interface SmsRuleUpdateInput {
  scope: SmsRuleScope
  pattern?: string
  action: SmsRuleAction
  name: string
  remark: string
  isEnabled: YesNo
}

export interface SmsRateLimitPolicy {
  key: SmsRateLimitPolicyKey
  mode: 'business'
  dimension: 'platform_phone'
  limit: number
  windowSeconds: number
  revision: number
  updatedAt: string
}

export interface SmsRateLimitPlatform {
  platformId: number
  platformCode: string
  platformName: string
  policies: SmsRateLimitPolicy[]
}

export interface SmsRateLimitSnapshot {
  platforms: SmsRateLimitPlatform[]
}

export interface SmsLogQuery {
  page: number
  pageSize: number
  platform?: string
  toPhone?: string
  scene?: SmsScene
  status?: SmsStatus
  from?: string
  to?: string
}

const sceneValues: readonly SmsScene[] = ['login', 'forget', 'bind_phone', 'change_password']
const scenes = new Set<SmsScene>(sceneValues)
const statuses = new Set<SmsStatus>(['pending', 'sent', 'failed'])
const ratePolicyKeys: readonly SmsRateLimitPolicyKey[] = [
  'business_phone_minute',
  'business_phone_10m',
]
const ratePolicyKeySet = new Set<SmsRateLimitPolicyKey>(ratePolicyKeys)
const rfc3339Pattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/

function text(value: unknown, context: string): string {
  return expectString(value, context)
}

function nonEmptyText(value: unknown, context: string): string {
  const result = text(value, context)
  if (result.trim() === '') throw new ProtocolError(`${context} is empty`)
  return result
}

function integer(value: unknown, context: string): number {
  return expectInteger(value, context)
}

function positiveInteger(value: unknown, context: string): number {
  const result = integer(value, context)
  if (result < 1) throw new ProtocolError(`${context} must be positive`)
  return result
}

function nonNegativeInteger(value: unknown, context: string): number {
  const result = integer(value, context)
  if (result < 0) throw new ProtocolError(`${context} must not be negative`)
  return result
}

function boundedInteger(value: unknown, minimum: number, maximum: number, context: string): number {
  const result = integer(value, context)
  if (result < minimum || result > maximum) {
    throw new ProtocolError(`${context} is out of range`)
  }
  return result
}

function timestamp(value: unknown, context: string): string {
  const result = text(value, context)
  if (!rfc3339Pattern.test(result) || Number.isNaN(Date.parse(result))) {
    throw new ProtocolError(`${context} must be an RFC3339 timestamp`)
  }
  return result
}

function scene(value: unknown, context: string): SmsScene {
  const result = text(value, context) as SmsScene
  if (!scenes.has(result)) throw new ProtocolError(`${context} is invalid`)
  return result
}

function status(value: unknown, context: string): SmsStatus {
  const result = text(value, context) as SmsStatus
  if (!statuses.has(result)) throw new ProtocolError(`${context} is invalid`)
  return result
}

function parameterKeys(value: unknown, context: string): ['code', 'ttl_minutes'] {
  const values = expectArray(value, context)
  if (values.length !== 2 || values[0] !== 'code' || values[1] !== 'ttl_minutes') {
    throw new ProtocolError(`${context} is invalid`)
  }
  return ['code', 'ttl_minutes']
}

function exampleVariables(value: unknown, context: string): { code: string; ttl_minutes: string } {
  const data = expectExactKeys(value, ['code', 'ttl_minutes'], context)
  return {
    code: text(data.code, `${context}.code`),
    ttl_minutes: text(data.ttl_minutes, `${context}.ttl_minutes`),
  }
}

export function parseSmsConfig(value: unknown): SmsConfig {
  const data = expectExactKeys(
    value,
    [
      'configured',
      'smsSdkAppId',
      'signName',
      'region',
      'endpoint',
      'ttlMinutes',
      'isEnabled',
      'lastTestAt',
      'lastTestError',
    ],
    'sms config',
  )
  if (typeof data.configured !== 'boolean' || !isYesNo(data.isEnabled)) {
    throw new ProtocolError('sms config is invalid')
  }
  return {
    configured: data.configured,
    smsSdkAppId: text(data.smsSdkAppId, 'sms config.smsSdkAppId'),
    signName: text(data.signName, 'sms config.signName'),
    region: text(data.region, 'sms config.region'),
    endpoint: text(data.endpoint, 'sms config.endpoint'),
    ttlMinutes: data.configured
      ? boundedInteger(data.ttlMinutes, 1, 60, 'sms config.ttlMinutes')
      : boundedInteger(data.ttlMinutes, 0, 0, 'sms config.ttlMinutes'),
    isEnabled: data.isEnabled,
    lastTestAt:
      data.lastTestAt === null ? null : timestamp(data.lastTestAt, 'sms config.lastTestAt'),
    lastTestError: text(data.lastTestError, 'sms config.lastTestError'),
  }
}

export function parseSmsTemplate(value: unknown): SmsTemplate {
  const data = expectExactKeys(
    value,
    [
      'id',
      'scene',
      'name',
      'tencentTemplateId',
      'parameterKeys',
      'exampleVariables',
      'isEnabled',
      'createdAt',
      'updatedAt',
    ],
    'sms template',
  )
  if (!isYesNo(data.isEnabled)) throw new ProtocolError('sms template is invalid')
  return {
    id: positiveInteger(data.id, 'sms template.id'),
    scene: scene(data.scene, 'sms template.scene'),
    name: nonEmptyText(data.name, 'sms template.name'),
    tencentTemplateId: text(data.tencentTemplateId, 'sms template.tencentTemplateId'),
    parameterKeys: parameterKeys(data.parameterKeys, 'sms template.parameterKeys'),
    exampleVariables: exampleVariables(data.exampleVariables, 'sms template.exampleVariables'),
    isEnabled: data.isEnabled,
    createdAt: timestamp(data.createdAt, 'sms template.createdAt'),
    updatedAt: timestamp(data.updatedAt, 'sms template.updatedAt'),
  }
}

export function parseSmsRule(value: unknown): SmsRule {
  const data = expectExactKeys(
    value,
    [
      'id',
      'scope',
      'patternHint',
      'action',
      'name',
      'remark',
      'isEnabled',
      'createdAt',
      'updatedAt',
    ],
    'sms rule',
  )
  const scope = text(data.scope, 'sms rule.scope')
  const action = text(data.action, 'sms rule.action')
  if (
    (scope !== 'phone' && scope !== 'prefix') ||
    (action !== 'allow' && action !== 'deny') ||
    !isYesNo(data.isEnabled)
  ) {
    throw new ProtocolError('sms rule is invalid')
  }
  return {
    id: positiveInteger(data.id, 'sms rule.id'),
    scope,
    patternHint: nonEmptyText(data.patternHint, 'sms rule.patternHint'),
    action,
    name: nonEmptyText(data.name, 'sms rule.name'),
    remark: text(data.remark, 'sms rule.remark'),
    isEnabled: data.isEnabled,
    createdAt: timestamp(data.createdAt, 'sms rule.createdAt'),
    updatedAt: timestamp(data.updatedAt, 'sms rule.updatedAt'),
  }
}

export function parseSmsLog(value: unknown): SmsLog {
  const data = expectExactKeys(
    value,
    [
      'id',
      'platformId',
      'platform',
      'userId',
      'username',
      'scene',
      'templateId',
      'toPhoneHint',
      'status',
      'requestId',
      'serialNo',
      'fee',
      'errorCode',
      'errorSummary',
      'latencyMs',
      'sentAt',
      'createdAt',
      'updatedAt',
    ],
    'sms log',
  )
  return {
    id: positiveInteger(data.id, 'sms log.id'),
    platformId: positiveInteger(data.platformId, 'sms log.platformId'),
    platform: nonEmptyText(data.platform, 'sms log.platform'),
    userId: data.userId === null ? null : positiveInteger(data.userId, 'sms log.userId'),
    username: text(data.username, 'sms log.username'),
    scene: scene(data.scene, 'sms log.scene'),
    templateId: positiveInteger(data.templateId, 'sms log.templateId'),
    toPhoneHint: nonEmptyText(data.toPhoneHint, 'sms log.toPhoneHint'),
    status: status(data.status, 'sms log.status'),
    requestId: text(data.requestId, 'sms log.requestId'),
    serialNo: text(data.serialNo, 'sms log.serialNo'),
    fee: nonNegativeInteger(data.fee, 'sms log.fee'),
    errorCode: text(data.errorCode, 'sms log.errorCode'),
    errorSummary: text(data.errorSummary, 'sms log.errorSummary'),
    latencyMs: nonNegativeInteger(data.latencyMs, 'sms log.latencyMs'),
    sentAt: data.sentAt === null ? null : timestamp(data.sentAt, 'sms log.sentAt'),
    createdAt: timestamp(data.createdAt, 'sms log.createdAt'),
    updatedAt: timestamp(data.updatedAt, 'sms log.updatedAt'),
  }
}

export function parseSmsLogPage(value: unknown): PageResult<SmsLog> {
  const data = expectExactKeys(value, ['list', 'total', 'page', 'pageSize'], 'sms log page')
  return {
    list: expectArray(data.list, 'sms log page.list').map(parseSmsLog),
    total: nonNegativeInteger(data.total, 'sms log page.total'),
    page: positiveInteger(data.page, 'sms log page.page'),
    pageSize: positiveInteger(data.pageSize, 'sms log page.pageSize'),
  }
}

export function parseSmsLogDetail(value: unknown): SmsLogDetail {
  const data = expectExactKeys(
    value,
    ['log', 'toPhone', 'verificationCode', 'verificationExpiresAt'],
    'sms log detail',
  )
  const toPhone = text(data.toPhone, 'sms log detail.toPhone')
  const verificationCode = text(data.verificationCode, 'sms log detail.verificationCode')
  if (!/^\+861[3-9][0-9]{9}$/.test(toPhone)) {
    throw new ProtocolError('sms log detail.toPhone is invalid')
  }
  if (verificationCode !== '' && !/^\d{6}$/.test(verificationCode)) {
    throw new ProtocolError('sms log detail.verificationCode is invalid')
  }
  return {
    log: parseSmsLog(data.log),
    toPhone,
    verificationCode,
    verificationExpiresAt:
      data.verificationExpiresAt === null
        ? null
        : timestamp(data.verificationExpiresAt, 'sms log detail.verificationExpiresAt'),
  }
}

export function parseSmsPageInit(value: unknown): SmsPageInit {
  const data = expectExactKeys(value, ['scenes'], 'sms page init')
  const result = expectArray(data.scenes, 'sms page init.scenes').map((item) => {
    const option = expectExactKeys(item, ['scene', 'name', 'parameterKeys'], 'sms scene')
    return {
      scene: scene(option.scene, 'sms scene.scene'),
      name: nonEmptyText(option.name, 'sms scene.name'),
      parameterKeys: parameterKeys(option.parameterKeys, 'sms scene.parameterKeys'),
    }
  })
  if (
    result.length !== sceneValues.length ||
    result.some((option, index) => option.scene !== sceneValues[index])
  ) {
    throw new ProtocolError('sms page init scenes are incomplete or out of order')
  }
  return { scenes: result }
}

function parseTest(value: unknown): SmsTestResult {
  const data = expectExactKeys(
    value,
    ['logId', 'status', 'requestId', 'serialNo'],
    'sms test result',
  )
  return {
    logId: positiveInteger(data.logId, 'sms test.logId'),
    status: status(data.status, 'sms test.status'),
    requestId: text(data.requestId, 'sms test.requestId'),
    serialNo: text(data.serialNo, 'sms test.serialNo'),
  }
}

function parsePolicy(value: unknown): SmsRateLimitPolicy {
  const data = expectExactKeys(
    value,
    ['key', 'mode', 'dimension', 'limit', 'windowSeconds', 'revision', 'updatedAt'],
    'sms rate policy',
  )
  const key = text(data.key, 'sms rate policy.key') as SmsRateLimitPolicyKey
  if (
    !ratePolicyKeySet.has(key) ||
    data.mode !== 'business' ||
    data.dimension !== 'platform_phone'
  ) {
    throw new ProtocolError('sms rate policy metadata is invalid')
  }
  return {
    key,
    mode: 'business',
    dimension: 'platform_phone',
    limit: boundedInteger(data.limit, 1, 100000, 'sms rate policy.limit'),
    windowSeconds: boundedInteger(data.windowSeconds, 1, 86400, 'sms rate policy.windowSeconds'),
    revision: positiveInteger(data.revision, 'sms rate policy.revision'),
    updatedAt: timestamp(data.updatedAt, 'sms rate policy.updatedAt'),
  }
}

function parsePlatform(value: unknown): SmsRateLimitPlatform {
  const data = expectExactKeys(
    value,
    ['platformId', 'platformCode', 'platformName', 'policies'],
    'sms rate platform',
  )
  const policies = expectArray(data.policies, 'sms rate policies').map(parsePolicy)
  if (
    policies.length !== ratePolicyKeys.length ||
    new Set(policies.map((policy) => policy.key)).size !== ratePolicyKeys.length ||
    !ratePolicyKeys.every((key) => policies.some((policy) => policy.key === key))
  ) {
    throw new ProtocolError('sms rate policies are incomplete or duplicated')
  }
  return {
    platformId: positiveInteger(data.platformId, 'sms platform.id'),
    platformCode: nonEmptyText(data.platformCode, 'sms platform.code'),
    platformName: nonEmptyText(data.platformName, 'sms platform.name'),
    policies,
  }
}

export function parseSmsRateLimitSnapshot(value: unknown): SmsRateLimitSnapshot {
  const data = expectExactKeys(value, ['platforms'], 'sms rate snapshot')
  const platforms = expectArray(data.platforms, 'sms rate snapshot.platforms').map(parsePlatform)
  if (platforms.length === 0) throw new ProtocolError('sms rate snapshot is empty')
  if (new Set(platforms.map((platform) => platform.platformId)).size !== platforms.length) {
    throw new ProtocolError('sms rate snapshot contains duplicate platforms')
  }
  return { platforms }
}

export async function getSmsPageInit(): Promise<SmsPageInit> {
  return parseSmsPageInit(
    await request<unknown>({ method: 'GET', url: '/api/admin/v1/message/sms/page-init' }),
  )
}

export async function getSmsConfig(): Promise<SmsConfig> {
  return parseSmsConfig(
    await request<unknown>({ method: 'GET', url: '/api/admin/v1/message/sms/config' }),
  )
}

export async function saveSmsConfig(data: SmsConfigInput): Promise<SmsConfig> {
  return parseSmsConfig(
    await request<unknown>({
      method: 'PUT',
      url: '/api/admin/v1/message/sms/config',
      data,
    }),
  )
}

export async function deleteSmsConfig(): Promise<void> {
  expectEmptyObject(
    await request<unknown>({ method: 'DELETE', url: '/api/admin/v1/message/sms/config' }),
    'sms config delete',
  )
}

export async function sendSmsTest(data: SmsTestInput): Promise<SmsTestResult> {
  return parseTest(
    await request<unknown>({ method: 'POST', url: '/api/admin/v1/message/sms/test', data }),
  )
}

export async function listSmsTemplates(): Promise<SmsTemplate[]> {
  const data = expectExactKeys(
    await request<unknown>({ method: 'GET', url: '/api/admin/v1/message/sms/template' }),
    ['list'],
    'sms templates',
  )
  return expectArray(data.list, 'sms templates.list').map(parseSmsTemplate)
}

export async function updateSmsTemplate(id: number, data: SmsTemplateInput): Promise<SmsTemplate> {
  return parseSmsTemplate(
    await request<unknown>({
      method: 'PUT',
      url: `/api/admin/v1/message/sms/template/${id}`,
      data,
    }),
  )
}

export async function updateSmsTemplateStatus(id: number, isEnabled: YesNo): Promise<void> {
  expectEmptyObject(
    await request<unknown>({
      method: 'PATCH',
      url: `/api/admin/v1/message/sms/template/${id}/status`,
      data: { isEnabled },
    }),
    'sms template status',
  )
}

export async function listSmsRules(): Promise<SmsRule[]> {
  const data = expectExactKeys(
    await request<unknown>({ method: 'GET', url: '/api/admin/v1/message/sms/recipient-rule' }),
    ['list'],
    'sms rules',
  )
  return expectArray(data.list, 'sms rules.list').map(parseSmsRule)
}

export async function createSmsRule(data: SmsRuleInput): Promise<SmsRule> {
  return parseSmsRule(
    await request<unknown>({
      method: 'POST',
      url: '/api/admin/v1/message/sms/recipient-rule',
      data,
    }),
  )
}

export async function updateSmsRule(id: number, data: SmsRuleUpdateInput): Promise<SmsRule> {
  return parseSmsRule(
    await request<unknown>({
      method: 'PUT',
      url: `/api/admin/v1/message/sms/recipient-rule/${id}`,
      data,
    }),
  )
}

export async function updateSmsRuleStatus(id: number, isEnabled: YesNo): Promise<void> {
  expectEmptyObject(
    await request<unknown>({
      method: 'PATCH',
      url: `/api/admin/v1/message/sms/recipient-rule/${id}/status`,
      data: { isEnabled },
    }),
    'sms rule status',
  )
}

export async function deleteSmsRule(id: number): Promise<void> {
  expectEmptyObject(
    await request<unknown>({
      method: 'DELETE',
      url: `/api/admin/v1/message/sms/recipient-rule/${id}`,
    }),
    'sms rule delete',
  )
}

export async function listSmsLogs(params: SmsLogQuery): Promise<PageResult<SmsLog>> {
  return parseSmsLogPage(
    await request<unknown>({
      method: 'GET',
      url: '/api/admin/v1/message/sms/log',
      params,
    }),
  )
}

export async function getSmsLogDetail(id: number): Promise<SmsLogDetail> {
  return parseSmsLogDetail(
    await request<unknown>({ method: 'GET', url: `/api/admin/v1/message/sms/log/${id}` }),
  )
}

export async function listSmsRateLimitPolicies(): Promise<SmsRateLimitSnapshot> {
  return parseSmsRateLimitSnapshot(
    await request<unknown>({
      method: 'GET',
      url: '/api/admin/v1/message/sms/rate-limit-policy',
    }),
  )
}

export async function updateSmsRateLimitPolicy(
  platformId: number,
  key: SmsRateLimitPolicyKey,
  data: { limit: number; windowSeconds: number },
): Promise<SmsRateLimitPlatform> {
  return parsePlatform(
    await request<unknown>({
      method: 'PUT',
      url: `/api/admin/v1/message/sms/rate-limit-policy/${platformId}/${encodeURIComponent(key)}`,
      data,
    }),
  )
}
