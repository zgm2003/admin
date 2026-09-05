import { request } from '@/utils/request'
import { isYesNo, type YesNo } from '@/enums/yes-no'
import type { PageResult } from '@/types/pagination'
import { ProtocolError } from '@/types/http'
import {
  expectArray,
  expectBoolean,
  expectEmptyObject,
  expectExactKeys,
  expectId,
  expectInteger,
  expectNullableString,
  expectString,
} from '@/api/protocol'

export interface MailConfig {
  configured: boolean
  region: string
  endpoint: string
  fromEmail: string
  fromName: string
  replyTo: string
  ttlMinutes: number
  isEnabled: YesNo
  lastTestAt: string | null
  lastTestError: string
}
export interface MailTemplate {
  id: number
  platformId: number
  scene: string
  name: string
  subject: string
  tencentTemplateId: number
  variables: Record<string, string>
  exampleVariables: Record<string, string>
  isEnabled: YesNo
  createdAt: string
  updatedAt: string
}
export interface MailLog {
  id: number
  platformId: number
  userId: number | null
  scene: string
  templateId: number
  toEmail: string
  subject: string
  status: string
  requestId: string
  messageId: string
  errorCode: string
  errorSummary: string
  latencyMs: number
  sentAt: string | null
  createdAt: string
  updatedAt: string
}
export interface MailLogDetail {
  log: MailLog
  verificationCode: string
  verificationExpiresAt: string | null
}
export interface MailRule {
  id: number
  platformId: number
  scope: 'email' | 'domain'
  pattern: string
  action: 'allow' | 'deny'
  name: string
  remark: string
  isEnabled: YesNo
  createdAt: string
  updatedAt: string
}
export interface MailConfigInput {
  secretId: string
  secretKey: string
  region: string
  endpoint: string
  fromEmail: string
  fromName: string
  replyTo: string
  ttlMinutes: number
  isEnabled: YesNo
}
export interface MailTemplateInput {
  scene: string
  name: string
  subject: string
  tencentTemplateId: number
  variables: Record<string, string>
  exampleVariables: Record<string, string>
}
export interface MailRuleInput {
  scope: 'email' | 'domain'
  pattern: string
  action: 'allow' | 'deny'
  name: string
  remark: string
  isEnabled: YesNo
}
export interface MailTestInput {
  toEmail: string
  scene: string
  variables: Record<string, string>
}
export interface MailTestResult {
  logId: number
  status: string
  requestId: string
  messageId: string
}

function record(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    throw new ProtocolError('mail response is invalid')
  return value as Record<string, unknown>
}
function exact(value: Record<string, unknown>, keys: readonly string[]) {
  const actual = Object.keys(value).sort()
  const expected = [...keys].sort()
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index]))
    throw new ProtocolError('mail response fields are invalid')
}
function integer(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value)
}
function text(value: unknown): value is string {
  return typeof value === 'string'
}
function stringMap(value: unknown): value is Record<string, string> {
  const data = record(value)
  return Object.values(data).every(text)
}
function parseStringMap(value: unknown, context: string): Record<string, string> {
  const data = record(value)
  const result: Record<string, string> = {}
  for (const [key, item] of Object.entries(data)) {
    if (!text(item)) throw new ProtocolError(`${context}.${key} must be a string`)
    result[key] = item
  }
  return result
}

const configKeys = [
  'configured',
  'region',
  'endpoint',
  'fromEmail',
  'fromName',
  'replyTo',
  'ttlMinutes',
  'isEnabled',
  'lastTestAt',
  'lastTestError',
] as const
export function parseMailConfig(value: unknown): MailConfig {
  const data = record(value)
  exact(data, configKeys)
  const isEnabled = data.isEnabled
  if (
    typeof data.configured !== 'boolean' ||
    !text(data.region) ||
    !text(data.endpoint) ||
    !text(data.fromEmail) ||
    !text(data.fromName) ||
    !text(data.replyTo) ||
    !integer(data.ttlMinutes) ||
    !isYesNo(isEnabled) ||
    (data.lastTestAt !== null && !text(data.lastTestAt)) ||
    !text(data.lastTestError)
  )
    throw new ProtocolError('mail config response is invalid')
  return {
    configured: expectBoolean(data.configured, 'mail config.configured'),
    region: expectString(data.region, 'mail config.region'),
    endpoint: expectString(data.endpoint, 'mail config.endpoint'),
    fromEmail: expectString(data.fromEmail, 'mail config.fromEmail'),
    fromName: expectString(data.fromName, 'mail config.fromName'),
    replyTo: expectString(data.replyTo, 'mail config.replyTo'),
    ttlMinutes: expectInteger(data.ttlMinutes, 'mail config.ttlMinutes'),
    isEnabled,
    lastTestAt: expectNullableString(data.lastTestAt, 'mail config.lastTestAt'),
    lastTestError: expectString(data.lastTestError, 'mail config.lastTestError'),
  }
}

