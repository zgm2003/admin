import { request } from '@/utils/request'
import { type YesNo } from '@/enums/yesNo'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface UploadRule {
  id: number
  platformId: number
  platformCode: string
  platformName: string
  codes: string[]
  name: string
  cosConfigId: number
  cosConfigName: string
  maxFileSizeBytes: number
  allowedExtensions: string[]
  allowedMimeTypes: string[]
  accessMode: 'private' | 'public'
  isEnabled: YesNo
  remark: string
  createdAt: string
  updatedAt: string
}

export interface UploadRuleQuery extends PageRequest {
  platformId?: number
  cosConfigId?: number
  keyword?: string
  isEnabled?: YesNo
}

interface UploadRuleCreateFields {
  codes: string[]
  name: string
  cosConfigId: number
  maxFileSizeBytes: number
  allowedExtensions: string[]
  allowedMimeTypes: string[]
  accessMode: 'private' | 'public'
  remark: string
}

export interface CreateUploadRuleInput extends UploadRuleCreateFields {
  platformId: number
  isEnabled: YesNo
}

export interface UpdateUploadRuleInput {
  codes: string[]
  name: string
  maxFileSizeBytes: number
  allowedExtensions: string[]
  allowedMimeTypes: string[]
  remark: string
  isEnabled?: YesNo
}

export interface PlatformOption {
  id: number
  code: string
  name: string
  isEnabled: YesNo
}

export interface ConfigSummary {
  id: number
  name: string
  bucket: string
  region: string
  isEnabled: YesNo
}

export interface UploadRulePageInit {
  platforms: PlatformOption[]
  configs: ConfigSummary[]
}

export async function listUploadRules(query: UploadRuleQuery): Promise<PageResult<UploadRule>> {
  return request.get<PageResult<UploadRule>>('/api/admin/v1/storage/uploadrule', { params: query })
}

export async function getUploadRule(id: number): Promise<UploadRule> {
  return request.get<UploadRule>(`/api/admin/v1/storage/uploadrule/${id}`)
}

export async function getUploadRulePageInit(): Promise<UploadRulePageInit> {
  return request.get<UploadRulePageInit>('/api/admin/v1/storage/uploadrule/page-init')
}

export async function createUploadRule(data: CreateUploadRuleInput): Promise<{ id: number }> {
  return request.post<{ id: number }>('/api/admin/v1/storage/uploadrule', data)
}

export async function updateUploadRule(
  id: number,
  data: UpdateUploadRuleInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>>(`/api/admin/v1/storage/uploadrule/${id}`, data)
}

export async function updateUploadRuleStatus(
  id: number,
  isEnabled: YesNo,
): Promise<{ id: number; isEnabled: YesNo }> {
  return request.patch<{ id: number; isEnabled: YesNo }>(
    `/api/admin/v1/storage/uploadrule/${id}/status`,
    { isEnabled },
  )
}

export async function deleteUploadRule(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`/api/admin/v1/storage/uploadrule/${id}`)
}
