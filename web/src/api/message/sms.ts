import { type YesNo } from '@/enums/yesNo'
import type { PageResult } from '@/types/pagination'
import { request } from '@/utils/request'
import type { BackendOption, StatusOption } from '@/types/option'

export type SmsScene = string

export type SmsStatus = number

export interface SmsOptions {
  rateLimitConstraints: {
    minLimit: number
    maxLimit: number
    minWindowSeconds: number
    maxWindowSeconds: number
  }
  scenes: Array<BackendOption<string> & { variableKeys: string[] }>
  statuses: StatusOption[]
  ruleScopes: BackendOption<number>[]
  ruleActions: StatusOption[]
  ruleDefaults: { scope: number; action: number }
  rateLimitPolicies: BackendOption<string>[]
  rateLimitModes: BackendOption<string>[]
  rateLimitDimensions: BackendOption<string>[]
}

export function getSmsOptions(): Promise<SmsOptions> {
  return request.get<SmsOptions>('/api/admin/v1/message/sms/options')
}

export type SmsRateLimitPolicyKey = string

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
  content: string
  variableKeys: string[]
  exampleVariables: Record<string, string>
  isEnabled: YesNo
  createdAt: string
  updatedAt: string
}

export interface SmsTemplateInput {
  scene: SmsScene
  name: string
  tencentTemplateId: string
  content: string
  variableKeys: string[]
  exampleVariables: Record<string, string>
}

export interface SmsRule {
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

export interface SmsLog {
  id: number
  platformId: number
  platform: string
  userId: number | null
  username: string
  scene: SmsScene
  templateId: number
  toPhone: string
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
  variableKeys: string[]
}

export interface SmsPageInit {
  scenes: SmsSceneOption[]
}

export interface SmsRuleInput {
  scope: number
  pattern: string
  action: number
  name: string
  remark: string
  isEnabled: YesNo
}

export interface SmsRuleUpdateInput {
  scope: number
  pattern?: string
  action: number
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

export async function getSmsPageInit(): Promise<SmsPageInit> {
  return request.get<SmsPageInit>('/api/admin/v1/message/sms/page-init')
}

export async function getSmsConfig(): Promise<SmsConfig> {
  return request.get<SmsConfig>('/api/admin/v1/message/sms/config')
}

export async function saveSmsConfig(data: SmsConfigInput): Promise<SmsConfig> {
  return request.put<SmsConfig>('/api/admin/v1/message/sms/config', data)
}

export async function deleteSmsConfig(): Promise<void> {
  return request.delete<void>('/api/admin/v1/message/sms/config')
}

export async function sendSmsTest(data: SmsTestInput): Promise<SmsTestResult> {
  return request.post<SmsTestResult>('/api/admin/v1/message/sms/test', data)
}

export async function listSmsTemplates(): Promise<SmsTemplate[]> {
  return request.get<SmsTemplate[]>('/api/admin/v1/message/sms/template')
}

export async function updateSmsTemplate(id: number, data: SmsTemplateInput): Promise<SmsTemplate> {
  return request.put<SmsTemplate>(`/api/admin/v1/message/sms/template/${id}`, data)
}

export async function updateSmsTemplateStatus(id: number, isEnabled: YesNo): Promise<void> {
  return request.patch<void>(`/api/admin/v1/message/sms/template/${id}/status`, { isEnabled })
}

export async function listSmsRules(): Promise<SmsRule[]> {
  return request.get<SmsRule[]>('/api/admin/v1/message/sms/recipient-rule')
}

export async function createSmsRule(data: SmsRuleInput): Promise<SmsRule> {
  return request.post<SmsRule>('/api/admin/v1/message/sms/recipient-rule', data)
}

export async function updateSmsRule(id: number, data: SmsRuleUpdateInput): Promise<SmsRule> {
  return request.put<SmsRule>(`/api/admin/v1/message/sms/recipient-rule/${id}`, data)
}

export async function updateSmsRuleStatus(id: number, isEnabled: YesNo): Promise<void> {
  return request.patch<void>(`/api/admin/v1/message/sms/recipient-rule/${id}/status`, { isEnabled })
}

export async function deleteSmsRule(id: number): Promise<void> {
  return request.delete<void>(`/api/admin/v1/message/sms/recipient-rule/${id}`)
}

export async function listSmsLogs(params: SmsLogQuery): Promise<PageResult<SmsLog>> {
  return request.get<PageResult<SmsLog>>('/api/admin/v1/message/sms/log', { params })
}

export async function getSmsLogDetail(id: number): Promise<SmsLogDetail> {
  return request.get<SmsLogDetail>(`/api/admin/v1/message/sms/log/${id}`)
}

export async function listSmsRateLimitPolicies(): Promise<SmsRateLimitSnapshot> {
  return request.get<SmsRateLimitSnapshot>('/api/admin/v1/message/sms/rate-limit-policy')
}

export async function updateSmsRateLimitPolicy(
  platformId: number,
  key: SmsRateLimitPolicyKey,
  data: { limit: number; windowSeconds: number },
): Promise<SmsRateLimitPlatform> {
  return request.put<SmsRateLimitPlatform>(
    `/api/admin/v1/message/sms/rate-limit-policy/${platformId}/${encodeURIComponent(key)}`,
    data,
  )
}
