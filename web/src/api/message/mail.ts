import { request } from '@/utils/request'
import { type YesNo } from '@/enums/yesNo'
import type { PageResult } from '@/types/pagination'
import type { BackendOption, StatusOption } from '@/types/option'

export type MailStatus = number

export interface MailOptions {
  rateLimitConstraints: {
    minLimit: number
    maxLimit: number
    minWindowSeconds: number
    maxWindowSeconds: number
  }
  importConstraints: { maxBytes: number; maxRows: number }
  scenes: Array<BackendOption<string> & { variableKeys: string[] }>
  statuses: StatusOption[]
  ruleScopes: BackendOption<number>[]
  ruleActions: StatusOption[]
  ruleDefaults: { scope: number; action: number }
  rateLimitPolicies: BackendOption<string>[]
  rateLimitModes: BackendOption<string>[]
  rateLimitDimensions: BackendOption<string>[]
}

export function getMailOptions(): Promise<MailOptions> {
  return request.get<MailOptions>('/api/admin/v1/message/mail/options')
}

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
  scope: number
  pattern: string
  action: number
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
  scope: number
  pattern: string
  action: number
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

export function getMailConfig(): Promise<MailConfig> {
  return request.get<MailConfig>('/api/admin/v1/message/mail/config')
}

export function saveMailConfig(data: MailConfigInput): Promise<MailConfig> {
  return request.put<MailConfig>('/api/admin/v1/message/mail/config', data)
}

export async function deleteMailConfig(): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>('/api/admin/v1/message/mail/config')
}

export function sendMailTest(data: MailTestInput): Promise<MailTestResult> {
  return request.post<MailTestResult>('/api/admin/v1/message/mail/test', data)
}

export function listMailTemplates(): Promise<MailTemplate[]> {
  return request.get<MailTemplate[]>('/api/admin/v1/message/mail/template')
}

export function updateMailTemplate(
  id: number,
  data: MailTemplateInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>>(`/api/admin/v1/message/mail/template/${id}`, data)
}

export function updateMailTemplateStatus(
  id: number,
  isEnabled: YesNo,
): Promise<{ id: number; isEnabled: YesNo }> {
  return request.patch<{ id: number; isEnabled: YesNo }>(
    `/api/admin/v1/message/mail/template/${id}/status`,
    { isEnabled },
  )
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

export function listMailLogs(params: MailLogQuery): Promise<PageResult<MailLog>> {
  return request.get<PageResult<MailLog>>('/api/admin/v1/message/mail/log', { params })
}

export function getMailLogDetail(id: number): Promise<MailLogDetail> {
  return request.get<MailLogDetail>(`/api/admin/v1/message/mail/log/${id}`)
}

export function listMailRules(): Promise<MailRule[]> {
  return request.get<MailRule[]>('/api/admin/v1/message/mail/recipient-rule')
}

export function createMailRule(data: MailRuleInput): Promise<{ id: number }> {
  return request.post<{ id: number }>('/api/admin/v1/message/mail/recipient-rule', data)
}

export function updateMailRule(id: number, data: MailRuleInput): Promise<Record<string, never>> {
  return request.put<Record<string, never>>(`/api/admin/v1/message/mail/recipient-rule/${id}`, data)
}

export function updateMailRuleStatus(
  id: number,
  isEnabled: YesNo,
): Promise<{ id: number; isEnabled: YesNo }> {
  return request.patch<{ id: number; isEnabled: YesNo }>(
    `/api/admin/v1/message/mail/recipient-rule/${id}/status`,
    { isEnabled },
  )
}

export function deleteMailRule(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`/api/admin/v1/message/mail/recipient-rule/${id}`)
}

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
  contentBase64: string
}

export async function getMailRuleImportTemplate(
  signal?: AbortSignal,
): Promise<{ objectKey: string }> {
  return request.get<{ objectKey: string }>(
    '/api/admin/v1/message/mail/recipient-rule/import-template',
    { ...(signal ? { signal } : {}) },
  )
}

export async function previewMailRuleXlsx(
  input: MailRuleXlsxImportInput,
  signal?: AbortSignal,
): Promise<MailRuleXlsxPreview> {
  return request.post<MailRuleXlsxPreview>(
    '/api/admin/v1/message/mail/recipient-rule/import/preview',
    input,
    { ...(signal ? { signal } : {}) },
  )
}

export async function importMailRuleXlsx(
  input: MailRuleXlsxImportInput,
  signal?: AbortSignal,
): Promise<{ imported: number }> {
  return request.post<{ imported: number }>(
    '/api/admin/v1/message/mail/recipient-rule/import',
    input,
    { ...(signal ? { signal } : {}) },
  )
}

export async function exportMailRuleXlsx(signal?: AbortSignal): Promise<MailRuleXlsxExportFile> {
  return request.get<MailRuleXlsxExportFile>('/api/admin/v1/message/mail/recipient-rule/export', {
    ...(signal ? { signal } : {}),
  })
}

export interface MailRuleXlsxImportInput {
  fileName: string
  contentBase64: string
}

export interface MailRateLimitPolicy {
  platformId: number
  platformCode?: string
  platformName?: string
  rowId?: string
  key: string
  mode: string
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

export function listMailRateLimitPolicies(): Promise<MailRateLimitSnapshot> {
  return request.get<MailRateLimitSnapshot>('/api/admin/v1/message/mail/rate-limit-policy')
}

export function updateMailRateLimitPolicy(
  platformId: number,
  key: string,
  data: MailRateLimitPolicyInput,
): Promise<MailRateLimitUpdateResult> {
  return request.put<MailRateLimitUpdateResult>(
    `/api/admin/v1/message/mail/rate-limit-policy/${platformId}/${encodeURIComponent(key)}`,
    data,
  )
}
