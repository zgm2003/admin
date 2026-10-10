import { request } from '@/utils/request'
import { type YesNo } from '@/enums/yesNo'
import type { PageRequest, PageResult } from '@/types/pagination'

export interface CosConfig {
  id: number
  name: string
  appId: string
  bucket: string
  region: string
  endpoint: string | null
  bucketDomain: string | null
  isEnabled: YesNo
  hasCredentials: boolean
  remark: string
  createdAt: string
  updatedAt: string
}

export interface CosConfigQuery extends PageRequest {
  keyword?: string
  isEnabled?: YesNo
}

export interface CreateCosConfigInput {
  name: string
  appId: string
  secretId: string
  secretKey: string
  bucket: string
  region: string
  endpoint?: string | null
  bucketDomain?: string | null
  isEnabled: YesNo
  remark: string
}

export type UpdateCosConfigInput = Omit<
  CreateCosConfigInput,
  'appId' | 'secretId' | 'secretKey' | 'isEnabled'
> & { secretId?: string; secretKey?: string }

export async function listCosConfigs(query: CosConfigQuery): Promise<PageResult<CosConfig>> {
  return request.get<PageResult<CosConfig>>('/api/admin/v1/storage/cosconfig', { params: query })
}

export async function getCosConfig(id: number): Promise<CosConfig> {
  return request.get<CosConfig>(`/api/admin/v1/storage/cosconfig/${id}`)
}

export async function createCosConfig(data: CreateCosConfigInput): Promise<{ id: number }> {
  return request.post<{ id: number }>('/api/admin/v1/storage/cosconfig', data)
}

export async function updateCosConfig(
  id: number,
  data: UpdateCosConfigInput,
): Promise<Record<string, never>> {
  return request.put<Record<string, never>>(`/api/admin/v1/storage/cosconfig/${id}`, data)
}

export async function updateCosConfigStatus(
  id: number,
  isEnabled: YesNo,
): Promise<{ id: number; isEnabled: YesNo }> {
  return request.patch<{ id: number; isEnabled: YesNo }>(
    `/api/admin/v1/storage/cosconfig/${id}/status`,
    { isEnabled },
  )
}

export async function testCosConfig(id: number): Promise<Record<string, never>> {
  return request.post<Record<string, never>>(
    `/api/admin/v1/storage/cosconfig/${id}/test`,
    undefined,
  )
}

export async function deleteCosConfig(id: number): Promise<Record<string, never>> {
  return request.delete<Record<string, never>>(`/api/admin/v1/storage/cosconfig/${id}`)
}