const templateKeys = [
  'id',
  'platformId',
  'scene',
  'name',
  'subject',
  'tencentTemplateId',
  'variables',
  'exampleVariables',
  'isEnabled',
  'createdAt',
  'updatedAt',
] as const
export function parseMailTemplate(value: unknown): MailTemplate {
  const data = record(value)
  exact(data, templateKeys)
  const isEnabled = data.isEnabled
  if (
    !integer(data.id) ||
    !integer(data.platformId) ||
    !text(data.scene) ||
    !text(data.name) ||
    !text(data.subject) ||
    !integer(data.tencentTemplateId) ||
    !stringMap(data.variables) ||
    !stringMap(data.exampleVariables) ||
    !isYesNo(isEnabled) ||
    !text(data.createdAt) ||
    !text(data.updatedAt)
  )
    throw new ProtocolError('mail template response is invalid')
  return {
    id: expectInteger(data.id, 'mail template.id'),
    platformId: expectInteger(data.platformId, 'mail template.platformId'),
    scene: expectString(data.scene, 'mail template.scene'),
    name: expectString(data.name, 'mail template.name'),
    subject: expectString(data.subject, 'mail template.subject'),
    tencentTemplateId: expectInteger(data.tencentTemplateId, 'mail template.tencentTemplateId'),
    variables: parseStringMap(data.variables, 'mail template.variables'),
    exampleVariables: parseStringMap(data.exampleVariables, 'mail template.exampleVariables'),
    isEnabled,
    createdAt: expectString(data.createdAt, 'mail template.createdAt'),
    updatedAt: expectString(data.updatedAt, 'mail template.updatedAt'),
  }
}

const logKeys = [
  'id',
  'platformId',
  'userId',
  'scene',
  'templateId',
  'toEmail',
  'subject',
  'status',
  'requestId',
  'messageId',
  'errorCode',
  'errorSummary',
  'latencyMs',
  'sentAt',
  'createdAt',
  'updatedAt',
] as const
export function parseMailLog(value: unknown): MailLog {
  const data = record(value)
  exact(data, logKeys)
  if (
    !integer(data.id) ||
    !integer(data.platformId) ||
    (data.userId !== null && !integer(data.userId)) ||
    !text(data.scene) ||
    !integer(data.templateId) ||
    !text(data.toEmail) ||
    !text(data.subject) ||
    !text(data.status) ||
    !text(data.requestId) ||
    !text(data.messageId) ||
    !text(data.errorCode) ||
    !text(data.errorSummary) ||
    !integer(data.latencyMs) ||
    (data.sentAt !== null && !text(data.sentAt)) ||
    !text(data.createdAt) ||
    !text(data.updatedAt)
  )
    throw new ProtocolError('mail log response is invalid')
  return {
    id: expectInteger(data.id, 'mail log.id'),
    platformId: expectInteger(data.platformId, 'mail log.platformId'),
    userId: data.userId === null ? null : expectInteger(data.userId, 'mail log.userId'),
    scene: expectString(data.scene, 'mail log.scene'),
    templateId: expectInteger(data.templateId, 'mail log.templateId'),
    toEmail: expectString(data.toEmail, 'mail log.toEmail'),
    subject: expectString(data.subject, 'mail log.subject'),
    status: expectString(data.status, 'mail log.status'),
    requestId: expectString(data.requestId, 'mail log.requestId'),
    messageId: expectString(data.messageId, 'mail log.messageId'),
    errorCode: expectString(data.errorCode, 'mail log.errorCode'),
    errorSummary: expectString(data.errorSummary, 'mail log.errorSummary'),
    latencyMs: expectInteger(data.latencyMs, 'mail log.latencyMs'),
    sentAt: expectNullableString(data.sentAt, 'mail log.sentAt'),
    createdAt: expectString(data.createdAt, 'mail log.createdAt'),
    updatedAt: expectString(data.updatedAt, 'mail log.updatedAt'),
  }
}

