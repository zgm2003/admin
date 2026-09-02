import { request } from '../../utils/request'
import { isYesNo, type YesNo } from '../../enums/yes-no'
import type { PageResult } from '../../types/pagination'
import { ProtocolError } from '../../types/http'

export interface MailConfig { configured: boolean; region: string; endpoint: string; fromEmail: string; fromName: string; replyTo: string; ttlMinutes: number; isEnabled: YesNo; lastTestAt: string | null; lastTestError: string }
export interface MailTemplate { id: number; platformId: number; scene: string; name: string; subject: string; tencentTemplateId: number; variables: Record<string, string>; exampleVariables: Record<string, string>; isEnabled: YesNo; createdAt: string; updatedAt: string }
export interface MailLog { id: number; platformId: number; userId: number | null; scene: string; templateId: number; toEmail: string; subject: string; status: string; requestId: string; messageId: string; errorCode: string; errorSummary: string; latencyMs: number; sentAt: string | null; createdAt: string; updatedAt: string }
export interface MailLogDetail { log: MailLog; verificationCode: string; verificationExpiresAt: string | null }
export interface MailRule { id: number; platformId: number; scope: 'email' | 'domain'; pattern: string; action: 'allow' | 'deny'; name: string; remark: string; isEnabled: YesNo; createdAt: string; updatedAt: string }
export interface MailConfigInput { secretId: string; secretKey: string; region: string; endpoint: string; fromEmail: string; fromName: string; replyTo: string; ttlMinutes: number; isEnabled: YesNo }
export interface MailTemplateInput { scene: string; name: string; subject: string; tencentTemplateId: number; variables: Record<string, string>; exampleVariables: Record<string, string> }
export interface MailRuleInput { scope: 'email' | 'domain'; pattern: string; action: 'allow' | 'deny'; name: string; remark: string; isEnabled: YesNo }
export interface MailTestInput { toEmail: string; scene: string; variables: Record<string, string> }

function record(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ProtocolError('mail response is invalid')
  return value as Record<string, unknown>
}
function exact(value: Record<string, unknown>, keys: readonly string[]) {
  const actual = Object.keys(value).sort()
  const expected = [...keys].sort()
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) throw new ProtocolError('mail response fields are invalid')
}
function integer(value: unknown): value is number { return typeof value === 'number' && Number.isInteger(value) }
function text(value: unknown): value is string { return typeof value === 'string' }
function stringMap(value: unknown): value is Record<string, string> {
  const data = record(value)
  return Object.values(data).every(text)
}

const configKeys = ['configured', 'region', 'endpoint', 'fromEmail', 'fromName', 'replyTo', 'ttlMinutes', 'isEnabled', 'lastTestAt', 'lastTestError'] as const
export function parseMailConfig(value: unknown): MailConfig {
  const data = record(value)
  exact(data, configKeys)
  if (typeof data.configured !== 'boolean' || !text(data.region) || !text(data.endpoint) || !text(data.fromEmail) || !text(data.fromName) || !text(data.replyTo) || !integer(data.ttlMinutes) || !isYesNo(data.isEnabled) || (data.lastTestAt !== null && !text(data.lastTestAt)) || !text(data.lastTestError)) throw new ProtocolError('mail config response is invalid')
  return data as unknown as MailConfig
}

const templateKeys = ['id', 'platformId', 'scene', 'name', 'subject', 'tencentTemplateId', 'variables', 'exampleVariables', 'isEnabled', 'createdAt', 'updatedAt'] as const
export function parseMailTemplate(value: unknown): MailTemplate {
  const data = record(value)
  exact(data, templateKeys)
  if (!integer(data.id) || !integer(data.platformId) || !text(data.scene) || !text(data.name) || !text(data.subject) || !integer(data.tencentTemplateId) || !stringMap(data.variables) || !stringMap(data.exampleVariables) || !isYesNo(data.isEnabled) || !text(data.createdAt) || !text(data.updatedAt)) throw new ProtocolError('mail template response is invalid')
  return data as unknown as MailTemplate
}

const logKeys = ['id', 'platformId', 'userId', 'scene', 'templateId', 'toEmail', 'subject', 'status', 'requestId', 'messageId', 'errorCode', 'errorSummary', 'latencyMs', 'sentAt', 'createdAt', 'updatedAt'] as const
export function parseMailLog(value: unknown): MailLog {
  const data = record(value)
  exact(data, logKeys)
  if (!integer(data.id) || !integer(data.platformId) || (data.userId !== null && !integer(data.userId)) || !text(data.scene) || !integer(data.templateId) || !text(data.toEmail) || !text(data.subject) || !text(data.status) || !text(data.requestId) || !text(data.messageId) || !text(data.errorCode) || !text(data.errorSummary) || !integer(data.latencyMs) || (data.sentAt !== null && !text(data.sentAt)) || !text(data.createdAt) || !text(data.updatedAt)) throw new ProtocolError('mail log response is invalid')
  return data as unknown as MailLog
}

