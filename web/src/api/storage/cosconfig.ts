import { request } from '@/utils/request'
import { isYesNo, type YesNo } from '@/enums/yes-no'
import type { PageRequest, PageResult } from '@/types/pagination'
import { ProtocolError } from '@/types/http'
import {
  expectBoolean,
  expectExactKeys,
  expectId,
  expectInteger,
  expectNullableString,
  expectPage,
  expectRecord,
  expectString,
} from '@/api/protocol'

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
  'secretId' | 'secretKey' | 'isEnabled'
> & { secretId?: string; secretKey?: string }
const configKeys = [
  'id',
  'name',
  'appId',
  'bucket',
  'region',
  'endpoint',
  'bucketDomain',
  'isEnabled',
  'hasCredentials',
  'remark',
  'createdAt',
  'updatedAt',
] as const
function parseConfig(v: unknown): CosConfig {
  const r = expectExactKeys(v, configKeys, 'COS config response')
  const isEnabled = r.isEnabled
  if (!isYesNo(isEnabled)) throw new ProtocolError('COS config response is invalid')
  return {
    id: expectInteger(r.id, 'COS config.id'),
    name: expectString(r.name, 'COS config.name'),
    appId: expectString(r.appId, 'COS config.appId'),
    bucket: expectString(r.bucket, 'COS config.bucket'),
    region: expectString(r.region, 'COS config.region'),
    endpoint: expectNullableString(r.endpoint, 'COS config.endpoint'),
    bucketDomain: expectNullableString(r.bucketDomain, 'COS config.bucketDomain'),
    isEnabled,
    hasCredentials: expectBoolean(r.hasCredentials, 'COS config.hasCredentials'),
    remark: expectString(r.remark, 'COS config.remark'),
    createdAt: expectString(r.createdAt, 'COS config.createdAt'),
    updatedAt: expectString(r.updatedAt, 'COS config.updatedAt'),
  }
}
export function parseCosConfigResponse(v: unknown): CosConfig {
  return parseConfig(v)
}
export async function listCosConfigs(query: CosConfigQuery): Promise<PageResult<CosConfig>> {
  return expectPage(
    await request<unknown>({
      method: 'GET',
      url: '/api/admin/v1/storage/cos-configs',
      params: query,
    }),
    parseConfig,
    'cos configs',
  )
}
export async function getCosConfig(id: number): Promise<CosConfig> {
  return parseConfig(
    await request<unknown>({ method: 'GET', url: `/api/admin/v1/storage/cos-configs/${id}` }),
  )
}
export async function createCosConfig(data: CreateCosConfigInput): Promise<{ id: number }> {
  return expectId(
    await request<unknown>({ method: 'POST', url: '/api/admin/v1/storage/cos-configs', data }),
    'cos config create result',
  )
}
export async function updateCosConfig(
  id: number,
  data: UpdateCosConfigInput,
): Promise<Record<string, never>> {
  await request<unknown>({
    method: 'PUT',
    url: `/api/admin/v1/storage/cos-configs/${id}`,
    data,
  })
  return {}
}
export async function updateCosConfigStatus(
  id: number,
  isEnabled: YesNo,
): Promise<{ id: number; isEnabled: YesNo }> {
  const result = expectRecord(
    await request<unknown>({
      method: 'PATCH',
      url: `/api/admin/v1/storage/cos-configs/${id}/status`,
      data: { isEnabled },
    }),
    'cos config status result',
  )
  if (!isYesNo(result.isEnabled)) throw new ProtocolError('cos config status is invalid')
  return { id: expectInteger(result.id, 'cos config status id'), isEnabled: result.isEnabled }
}
export async function testCosConfig(id: number): Promise<Record<string, never>> {
  await request<unknown>({ method: 'POST', url: `/api/admin/v1/storage/cos-configs/${id}/test` })
  return {}
}
export async function deleteCosConfig(id: number): Promise<Record<string, never>> {
  await request<unknown>({ method: 'DELETE', url: `/api/admin/v1/storage/cos-configs/${id}` })
  return {}
}