const ruleKeys = [
  'id',
  'platformId',
  'scope',
  'pattern',
  'action',
  'name',
  'remark',
  'isEnabled',
  'createdAt',
  'updatedAt',
] as const
export function parseMailRule(value: unknown): MailRule {
  const data = record(value)
  const scope = data.scope
  const action = data.action
  exact(data, ruleKeys)
  const isEnabled = data.isEnabled
  if (
    !integer(data.id) ||
    !integer(data.platformId) ||
    (scope !== 'email' && scope !== 'domain') ||
    !text(data.pattern) ||
    (action !== 'allow' && action !== 'deny') ||
    !text(data.name) ||
    !text(data.remark) ||
    !isYesNo(isEnabled) ||
    !text(data.createdAt) ||
    !text(data.updatedAt)
  )
    throw new ProtocolError('mail recipient rule response is invalid')
  return {
    id: expectInteger(data.id, 'mail rule.id'),
    platformId: expectInteger(data.platformId, 'mail rule.platformId'),
    scope,
    pattern: expectString(data.pattern, 'mail rule.pattern'),
    action,
    name: expectString(data.name, 'mail rule.name'),
    remark: expectString(data.remark, 'mail rule.remark'),
    isEnabled,
    createdAt: expectString(data.createdAt, 'mail rule.createdAt'),
    updatedAt: expectString(data.updatedAt, 'mail rule.updatedAt'),
  }
}

export function parseMailLogPage(value: unknown): PageResult<MailLog> {
  const data = record(value)
  exact(data, ['list', 'total', 'page', 'pageSize'])
  if (
    !Array.isArray(data.list) ||
    !integer(data.total) ||
    !integer(data.page) ||
    !integer(data.pageSize)
  )
    throw new ProtocolError('mail log page response is invalid')
  return {
    list: data.list.map(parseMailLog),
    total: data.total,
    page: data.page,
    pageSize: data.pageSize,
  }
}
export function parseMailLogDetail(value: unknown): MailLogDetail {
  const data = record(value)
  exact(data, ['log', 'verificationCode', 'verificationExpiresAt'])
  if (
    !text(data.verificationCode) ||
    (data.verificationExpiresAt !== null && !text(data.verificationExpiresAt))
  )
    throw new ProtocolError('mail log detail response is invalid')
  return {
    log: parseMailLog(data.log),
    verificationCode: data.verificationCode,
    verificationExpiresAt: data.verificationExpiresAt,
  }
}

function parseMailTestResult(value: unknown): MailTestResult {
  const data = expectExactKeys(
    value,
    ['logId', 'status', 'requestId', 'messageId'],
    'mail test result',
  )
  return {
    logId: expectInteger(data.logId, 'mail test result.logId'),
    status: expectString(data.status, 'mail test result.status'),
    requestId: expectString(data.requestId, 'mail test result.requestId'),
    messageId: expectString(data.messageId, 'mail test result.messageId'),
  }
}

function parseMailTemplateStatus(value: unknown): { id: number; isEnabled: YesNo } {
  const data = expectExactKeys(value, ['id', 'isEnabled'], 'mail template status result')
  if (!isYesNo(data.isEnabled)) throw new ProtocolError('mail template status result is invalid')
  return {
    id: expectInteger(data.id, 'mail template status result.id'),
    isEnabled: data.isEnabled,
  }
}

function parseMailRuleStatus(value: unknown): { id: number; isEnabled: YesNo } {
  const data = expectExactKeys(value, ['id', 'isEnabled'], 'mail rule status result')
  if (!isYesNo(data.isEnabled)) throw new ProtocolError('mail rule status result is invalid')
  return { id: expectInteger(data.id, 'mail rule status result.id'), isEnabled: data.isEnabled }
}