const ruleKeys = ['id', 'platformId', 'scope', 'pattern', 'action', 'name', 'remark', 'isEnabled', 'createdAt', 'updatedAt'] as const
export function parseMailRule(value: unknown): MailRule {
  const data = record(value)
  exact(data, ruleKeys)
  if (!integer(data.id) || !integer(data.platformId) || (data.scope !== 'email' && data.scope !== 'domain') || !text(data.pattern) || (data.action !== 'allow' && data.action !== 'deny') || !text(data.name) || !text(data.remark) || !isYesNo(data.isEnabled) || !text(data.createdAt) || !text(data.updatedAt)) throw new ProtocolError('mail recipient rule response is invalid')
  return data as unknown as MailRule
}

export function parseMailLogPage(value: unknown): PageResult<MailLog> {
  const data = record(value)
  exact(data, ['list', 'total', 'page', 'pageSize'])
  if (!Array.isArray(data.list) || !integer(data.total) || !integer(data.page) || !integer(data.pageSize)) throw new ProtocolError('mail log page response is invalid')
  return { list: data.list.map(parseMailLog), total: data.total, page: data.page, pageSize: data.pageSize }
}
export function parseMailLogDetail(value: unknown): MailLogDetail {
  const data = record(value)
  exact(data, ['log', 'verificationCode', 'verificationExpiresAt'])
  if (!text(data.verificationCode) || (data.verificationExpiresAt !== null && !text(data.verificationExpiresAt))) throw new ProtocolError('mail log detail response is invalid')
  return { log: parseMailLog(data.log), verificationCode: data.verificationCode, verificationExpiresAt: data.verificationExpiresAt }
}

export function getMailConfig() { return request<MailConfig>({ method: 'GET', url: '/api/admin/v1/mail/config' }).then(parseMailConfig) }
export function saveMailConfig(data: MailConfigInput) { return request<MailConfig>({ method: 'PUT', url: '/api/admin/v1/mail/config', data }).then(parseMailConfig) }
export function deleteMailConfig() { return request<Record<string, never>>({ method: 'DELETE', url: '/api/admin/v1/mail/config' }) }
export function sendMailTest(data: MailTestInput) { return request({ method: 'POST', url: '/api/admin/v1/mail/test', data }) }
export function listMailTemplates() { return request<unknown>({ method: 'GET', url: '/api/admin/v1/mail/templates' }).then((value) => { if (!Array.isArray(value)) throw new ProtocolError('mail templates response is invalid'); return value.map(parseMailTemplate) }) }
export function updateMailTemplate(id: number, data: MailTemplateInput) { return request({ method: 'PUT', url: `/api/admin/v1/mail/templates/${id}`, data }) }
export function updateMailTemplateStatus(id: number, isEnabled: YesNo) { return request({ method: 'PATCH', url: `/api/admin/v1/mail/templates/${id}/status`, data: { isEnabled } }) }
export function listMailLogs(params: { page: number; pageSize: number }) { return request<unknown>({ method: 'GET', url: '/api/admin/v1/mail/logs', params }).then(parseMailLogPage) }
export function getMailLogDetail(id: number) { return request<unknown>({ method: 'GET', url: `/api/admin/v1/mail/logs/${id}` }).then(parseMailLogDetail) }
export function deleteMailLog(id: number) { return request({ method: 'DELETE', url: `/api/admin/v1/mail/logs/${id}` }) }
export function deleteMailLogs(ids: number[]) { return request({ method: 'DELETE', url: '/api/admin/v1/mail/logs', data: ids }) }
export function listMailRules() { return request<unknown>({ method: 'GET', url: '/api/admin/v1/mail/recipient-rules' }).then((value) => { if (!Array.isArray(value)) throw new ProtocolError('mail recipient rules response is invalid'); return value.map(parseMailRule) }) }
export function createMailRule(data: MailRuleInput) { return request({ method: 'POST', url: '/api/admin/v1/mail/recipient-rules', data }) }
export function updateMailRule(id: number, data: MailRuleInput) { return request({ method: 'PUT', url: `/api/admin/v1/mail/recipient-rules/${id}`, data }) }
export function updateMailRuleStatus(id: number, isEnabled: YesNo) { return request({ method: 'PATCH', url: `/api/admin/v1/mail/recipient-rules/${id}/status`, data: { isEnabled } }) }
export function deleteMailRule(id: number) { return request({ method: 'DELETE', url: `/api/admin/v1/mail/recipient-rules/${id}` }) }
