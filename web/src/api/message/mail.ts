import { isStorageObjectKey } from '@/utils/storageObjectKey'
import { request } from '@/utils/request'
import { isYesNo, type YesNo } from '@/enums/yesNo'
import type { PageResult } from '@/types/pagination'
import { ProtocolError } from '@/types/http'

export const MailStatus = {
  Pending: 1,
  Sent: 2,
  Failed: 3,
} as const
export type MailStatus = (typeof MailStatus)[keyof typeof MailStatus]
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
  scene: string
  name: string
  subject: string
  tencentTemplateId: number | null
  content: string
  variableKeys: string[]
  exampleVariables: Record<string, string>
  isEnabled: YesNo
  createdAt: string
  updatedAt: string
}
export interface MailLog {
  id: number
  platformId: number
  platform: string
  userId: number | null
  username: string
  scene: string
  templateId: number
  toEmail: string
  subject: string
  status: MailStatus
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
  tencentTemplateId: number | null
  content: string
  variableKeys: string[]
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
  status: MailStatus
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
function status(value: unknown, context: string): MailStatus {
  if (value !== MailStatus.Pending && value !== MailStatus.Sent && value !== MailStatus.Failed) {
    throw new ProtocolError(`${context} is invalid`)
  }
  return value
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
function parseStringArray(value: unknown, context: string): string[] {
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string'))
    throw new ProtocolError(`${context} is invalid`)
  return value as string[]
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
  'scene',
  'name',
  'subject',
  'tencentTemplateId',
  'content',
  'variableKeys',
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
    !text(data.scene) ||
    !text(data.name) ||
    !text(data.subject) ||
    (data.tencentTemplateId !== null && !integer(data.tencentTemplateId)) ||
    !text(data.content) ||
    !Array.isArray(data.variableKeys) ||
    data.variableKeys.some((key) => !text(key)) ||
    !stringMap(data.exampleVariables) ||
    !isYesNo(isEnabled) ||
    !text(data.createdAt) ||
    !text(data.updatedAt)
  )
    throw new ProtocolError('mail template response is invalid')
  const variableKeys = parseStringArray(data.variableKeys, 'mail template.variableKeys')
  const exampleVariables = parseStringMap(data.exampleVariables, 'mail template.exampleVariables')
  if (
    new Set(variableKeys).size !== variableKeys.length ||
    Object.keys(exampleVariables).length !== variableKeys.length ||
    variableKeys.some((key) => !(key in exampleVariables))
  )
    throw new ProtocolError('mail template variables are invalid')
  return {
    id: expectInteger(data.id, 'mail template.id'),
    scene: expectString(data.scene, 'mail template.scene'),
    name: expectString(data.name, 'mail template.name'),
    subject: expectString(data.subject, 'mail template.subject'),
    tencentTemplateId:
      data.tencentTemplateId === null
        ? null
        : expectInteger(data.tencentTemplateId, 'mail template.tencentTemplateId'),
    content: expectString(data.content, 'mail template.content'),
    variableKeys,
    exampleVariables,
    isEnabled,
    createdAt: expectString(data.createdAt, 'mail template.createdAt'),
    updatedAt: expectString(data.updatedAt, 'mail template.updatedAt'),
  }
}

const logKeys = [
  'id',
  'platformId',
  'platform',
  'userId',
  'username',
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
    !text(data.platform) ||
    (data.userId !== null && !integer(data.userId)) ||
    !text(data.username) ||
    !text(data.scene) ||
    !integer(data.templateId) ||
    !text(data.toEmail) ||
    !text(data.subject) ||
    !Number.isInteger(data.status) ||
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
    platform: expectString(data.platform, 'mail log.platform'),
    userId: data.userId === null ? null : expectInteger(data.userId, 'mail log.userId'),
    username: expectString(data.username, 'mail log.username'),
    scene: expectString(data.scene, 'mail log.scene'),
    templateId: expectInteger(data.templateId, 'mail log.templateId'),
    toEmail: expectString(data.toEmail, 'mail log.toEmail'),
    subject: expectString(data.subject, 'mail log.subject'),
    status: status(data.status, 'mail log.status'),
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
    status: status(data.status, 'mail test result.status'),
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
  return request({ method: 'GET', url: '/api/admin/v1/message/mail/config' }).then(parseMailConfig)
}
export function saveMailConfig(data: MailConfigInput) {
  return request({ method: 'PUT', url: '/api/admin/v1/message/mail/config', data }).then(
    parseMailConfig,
  )
}
export async function deleteMailConfig(): Promise<Record<string, never>> {
  return expectEmptyObject(
    await request({ method: 'DELETE', url: '/api/admin/v1/message/mail/config' }),
    'mail config delete result',
  )
}
export function sendMailTest(data: MailTestInput): Promise<MailTestResult> {
  return request({ method: 'POST', url: '/api/admin/v1/message/mail/test', data }).then(
    parseMailTestResult,
  )
}
export function listMailTemplates() {
  return request({ method: 'GET', url: '/api/admin/v1/message/mail/template' }).then((value) => {
    if (!Array.isArray(value)) throw new ProtocolError('mail templates response is invalid')
    return value.map(parseMailTemplate)
  })
}
export function updateMailTemplate(id: number, data: MailTemplateInput) {
  return request({
    method: 'PUT',
    url: `/api/admin/v1/message/mail/template/${id}`,
    data,
  }).then((value) => expectEmptyObject(value, 'mail template update result'))
}
export function updateMailTemplateStatus(id: number, isEnabled: YesNo) {
  return request({
    method: 'PATCH',
    url: `/api/admin/v1/message/mail/template/${id}/status`,
    data: { isEnabled },
  }).then(parseMailTemplateStatus)
}
export interface MailLogQuery {
  page: number
  pageSize: number
  platform?: string
  toEmail?: string
  scene?: string
  status?: MailStatus
  from?: string
  to?: string
}
export function listMailLogs(params: MailLogQuery) {
  return request({ method: 'GET', url: '/api/admin/v1/message/mail/log', params }).then(
    parseMailLogPage,
  )
}
export function getMailLogDetail(id: number) {
  return request({ method: 'GET', url: `/api/admin/v1/message/mail/log/${id}` }).then(
    parseMailLogDetail,
  )
}
export function listMailRules() {
  return request({ method: 'GET', url: '/api/admin/v1/message/mail/recipient-rule' }).then(
    (value) => {
      if (!Array.isArray(value)) throw new ProtocolError('mail recipient rules response is invalid')
      return value.map(parseMailRule)
    },
  )
}
export function createMailRule(data: MailRuleInput): Promise<{ id: number }> {
  return request({
    method: 'POST',
    url: '/api/admin/v1/message/mail/recipient-rule',
    data,
  }).then((value) => expectId(value, 'mail rule create result'))
}
export function updateMailRule(id: number, data: MailRuleInput) {
  return request({
    method: 'PUT',
    url: `/api/admin/v1/message/mail/recipient-rule/${id}`,
    data,
  }).then((value) => expectEmptyObject(value, 'mail rule update result'))
}
export function updateMailRuleStatus(id: number, isEnabled: YesNo) {
  return request({
    method: 'PATCH',
    url: `/api/admin/v1/message/mail/recipient-rule/${id}/status`,
    data: { isEnabled },
  }).then(parseMailRuleStatus)
}
export function deleteMailRule(id: number): Promise<Record<string, never>> {
  return request({
    method: 'DELETE',
    url: `/api/admin/v1/message/mail/recipient-rule/${id}`,
  }).then((value) => expectEmptyObject(value, 'mail rule delete result'))
}

export const mailRuleCSVMaxRows = 1000
export const mailRuleCSVMaxBytes = 1024 * 1024
export const mailRuleCSVHeader = '类型,邮箱/域名,动作,名称,备注,启用状态'
export const mailRuleCSVErrorCodes = [
  'empty',
  'too_large',
  'too_many_rows',
  'invalid_encoding',
  'invalid_header',
  'invalid_csv',
  'invalid_columns',
  'invalid_scope',
  'invalid_pattern',
  'invalid_action',
  'invalid_name',
  'invalid_remark',
  'invalid_status',
  'invalid_character',
  'duplicate_file',
  'duplicate_existing',
] as const
export type MailRuleCSVError = (typeof mailRuleCSVErrorCodes)[number]
export interface MailRuleCSVRow {
  line: number
  values: string[]
  errors: MailRuleCSVError[]
}
export interface MailRuleCSVPreview {
  rows: MailRuleCSVRow[]
  errors: MailRuleCSVError[]
}
export interface MailRuleCSVFile {
  fileName: string
  content: string
}

function parseCSVErrors(value: unknown): MailRuleCSVError[] {
  const codes = expectArray(value, 'mail CSV errors').map((code) => {
    const text = expectString(code, 'mail CSV error')
    const known = mailRuleCSVErrorCodes.find((candidate) => candidate === text)
    if (known === undefined) throw new ProtocolError('mail CSV error code is unknown')
    return known
  })
  if (new Set(codes).size !== codes.length)
    throw new ProtocolError('mail CSV errors are duplicated')
  return codes
}

function parseMailRuleCSVPreview(value: unknown): MailRuleCSVPreview {
  const data = expectExactKeys(value, ['rows', 'errors'], 'mail CSV preview')
  let previousLine = 1
  const rows = expectArray(data.rows, 'mail CSV rows').map((value): MailRuleCSVRow => {
    const row = expectExactKeys(value, ['line', 'values', 'errors'], 'mail CSV row')
    const line = expectInteger(row.line, 'mail CSV row.line')
    if (line <= previousLine) throw new ProtocolError('mail CSV row lines are invalid')
    previousLine = line
    const values = expectArray(row.values, 'mail CSV row.values').map((value) =>
      expectString(value, 'mail CSV cell'),
    )
    const errors = parseCSVErrors(row.errors)
    if (values.length !== 6 && !errors.includes('invalid_columns'))
      throw new ProtocolError('mail CSV row columns are invalid')
    if (
      errors.length === 0 &&
      ((values[0] !== 'email' && values[0] !== 'domain') ||
        values[1] === '' ||
        (values[2] !== 'allow' && values[2] !== 'deny') ||
        expectString(values[3], 'mail CSV name').trim() === '' ||
        (values[5] !== '0' && values[5] !== '1'))
    )
      throw new ProtocolError('mail CSV valid row is invalid')
    return { line, values, errors }
  })
  if (rows.length > mailRuleCSVMaxRows)
    throw new ProtocolError('mail CSV preview exceeds row limit')
  return { rows, errors: parseCSVErrors(data.errors) }
}

export async function getMailRuleImportTemplate(): Promise<{ objectKey: string }> {
  const data = expectExactKeys(
    await request({
      method: 'GET',
      url: '/api/admin/v1/message/mail/recipient-rule/import-template',
    }),
    ['objectKey'],
    'mail CSV template',
  )
  const objectKey = expectString(data.objectKey, 'mail CSV template.objectKey')
  if (objectKey !== '' && (!isStorageObjectKey(objectKey) || !objectKey.endsWith('.csv')))
    throw new ProtocolError('mail CSV template object key is invalid')
  return { objectKey }
}

export async function previewMailRuleImport(content: string): Promise<MailRuleCSVPreview> {
  return parseMailRuleCSVPreview(
    await request({
      method: 'POST',
      url: '/api/admin/v1/message/mail/recipient-rule/import/preview',
      data: { content },
    }),
  )
}

export async function importMailRules(content: string): Promise<{ imported: number }> {
  const data = expectExactKeys(
    await request({
      method: 'POST',
      url: '/api/admin/v1/message/mail/recipient-rule/import',
      data: { content },
    }),
    ['imported'],
    'mail CSV import',
  )
  const imported = expectInteger(data.imported, 'mail CSV import.imported')
  if (imported < 1 || imported > mailRuleCSVMaxRows)
    throw new ProtocolError('mail CSV imported count is invalid')
  return { imported }
}

export async function exportMailRules(): Promise<MailRuleCSVFile> {
  const data = expectExactKeys(
    await request({ method: 'GET', url: '/api/admin/v1/message/mail/recipient-rule/export' }),
    ['fileName', 'content'],
    'mail CSV export',
  )
  const fileName = expectString(data.fileName, 'mail CSV export.fileName')
  const content = expectString(data.content, 'mail CSV export.content')
  if (
    fileName !== 'mail-recipient-rule.csv' ||
    !content.startsWith(`\ufeff${mailRuleCSVHeader}\n`) ||
    new TextEncoder().encode(content).byteLength > mailRuleCSVMaxBytes
  )
    throw new ProtocolError('mail CSV file is invalid')
  return { fileName, content }
}

export interface MailRateLimitPolicy {
  platformId: number
  platformCode?: string
  platformName?: string
  rowId?: string
  key: string
  mode: 'business'
  dimension: string
  limit: number
  windowSeconds: number
  updatedAt: string
}
export interface MailRateLimitPlatformCatalog {
  platformId: number
  platformCode: string
  platformName: string
  policies: MailRateLimitPolicy[]
}
export interface MailRateLimitSnapshot {
  platforms: MailRateLimitPlatformCatalog[]
}
export interface MailRateLimitUpdateResult {
  platformId: number
  policy: MailRateLimitPolicy
}
export interface MailRateLimitPolicyInput {
  limit: number
  windowSeconds: number
}

const rateLimitPolicyKeys = ['business_email_minute', 'business_email_10m'] as const
const rateLimitPolicyKeySet = new Set<string>(rateLimitPolicyKeys)
const rateLimitPolicyMetadata: Record<
  (typeof rateLimitPolicyKeys)[number],
  { mode: MailRateLimitPolicy['mode']; dimension: string }
> = {
  business_email_minute: { mode: 'business', dimension: 'platform_email' },
  business_email_10m: { mode: 'business', dimension: 'platform_email' },
}

export function parseMailRateLimitPolicy(value: unknown): MailRateLimitPolicy {
  const data = expectExactKeys(
    value,
    ['platformId', 'key', 'mode', 'dimension', 'limit', 'windowSeconds', 'updatedAt'],
    'mail rate limit policy',
  )
  const platformId = expectInteger(data.platformId, 'mail rate limit policy.platformId')
  if (platformId < 1) throw new ProtocolError('mail rate limit policy.platformId is invalid')
  const key = expectString(data.key, 'mail rate limit policy.key')
  if (!rateLimitPolicyKeySet.has(key)) {
    throw new ProtocolError('mail rate limit policy.key is unknown')
  }
  const mode = expectString(data.mode, 'mail rate limit policy.mode')
  if (mode !== 'business') {
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
    platformId,
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
  const data = expectExactKeys(value, ['platforms'], 'mail rate limit snapshot')
  const platforms = expectArray(data.platforms, 'mail rate limit snapshot.platforms').map(
    (value) => {
      const platform = expectExactKeys(
        value,
        ['platformId', 'platformCode', 'platformName', 'policies'],
        'mail rate limit platform catalog',
      )
      const platformId = expectInteger(
        platform.platformId,
        'mail rate limit platform catalog.platformId',
      )
      if (platformId < 1)
        throw new ProtocolError('mail rate limit platform catalog.platformId is invalid')
      const platformCode = expectString(
        platform.platformCode,
        'mail rate limit platform catalog.platformCode',
      )
      const platformName = expectString(
        platform.platformName,
        'mail rate limit platform catalog.platformName',
      )
      if (platformCode.trim() === '' || platformName.trim() === '')
        throw new ProtocolError('mail rate limit platform catalog identity is invalid')
      const policies = expectArray(
        platform.policies,
        'mail rate limit platform catalog.policies',
      ).map((policy) => {
        const policyData = expectExactKeys(
          policy,
          ['key', 'mode', 'dimension', 'limit', 'windowSeconds', 'updatedAt'],
          'mail rate limit platform catalog.policy',
        )
        return parseMailRateLimitPolicy({ ...policyData, platformId })
      })
      const keys = new Set(policies.map((policy) => policy.key))
      if (
        policies.length !== rateLimitPolicyKeys.length ||
        keys.size !== rateLimitPolicyKeys.length
      ) {
        throw new ProtocolError('mail rate limit platform catalog policies are incomplete')
      }
      return { platformId, platformCode, platformName, policies }
    },
  )
  const platformIds = new Set(platforms.map((platform) => platform.platformId))
  if (platformIds.size !== platforms.length || platforms.length === 0) {
    throw new ProtocolError('mail rate limit snapshot platforms are invalid')
  }
  return { platforms }
}

export function parseMailRateLimitUpdateResult(
  value: unknown,
  expectedKey?: string,
): MailRateLimitUpdateResult {
  const data = expectExactKeys(value, ['platformId', 'policy'], 'mail rate limit update result')
  const platformId = expectInteger(data.platformId, 'mail rate limit update result.platformId')
  if (platformId < 1) throw new ProtocolError('mail rate limit update result.platformId is invalid')
  const policyData = expectExactKeys(
    data.policy,
    ['key', 'mode', 'dimension', 'limit', 'windowSeconds', 'updatedAt'],
    'mail rate limit update result.policy',
  )
  const policy = parseMailRateLimitPolicy({ ...policyData, platformId })
  if (expectedKey !== undefined && policy.key !== expectedKey) {
    throw new ProtocolError('mail rate limit update result.policy.key does not match the request')
  }
  return { platformId, policy }
}

export function listMailRateLimitPolicies(): Promise<MailRateLimitSnapshot> {
  return request({
    method: 'GET',
    url: '/api/admin/v1/message/mail/rate-limit-policy',
  }).then(parseMailRateLimitSnapshot)
}

export function updateMailRateLimitPolicy(
  platformId: number,
  key: string,
  data: MailRateLimitPolicyInput,
): Promise<MailRateLimitUpdateResult> {
  return request({
    method: 'PUT',
    url: `/api/admin/v1/message/mail/rate-limit-policy/${platformId}/${encodeURIComponent(key)}`,
    data,
  }).then((value) => parseMailRateLimitUpdateResult(value, key))
}