export function getMailConfig() {
  return request<unknown>({ method: 'GET', url: '/api/admin/v1/mail/config' }).then(parseMailConfig)
}
export function saveMailConfig(data: MailConfigInput) {
  return request<unknown>({ method: 'PUT', url: '/api/admin/v1/mail/config', data }).then(
    parseMailConfig,
  )
}
export async function deleteMailConfig(): Promise<Record<string, never>> {
  return expectEmptyObject(
    await request<unknown>({ method: 'DELETE', url: '/api/admin/v1/mail/config' }),
    'mail config delete result',
  )
}
export function sendMailTest(data: MailTestInput): Promise<MailTestResult> {
  return request<unknown>({ method: 'POST', url: '/api/admin/v1/mail/test', data }).then(
    parseMailTestResult,
  )
}
export function listMailTemplates() {
  return request<unknown>({ method: 'GET', url: '/api/admin/v1/mail/templates' }).then((value) => {
    if (!Array.isArray(value)) throw new ProtocolError('mail templates response is invalid')
    return value.map(parseMailTemplate)
  })
}
export function updateMailTemplate(id: number, data: MailTemplateInput) {
  return request<unknown>({ method: 'PUT', url: `/api/admin/v1/mail/templates/${id}`, data }).then(
    (value) => expectEmptyObject(value, 'mail template update result'),
  )
}
export function updateMailTemplateStatus(id: number, isEnabled: YesNo) {
  return request<unknown>({
    method: 'PATCH',
    url: `/api/admin/v1/mail/templates/${id}/status`,
    data: { isEnabled },
  }).then(parseMailTemplateStatus)
}
export function listMailLogs(params: { page: number; pageSize: number }) {
  return request<unknown>({ method: 'GET', url: '/api/admin/v1/mail/logs', params }).then(
    parseMailLogPage,
  )
}
export function getMailLogDetail(id: number) {
  return request<unknown>({ method: 'GET', url: `/api/admin/v1/mail/logs/${id}` }).then(
    parseMailLogDetail,
  )
}
export function deleteMailLog(id: number): Promise<Record<string, never>> {
  return request<unknown>({ method: 'DELETE', url: `/api/admin/v1/mail/logs/${id}` }).then(
    (value) => expectEmptyObject(value, 'mail log delete result'),
  )
}
export function deleteMailLogs(ids: number[]): Promise<Record<string, never>> {
  return request<unknown>({ method: 'DELETE', url: '/api/admin/v1/mail/logs', data: ids }).then(
    (value) => expectEmptyObject(value, 'mail logs delete result'),
  )
}
export function listMailRules() {
  return request<unknown>({ method: 'GET', url: '/api/admin/v1/mail/recipient-rules' }).then(
    (value) => {
      if (!Array.isArray(value)) throw new ProtocolError('mail recipient rules response is invalid')
      return value.map(parseMailRule)
    },
  )
}
export function createMailRule(data: MailRuleInput): Promise<{ id: number }> {
  return request<unknown>({ method: 'POST', url: '/api/admin/v1/mail/recipient-rules', data }).then(
    (value) => expectId(value, 'mail rule create result'),
  )
}
export function updateMailRule(id: number, data: MailRuleInput) {
  return request<unknown>({
    method: 'PUT',
    url: `/api/admin/v1/mail/recipient-rules/${id}`,
    data,
  }).then((value) => expectEmptyObject(value, 'mail rule update result'))
}
export function updateMailRuleStatus(id: number, isEnabled: YesNo) {
  return request<unknown>({
    method: 'PATCH',
    url: `/api/admin/v1/mail/recipient-rules/${id}/status`,
    data: { isEnabled },
  }).then(parseMailRuleStatus)
}
export function deleteMailRule(id: number): Promise<Record<string, never>> {
  return request<unknown>({
    method: 'DELETE',
    url: `/api/admin/v1/mail/recipient-rules/${id}`,
  }).then((value) => expectEmptyObject(value, 'mail rule delete result'))
}

export interface MailRateLimitPolicy {
  key: string
  mode: 'business' | 'admin_test'
  dimension: string
  limit: number
  windowSeconds: number
  updatedAt: string
}
export interface MailRateLimitSnapshot {
  version: number
  policies: MailRateLimitPolicy[]
}
export interface MailRateLimitUpdateResult {
  version: number
  policy: MailRateLimitPolicy
}
export interface MailRateLimitPolicyInput {
  limit: number
  windowSeconds: number
}

const rateLimitPolicyKeys = [
  'business_email_minute',
  'business_email_10m',
  'business_ip_minute',
  'business_scene_minute',
  'admin_test_user_10m',
  'admin_test_ip_minute',
  'admin_test_email_10m',
] as const
const rateLimitPolicyKeySet = new Set<string>(rateLimitPolicyKeys)
const rateLimitPolicyMetadata: Record<
  (typeof rateLimitPolicyKeys)[number],
  { mode: MailRateLimitPolicy['mode']; dimension: string }
