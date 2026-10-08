import { isStorageObjectKey } from '@/utils/storageObjectKey'
import { request } from '@/utils/request'
import { isYesNo, type YesNo } from '@/enums/yesNo'
import {
  isMailRuleAction,
  isMailRuleScope,
  type MailRuleAction,
  type MailRuleScope,
} from '@/enums/mailRecipientRule'
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
  scope: MailRuleScope
  pattern: string
  action: MailRuleAction
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
  scope: MailRuleScope
  pattern: string
  action: MailRuleAction
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
    !isMailRuleScope(scope) ||
    !text(data.pattern) ||
    !isMailRuleAction(action) ||
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

export const mailRuleXlsxMaxRows = 1000
export const mailRuleXlsxMaxBytes = 2 * 1024 * 1024
export const mailRuleXlsxMime = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
export const mailRuleXlsxErrorCodes = [
  'empty',
  'too_large',
  'too_many_rows',
  'invalid_header',
  'invalid_xlsx',
  'missing_sheet',
  'unsupported_formula',
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
export type MailRuleXlsxError = (typeof mailRuleXlsxErrorCodes)[number]
export interface MailRuleXlsxRow {
  line: number
  rawValues: string[]
  data: MailRuleInput | null
  errors: MailRuleXlsxError[]
}
export interface MailRuleXlsxPreview {
  rows: MailRuleXlsxRow[]
  errors: MailRuleXlsxError[]
}
export interface MailRuleXlsxExportFile {
  fileName: string
  content: ArrayBuffer
}

const mailRuleXlsxFileErrors = new Set<MailRuleXlsxError>([
  'empty',
  'too_large',
  'too_many_rows',
  'invalid_header',
  'invalid_xlsx',
  'missing_sheet',
  'unsupported_formula',
  'invalid_columns',
])

function parseXlsxErrors(value: unknown, level: 'file' | 'row'): MailRuleXlsxError[] {
  const codes = expectArray(value, 'mail XLSX errors').map((code) => {
    const text = expectString(code, 'mail XLSX error')
    const known = mailRuleXlsxErrorCodes.find((candidate) => candidate === text)
    if (known === undefined) throw new ProtocolError('mail XLSX error code is unknown')
    if (mailRuleXlsxFileErrors.has(known) !== (level === 'file'))
      throw new ProtocolError('mail XLSX error level is invalid')
    return known
  })
  if (new Set(codes).size !== codes.length)
    throw new ProtocolError('mail XLSX errors are duplicated')
  return codes
}

function parseMailRuleXlsxData(value: unknown): MailRuleInput {
  const data = expectExactKeys(
    value,
    ['scope', 'pattern', 'action', 'name', 'remark', 'isEnabled'],
    'mail XLSX row.data',
  )
  if (!isMailRuleScope(data.scope) || !isMailRuleAction(data.action) || !isYesNo(data.isEnabled))
    throw new ProtocolError('mail XLSX row.data enum is invalid')
  return {
    scope: data.scope,
    pattern: expectString(data.pattern, 'mail XLSX row.data.pattern'),
    action: data.action,
    name: expectString(data.name, 'mail XLSX row.data.name'),
    remark: expectString(data.remark, 'mail XLSX row.data.remark'),
    isEnabled: data.isEnabled,
  }
}

function parseMailRuleXlsxPreview(value: unknown): MailRuleXlsxPreview {
  const data = expectExactKeys(value, ['rows', 'errors'], 'mail XLSX preview')
  let previousLine = 1
  const rows = expectArray(data.rows, 'mail XLSX rows').map((value): MailRuleXlsxRow => {
    const row = expectExactKeys(value, ['line', 'rawValues', 'data', 'errors'], 'mail XLSX row')
    const line = expectInteger(row.line, 'mail XLSX row.line')
    if (line <= previousLine || line > mailRuleXlsxMaxRows + 1)
      throw new ProtocolError('mail XLSX row lines are invalid')
    previousLine = line
    const rawValues = expectArray(row.rawValues, 'mail XLSX row.rawValues').map((value) =>
      expectString(value, 'mail XLSX raw cell'),
    )
    if (rawValues.length !== 6) throw new ProtocolError('mail XLSX raw columns are invalid')
    const errors = parseXlsxErrors(row.errors, 'row')
    const parsedData = row.data === null ? null : parseMailRuleXlsxData(row.data)
    const duplicate = (error: MailRuleXlsxError) =>
      error === 'duplicate_file' || error === 'duplicate_existing'
    if (
      parsedData === null
        ? errors.length === 0 || errors.some(duplicate)
        : errors.some((error) => !duplicate(error))
    )
      throw new ProtocolError('mail XLSX row data and errors are inconsistent')
    return { line, rawValues, data: parsedData, errors }
  })
  if (rows.length > mailRuleXlsxMaxRows)
    throw new ProtocolError('mail XLSX preview exceeds row limit')
  return { rows, errors: parseXlsxErrors(data.errors, 'file') }
}

export async function getMailRuleImportTemplate(
  signal?: AbortSignal,
): Promise<{ objectKey: string }> {
  const data = expectExactKeys(
    await request({
      method: 'GET',
      url: '/api/admin/v1/message/mail/recipient-rule/import-template',
      ...(signal ? { signal } : {}),
    }),
    ['objectKey'],
    'mail Xlsx template',
  )
  const objectKey = expectString(data.objectKey, 'mail Xlsx template.objectKey')
  if (objectKey !== '' && (!isStorageObjectKey(objectKey) || !objectKey.endsWith('.xlsx')))
    throw new ProtocolError('mail Xlsx template object key is invalid')
  return { objectKey }
}

export async function previewMailRuleXlsx(
  input: MailRuleXlsxImportInput,
  signal?: AbortSignal,
): Promise<MailRuleXlsxPreview> {
  const data = parseMailRuleXlsxInput(input)
  return parseMailRuleXlsxPreview(
    await request({
      method: 'POST',
      url: '/api/admin/v1/message/mail/recipient-rule/import/preview',
      data,
      ...(signal ? { signal } : {}),
    }),
  )
}

export async function importMailRuleXlsx(
  input: MailRuleXlsxImportInput,
  signal?: AbortSignal,
): Promise<{ imported: number }> {
  const payload = parseMailRuleXlsxInput(input)
  const data = expectExactKeys(
    await request({
      method: 'POST',
      url: '/api/admin/v1/message/mail/recipient-rule/import',
      data: payload,
      ...(signal ? { signal } : {}),
    }),
    ['imported'],
    'mail Xlsx import',
  )
  const imported = expectInteger(data.imported, 'mail Xlsx import.imported')
  if (imported < 1 || imported > mailRuleXlsxMaxRows)
    throw new ProtocolError('mail Xlsx imported count is invalid')
  return { imported }
}

export async function exportMailRuleXlsx(signal?: AbortSignal): Promise<MailRuleXlsxExportFile> {
  const data = expectExactKeys(
    await request({
      method: 'GET',
      url: '/api/admin/v1/message/mail/recipient-rule/export',
      ...(signal ? { signal } : {}),
    }),
    ['fileName', 'contentBase64'],
    'mail xlsx export',
  )
  const fileName = expectString(data.fileName, 'xlsx filename')
  if (fileName !== 'mail-recipient-rule.xlsx') throw new ProtocolError('xlsx filename is invalid')
  const content = decodeMailRuleXlsx(expectString(data.contentBase64, 'xlsx content'))
  return { fileName, content }
}

export interface MailRuleXlsxImportInput {
  fileName: string
  contentBase64: string
}

function parseMailRuleXlsxInput(value: unknown): MailRuleXlsxImportInput {
  const data = expectExactKeys(value, ['fileName', 'contentBase64'], 'mail XLSX input')
  const fileName = expectString(data.fileName, 'mail XLSX input.fileName')
  if (
    fileName !== fileName.trim() ||
    fileName.length <= 5 ||
    fileName.length > 255 ||
    !fileName.toLowerCase().endsWith('.xlsx') ||
    /[\\/:*?"<>|]/u.test(fileName) ||
    [...fileName].some(
      (character) => character.charCodeAt(0) < 32 || character.charCodeAt(0) === 127,
    )
  )
    throw new ProtocolError('mail XLSX input filename is invalid')
  const contentBase64 = expectString(data.contentBase64, 'mail XLSX input.contentBase64')
  decodeMailRuleXlsx(contentBase64)
  return { fileName, contentBase64 }
}

export function decodeMailRuleXlsx(value: string): ArrayBuffer {
  if (
    value.length === 0 ||
    value.length > 4 * Math.ceil(mailRuleXlsxMaxBytes / 3) ||
    value.length % 4 !== 0 ||
    !/^[A-Za-z0-9+/]*={0,2}$/.test(value)
  )
    throw new ProtocolError('xlsx base64 is invalid')
  let binary: string
  try {
    binary = atob(value)
  } catch {
    throw new ProtocolError('xlsx base64 is invalid')
  }
  if (
    btoa(binary) !== value ||
    binary.length > mailRuleXlsxMaxBytes ||
    binary.slice(0, 4) !== 'PK\x03\x04'
  )
    throw new ProtocolError('xlsx bytes are invalid')
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes.buffer
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