> = {
  business_email_minute: { mode: 'business', dimension: 'platform_scene_email' },
  business_email_10m: { mode: 'business', dimension: 'platform_scene_email' },
  business_ip_minute: { mode: 'business', dimension: 'platform_ip' },
  business_scene_minute: { mode: 'business', dimension: 'platform_scene' },
  admin_test_user_10m: { mode: 'admin_test', dimension: 'admin_user' },
  admin_test_ip_minute: { mode: 'admin_test', dimension: 'ip' },
  admin_test_email_10m: { mode: 'admin_test', dimension: 'email' },
}

export function parseMailRateLimitPolicy(value: unknown): MailRateLimitPolicy {
  const data = expectExactKeys(
    value,
    ['key', 'mode', 'dimension', 'limit', 'windowSeconds', 'updatedAt'],
    'mail rate limit policy',
  )
  const key = expectString(data.key, 'mail rate limit policy.key')
  if (!rateLimitPolicyKeySet.has(key)) {
    throw new ProtocolError('mail rate limit policy.key is unknown')
  }
  const mode = expectString(data.mode, 'mail rate limit policy.mode')
  if (mode !== 'business' && mode !== 'admin_test') {
    throw new ProtocolError('mail rate limit policy.mode is invalid')
  }
  const dimension = expectString(data.dimension, 'mail rate limit policy.dimension')
  if (dimension === '') throw new ProtocolError('mail rate limit policy.dimension is empty')
  const metadata = rateLimitPolicyMetadata[key as (typeof rateLimitPolicyKeys)[number]]
  if (mode !== metadata.mode || dimension !== metadata.dimension) {
    throw new ProtocolError('mail rate limit policy metadata is invalid')
  }
  const limit = expectInteger(data.limit, 'mail rate limit policy.limit')
  const windowSeconds = expectInteger(data.windowSeconds, 'mail rate limit policy.windowSeconds')
  if (limit < 1 || limit > 100000 || windowSeconds < 1 || windowSeconds > 86400) {
    throw new ProtocolError('mail rate limit policy value is out of range')
  }
  return {
    key,
    mode,
    dimension,
    limit,
    windowSeconds,
    updatedAt: parseRateLimitPolicyTimestamp(data.updatedAt),
  }
}

function parseRateLimitPolicyTimestamp(value: unknown): string {
  const timestamp = expectString(value, 'mail rate limit policy.updatedAt')
  if (timestamp.trim() === '' || Number.isNaN(Date.parse(timestamp))) {
    throw new ProtocolError('mail rate limit policy.updatedAt must be a valid date')
  }
  return timestamp
}

export function parseMailRateLimitSnapshot(value: unknown): MailRateLimitSnapshot {
  const data = expectExactKeys(value, ['version', 'policies'], 'mail rate limit snapshot')
  const version = expectInteger(data.version, 'mail rate limit snapshot.version')
  if (version < 1) throw new ProtocolError('mail rate limit snapshot.version is invalid')
  const policies = expectArray(data.policies, 'mail rate limit snapshot.policies').map(
    parseMailRateLimitPolicy,
  )
  const keys = new Set(policies.map((policy) => policy.key))
  if (policies.length !== rateLimitPolicyKeys.length || keys.size !== rateLimitPolicyKeys.length) {
    throw new ProtocolError('mail rate limit snapshot policies are incomplete')
  }
  return { version, policies }
}

export function parseMailRateLimitUpdateResult(
  value: unknown,
  expectedKey?: string,
): MailRateLimitUpdateResult {
  const data = expectExactKeys(value, ['version', 'policy'], 'mail rate limit update result')
  const version = expectInteger(data.version, 'mail rate limit update result.version')
  if (version < 1) throw new ProtocolError('mail rate limit update result.version is invalid')
  const policy = parseMailRateLimitPolicy(data.policy)
  if (expectedKey !== undefined && policy.key !== expectedKey) {
    throw new ProtocolError('mail rate limit update result.policy.key does not match the request')
  }
  return { version, policy }
}

export function listMailRateLimitPolicies(): Promise<MailRateLimitSnapshot> {
  return request<unknown>({ method: 'GET', url: '/api/admin/v1/mail/rate-limit-policies' }).then(
    parseMailRateLimitSnapshot,
  )
}

export function updateMailRateLimitPolicy(
  key: string,
  data: MailRateLimitPolicyInput,
): Promise<MailRateLimitUpdateResult> {
  return request<unknown>({
    method: 'PUT',
    url: `/api/admin/v1/mail/rate-limit-policies/${encodeURIComponent(key)}`,
    data,
  }).then((value) => parseMailRateLimitUpdateResult(value, key))
}
